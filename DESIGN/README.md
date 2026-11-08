# 設計

asq は、agentsview が集めたコーディングエージェントのセッションを取得してローカルにキャッシュし、一覧・検索・resume を提供する。決定の経緯は [ADR](../ADR/) にある。

## パッケージ

| パッケージ | 責務 |
|---|---|
| `internal/cli` | サブコマンド、出力（表・TSV・JSON）、ピッカー行とプレビュー、resume。`list --fzf` の行レイアウトは `selection.go` に、`list --json` の出力形は `json.go` にだけ置く |
| `internal/tui` | 対話ピッカー |
| `internal/agentsview` | agentsview からの取得とメッセージキャッシュ。保存形式は `cache.go` にだけ置く |
| `internal/progress` | 同期進捗と警告の stderr 描画 |
| `internal/source` | 表示色と、resume できるエージェントの登録簿（argv と ID の検証）。`session.Source` で分岐するのはここだけ |
| `internal/session` | 共通モデル、正規化、絞り込み、ID 解決、cwd の一致判定 |
| `internal/ansitext` | ANSI エスケープの除去と、ANSI を含む文字列の幅計算・切り詰め・折り返し・幅揃え。エスケープの文法を知るのはここだけ |
| `internal/buildinfo` | バージョン表示 |
| `internal/agentsviewtest` | テスト用の agentsview スタブ。CLI 契約を模すのはここだけ |

内部パッケージを束ねるのは `cli` だけ。`agentsview` と `source` は `session` に、`tui` は `ansitext` に依存する。`progress` は内部パッケージに依存せず、`agentsview.Reporter` を暗黙に満たす。

## 処理の流れ

1. `cli` が `agentsview.Client.Sessions()` で全セッションを得る
2. `session.Filter` で絞り込み、新しい順に並べる
3. `list` は出力する。`pick` と `resume` はセッションを選び、そのエージェントの CLI をセッションの cwd で起動する

stdout はコマンド出力に使う。進捗と警告は stderr、ピッカーは `/dev/tty` に描く。

## 文書

- [agentsview.md](agentsview.md): 取得、前提、キャッシュ、劣化動作、進捗
- [cli.md](cli.md): 振り分け、セッション指定、list、選択プロトコル、resume、ピッカー、色
