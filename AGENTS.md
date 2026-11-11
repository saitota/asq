# AGENTS.md

asq は、agentsview が集めたコーディングエージェントのセッションを一覧・検索・resume する Go 製 CLI。

## コマンド

```sh
task check                                    # ビルド、テスト、vet
go test ./internal/cli -run TestName          # 単体のテスト
ASQ_AGENTSVIEW_SMOKE=1 go test ./internal/agentsview   # 実物の agentsview との契約確認
task install-local                            # 現在のブランチを ~/.local/bin に配置
task demo                                     # demo/demo.gif を録り直す
```

## 構成

- `cmd/asq`: エントリポイント
- `internal/`: パッケージの責務と依存方向は [DESIGN/README.md](DESIGN/README.md) の表に従う
- `DESIGN/`: 現在の仕組み
- `ADR/`: 決定の経緯。連番で追加し、過去の ADR は書き換えない
- `demo/`: デモ録画用のスタブとダミーセッション

## 規約

- 内部パッケージを束ねるのは `internal/cli` だけ
- `session.Source` による分岐は `internal/source` にだけ置く
- stdout はコマンド出力、進捗と警告は stderr、ピッカーは `/dev/tty`
- agentsview の CLI 契約を模すのは `internal/agentsviewtest` だけ
- 挙動を変えたら `DESIGN/` を、方針を変えたら `ADR/` を同じ PR で更新する
- README は英語、`DESIGN/` と `ADR/` は日本語
- ピッカーの見た目を変えたら `task demo` で GIF を録り直す
