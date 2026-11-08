# resume の対象を agentsview の resume に合わせる

- 状態: 採用
- 日付: 2026-11-08
- 関連: [0007](0007-delegate-sessions-to-agentsview.md)

## 背景

- 一覧には agentsview が知る全エージェントが出るが、resume できるのは Claude Code・Codex・Cursor・Copilot の4つだけだった
- agentsview は自分の resume メニューのために、エージェントごとの resume コマンドを `internal/server/resume.go` の `resumeAgents` に持っている
- その表を使うには `POST /api/v1/sessions/{id}/resume` を呼ぶ必要がある。これは web UI 向けの HTTP API で、CLI からは呼べない。ブラウザ以外から呼ぶと `Origin` ヘッダが無いため `Forbidden` になる

## 決定

- `resumeAgents` にあって asq に無いエージェント（Gemini、OpenCode、Amp、Kiro、Pi、TraeX、Augure Code）を `source` の登録簿に写す
- 既存の4エージェントのコマンドは変えない。Cursor は agentsview の `cursor agent --resume <id>` と違い、`cursor-agent --resume=<id>` のまま

## 却下した案

- HTTP API に resume コマンドを問い合わせる: web UI 向けで、`Origin` を偽装する必要がある。デーモンの URL も、人向けの `daemon status` の出力からしか取れない

## 結果

- agentsview が resume できるエージェントは、asq でも resume できる
- agentsview に resume 対象が増えても、asq には自動では入らない。表を写し直す必要がある
- ID の正規化は asq の既存の規則（`source_session_id`、無ければ `<agent>:` を外した `id`）のままにした。agentsview が Codex 系で行う `<thread>_<rollout>` からの thread ID の取り出しは写していない
