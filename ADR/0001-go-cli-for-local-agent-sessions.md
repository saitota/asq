# ローカルのエージェントセッションを横断して resume する Go CLI を作る

- 状態: 採用
- 日付: 2026-05-26

## 背景

- 目的は、各コーディングエージェントが手元に残したセッションを見つけて検索できる一覧に出し、選んだものを resume すること
- 同じ目的のツールとして recall があり、gwq はワークツリーを探して移動する CLI だった

## 決定

- Go の単一バイナリの CLI にし、主なコマンドは gwq にならって `reindex`、`list`、`resume` とする
- 出力は人向けの表、スクリプト向けの JSON、ピッカー向けのタブ区切り行
- resume は各エージェントの CLI を、セッションの cwd で子プロセスとして起動する。起動前に ID の文字種と cwd の存在を確かめる
- 対象は Claude Code、Codex CLI、Cursor CLI から始め、Copilot CLI も加える。Factory、OpenCode、tmux 連携は初期対応から外す
- 配布は GitHub Releases の zip にし、バージョンはビルド時に埋め込む

## 結果

セッションの取得は [0002](0002-native-parsers-and-index.md)、対話選択は [0003](0003-fzf-picker.md) で決め、どちらも後に置き換えた。
