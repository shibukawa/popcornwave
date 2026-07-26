---
title: pw seed
description: シードデータセットを設定済みのデータベースに読み込む。
sidebar:
  order: 2
---

```sh
pw seed [--dir=testdata/seed] [name...]
```

アプリケーションが使うよう設定されているデータベースに、シードデータセットを適用します。

## 引数とオプション

| 引数 | 効果 |
| --- | --- |
| *(なし)* | シードディレクトリ内のすべてのデータセットを適用 |
| `name...` | 指定したデータセットのみを、与えた順に適用 |
| `--dir <path>` | シードディレクトリ。既定は `testdata/seed` |

名前はシードディレクトリからの相対パスで、`.yaml` 拡張子は省略できます。したがって
`pw seed users` と `pw seed users.yaml` は同じ指定です。

適用のたびにファイル名が報告されます。

```
seeding testdata/seed/users.yaml
seeding testdata/seed/orders.yaml
```

## データセットの形式

データセットは、テーブル名に行のリストを対応させたものです。

```yaml
member:
- { id: 1, name: Frank }
- { id: 2, name: Grace }
- { id: 3, name: Heidi }
```

1 つのファイルに複数のテーブルを書けます。行が互いを参照する場合は順序が重要です。
ファイル内でも、渡す名前の順序についても同様です。

## テストとの共有

これはテストヘルパーが `testutil.WithSeed` で読み込むのと同じファイルです。そのため
フィクスチャが CLI とテストスイートの間でずれることはありません。

```go
server := testutil.TestRun(t, Handlers(), nil,
	testutil.WithMigrations("../migrations"),
	testutil.WithSeed("initial"),
)
```

[テスト](/ja/guides/testing/)を参照。

## DSN の取得元

[`pw migrate`](/ja/pw/database/migrate/) と同様に、`pw` は設定の優先順位を再実装せず
アプリケーションに DSN を尋ね、報告するエラーからはマスクします。SQL の方言はその DSN
から判定されます。

シード投入は `APP_ENV` が選んだ環境に対して実行されます。開発用でないデータベースに
対して実行する前に、いまどの環境にいるかを確認してください。
