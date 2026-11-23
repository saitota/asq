# CLI

## 振り分け

`cli.App.Run` が先頭の語でサブコマンドを選ぶ。引数が無い時か先頭の語がサブコマンドでない時は `pick` とし、先頭に並んだフラグを読んで残りの引数をクエリにする。`preview` は色なしのプレビューを出す隠しコマンドで、ヘルプには載せない。

## セッション指定

`resume`・`cwd`・`preview` の位置引数は次を受け付ける。

- ID か一意な前方一致。完全一致を優先し、曖昧なら見つからない扱い
- `list --fzf` の1行。シェルで単語分割された場合は最後の引数を ID とする

## list

既定の絞り込みも、表で現在のディレクトリのセッションに付ける `●` も、`session.CWDMatcher` で現在のディレクトリと比べる。パスが等しいか、同じディレクトリを指していれば一致とする。

## PR 連携

agentsview が `pr_links` として持つ、セッションに連携した PR を検索対象にする。連携ごとに `owner/repo#N`、`pull/N`、URL を足すので、`#N`・`pull/N`・`repo#N` のどれでも当たる。ツールの出力からは番号を拾わない（`gh pr list` の一覧のような関係の薄い番号まで当たるため）。連携の無いセッションの検索語は変わらない。

`list --json` は、連携があるセッションに `pr_links`（`url`・`host`・`repository`・`number` の配列）を出す。連携が無ければキーを出さない。

## 選択プロトコル

`list --fzf` は外部ピッカー向けのタブ区切り行を出す。フィールド順は source、時刻、名前、cwd、ブランチ、スニペット、検索用本文、ID。検索用本文と ID は ANSI conceal で隠す。検索用本文は、連携した PR の検索語（上の「PR 連携」）、会話本文の順に並べる。

## resume

`pick` と `resume` は、次の同じ手順でエージェントの CLI を起動する。

1. `source` の登録簿から argv を得る。登録の無いエージェントは resume できない。Claude Code・Codex・Cursor・Copilot 以外の登録は、agentsview の `internal/server/resume.go` の `resumeAgents` に合わせる
2. ID が登録簿の規則（既定は `^[A-Za-z0-9._-]+$`）に合うか確かめる
3. cwd が分かっていて、実在するディレクトリか確かめる
4. コマンドを PATH から探し、cwd と argv を stderr に出し、cwd で子プロセスとして実行する

`resume --dry-run` は手順4の代わりに cwd と argv を stdout に出す。

## ピッカー

- `/dev/tty` を開けなければエラーにする
- クエリを小文字にして空白で分け、すべての語が、表示項目・ID・連携した PR の検索語・会話本文の全文をつないで小文字にした文字列に含まれれば一致とする
- セッション由来の文字列は ANSI と制御文字を除いてから描く

## 色

- `NO_COLOR` があれば色を付けない。表は出力先が端末の時だけ色を付ける
- エージェントの色は `source` の登録簿、無ければ agentsview の配色、それも無ければ灰色
- 色は `ansitext.RGB` で持ち、SGR への変換は `ansitext.Paint`・`PaintOn` だけが行う
