# resume の登録をユーザー定義ファイルで足せるようにする

- 状態: 採用
- 日付: 2026-11-12
- 関連: [0008](0008-resume-agents-from-agentsview.md), [今の仕組み](../DESIGN/cli.md)

## 背景

- [0008](0008-resume-agents-from-agentsview.md) で、resume のコマンドと色を agentsview の内部の表から `source.go` に写した。上流は公開契約にしていない表なので、写しはずれる。直すにはソース変更とリリースが要る
- 上流 a078a250（2026-11-08）との差:
  - `resumeAgents` に 2026-08-10 以降に4つ（TraeX、Pi、Augure Code、Open Interpreter）が足された。asq に無かった `open-interpreter` は、一覧には出るが選ぶと `resume is not supported for open-interpreter sessions` になる
  - `agents.ts` は 2026-09-10 以降に12回変わった。asq には14個のエージェントの配色が無く、灰色で出る

## 決定

- 利用者が resume の登録を足せる、上書きできるファイル `agents.json` を設ける
- 場所は `$ASQ_AGENTS_FILE`。未設定なら `os.UserConfigDir()` の下の `asq/agents.json`。環境変数が優先する
- エントリは `source`、`command`（argv。要素の中の `{id}` をセッション ID に置き換える）、省略できる `color`（`#RRGGBB`）を持つ
- 組み込みと同じ `source` は、ユーザー定義がコマンドを置き換える。`color` を省くと組み込みの色が残る
- ID の検証は、ユーザー定義も組み込みと同じ既定の規則を通す
- ファイルが無ければ何もしない。読めない、JSON が不正、エントリが1つでも不正なら、ユーザー定義を全部使わず、stderr に警告を1行出して組み込みだけで動く
- 組み込みの表を a078a250 に合わせる。`open-interpreter` の resume と14個の配色を足し、`augure-code` の色を灰色から lime にする

## 却下した案

- 組み込みの表を合わせ直すだけにする: 上流が変えるたびに、ソース変更とリリースが要る
- 上流の HTTP API に問い合わせる: [0008](0008-resume-agents-from-agentsview.md) のとおり、web UI 向けで CLI からは呼べない

## 結果

- 上流に resume の対象や色が増えても、利用者はリリースを待たずに足せる
- 組み込みの表は a078a250 のまま止まる。ずれを見つける仕組みは無く、写し直す必要は残る
- ユーザー定義の argv は、書かれたとおりに実行する。差し込むのは既定の規則を通ったセッション ID だけ
