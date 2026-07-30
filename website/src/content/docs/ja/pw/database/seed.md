---
title: pw seed
description: シードデータセットを設定済みのデータベースに読み込む。
sidebar:
  order: 2
---

```sh
pw seed [--dir=testdata/seed] [name...]
```

シードデータとは何か、どこからが[マイグレーション](/ja/productivity/migrations/)では
なくなるのかは[シードデータ](/ja/productivity/seed-data/)にあります。

シードデータは意図したデータベースに入って初めて役立ちます。`pw seed` は、
アプリケーション自身の設定から解決したデータベースにデータセットを適用します。

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

1 つのファイルに複数のテーブルを書けます。行が互いを参照する場合、ファイル内でも、
コマンドに渡すデータセット名の間でも順序が重要です。

## テストとの共有

テストヘルパーも `testutil.WithSeed` でまったく同じファイルを読みます。CLI と
テストスイートが 2 つのフィクスチャを管理する代わりに、1 つを共有します。

```go
server := testutil.TestRun(t, Handlers(), nil,
	testutil.WithMigrations("../migrations"),
	testutil.WithSeed("initial"),
)
```

[テスト](/ja/productivity/testing/)を参照。

## DSN の取得元

[`pw migrate`](/ja/pw/database/migrate/) と同様に、`pw` は設定の優先順位を複製せず、
解決済み DSN をアプリケーションに尋ねます。エラーではその値をマスクしますが、SQL の
方言は DSN から判定されます。

この一貫性はリスクにもつながります。シード投入は `APP_ENV` が選んだ環境に従うため、
開発用でないデータベースを対象にする前に環境を確認してください。
