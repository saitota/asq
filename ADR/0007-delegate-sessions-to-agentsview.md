# セッションの取得を agentsview に委ねる

- 状態: 採用（[0002](0002-native-parsers-and-index.md) を置き換え。resume の対象は [0008](0008-resume-agents-from-agentsview.md) で広げた）
- 日付: 2026-09-01
- 関連: [今の仕組み](../DESIGN/agentsview.md)

## 背景

- 自前のパーサ・発見・インデックス層が約2150行あり、各エージェントのログ形式の変更に追従し続けていた
- agentsview は多数のエージェントのセッションを正規化して保持し、`session` コマンド群の JSON を、フィールドは追加のみで型を変えない契約として文書化している
- Go ライブラリとしては import できず、内部の SQLite もスキーマが保証されない

## 決定

- agentsview の `session` コマンドをセッションの唯一の取得元にし、自前のパーサ・発見・インデックスを削除する
- 会話本文はローカルにキャッシュし、検索は asq 内で行う
- 一覧には agentsview が知る全エージェントを出し、resume は従来の4エージェントに限る

## 却下した案

- 自前パーサを残してバックエンドを選べるようにする: opt-in のバックエンドとして実装を始めたが、asq を picker と resume に専念させる方針でマージ前に撤去した
- cass（Rust）や claude-code-log（Python）のパーサを再利用する: agentsview と同じく Go に組み込めず、対応エージェント数と API の成熟度で agentsview に劣る
- agentsview の SQLite を直接読む: スキーマが保証されない
- 検索を agentsview の `session search` に委ねる: 検索結果のシークレットがマスクされ、原文と一致しないことがある

## 結果

- 実行時に agentsview が必要になり、外部依存の無い単一バイナリではなくなった
- セッション名と本文の抽出は agentsview の正規化に従うため、旧パーサの結果とは一致しない。セッションの元ファイルのパスは扱わない
- `pick --print`、`--home`、`reindex --source`、`--backend` を削除した。`reindex` はキャッシュを温める用途になった
