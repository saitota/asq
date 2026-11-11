# agentsview 連携

agentsview の [Session API](https://github.com/kenn-io/agentsview/blob/main/docs/session-api.md) を CLI 経由で使う。

## 取得

`agentsview` を子プロセスで実行し、JSON を読む。バイナリは `$ASQ_AGENTSVIEW_BIN`、未設定なら PATH の `agentsview`。

- 一覧は `session list --json --include-one-shot --include-automated` をカーソルが尽きるまで読む
- 本文は、キャッシュに無いセッションとシグネチャが変わったセッションだけ、`session messages <id> --json --role user,assistant` を並列に実行して取る
- `--from <last_ordinal + 1>` でページングし、空ページで終える。`last_ordinal` が進まない時かページ上限に達した時は不完全とする
- resume 用の ID は `source_session_id`、無ければ `id` から `<agent>:` を外したもの
- 時刻は `ended_at`、無ければ `started_at`

## ドキュメントに無い前提

観測に基づく。`ASQ_AGENTSVIEW_SMOKE=1 go test ./internal/agentsview -run Smoke` は、実際の agentsview の `id` から resume 用の ID を取り出せること（1）と、最長の会話のページングが最後まで進むこと（2）を確かめる。

1. `id` に `<agent>:` が付くことがあり、エージェントの CLI はその形を受け付けない
2. `--role` で絞ると ordinal が飛ぶため、`--limit` に満たないページの後にも続きがある
3. アイドル終了したデーモンを起動した直後の `session list` は、ストア全体の再同期を待つため数十秒かかる
4. resume のコマンドと、Claude・Codex・Cursor・Copilot 以外の色は、agentsview の内部の定義（`internal/server/resume.go` の `resumeAgents`、フロントエンドの `agents.ts`）を `source.go` に転記したもの。a078a250 に合わせてあり、上流が変わるとずれる。ずれはユーザー定義ファイルで直す（[cli.md](cli.md)）

## キャッシュ

- 場所: macOS は `~/Library/Caches/asq/agentsview-messages.json`、それ以外は `${XDG_CACHE_HOME:-~/.cache}/asq/agentsview-messages.json`
- 中身: agentsview の ID ごとにシグネチャとセッション（メタデータと会話本文）。JSON タグを変えたら `cacheVersion` を上げる
- シグネチャ: `ended_at|message_count|transcript_revision`
- 一覧が取れた時は、名前・cwd・ブランチ・時刻をそこから取り、キャッシュからは本文だけを使う
- 一覧から消えたセッションはキャッシュからも消える
- 内容が変わった時だけ、一時ファイルに書いてから rename する。書き込みの失敗は無視する
- ファイルが読めない時、JSON が不正な時、版が違う時は空のキャッシュとして扱う

## 劣化動作

| 状況 | 結果 | キャッシュ |
|---|---|---|
| 一覧が失敗し、キャッシュがある | キャッシュの全セッションをそのまま返し、警告する | 更新しない |
| 一覧が失敗し、キャッシュが無い（`reindex --force` でキャッシュを読まない時を含む） | エラー | — |
| 本文の取得が失敗 | 一覧の `first_message` を本文にし、失敗理由を `Errors` に入れる | 保存しない |
| 本文が空 | 一覧の `first_message` を本文にする。それも空なら結果から外す | 保存しない |
| 本文が不完全 | 取得できた分を使い、`Errors` に記録する | 保存しない |

`Errors` を持つセッションがあると、`list` はその件数を stderr で警告する。プレビューはエラーの内容を `warning:` 行に出し、`reindex` は `errors=` に数える。

## 進捗

一覧の応答が遅い時は agentsview を待っていることを、取得件数が多い時は同期中の件数を stderr に出す。端末では同じ行を書き換え、それ以外では行単位で出す。
