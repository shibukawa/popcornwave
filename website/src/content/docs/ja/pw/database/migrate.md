---
title: pw migrate
description: スキーママイグレーションの確認、適用、ロールバック。
sidebar:
  order: 1
---

```sh
pw migrate <action> [--dir <path>] [--dsn <dsn>] [--yes] [--dry-run]
```

## アクション

| アクション | 効果 |
| --- | --- |
| `status` | 適用済みと未適用のマイグレーション |
| `version` | 現在のスキーマバージョン |
| `up` | 未適用のマイグレーションをすべて適用 |
| `up-by-one` | 次の 1 件だけ適用 |
| `up-to <version>` | 指定バージョンまで適用 |
| `down` | 直近の 1 件をロールバック |
| `down-to <version>` | 指定バージョンまでロールバック |
| `create <name>` | 次のマイグレーションファイルを作成 |
| `validate` | マイグレーション一式を検査 |
| `snapshot` | 現在のスキーマを取得 |

`up-to` と `down-to` はバージョン引数が必須、`create` は名前が必須です。それ以外の
アクションは、位置引数を無視せず拒否します。

## オプション

| オプション | 効果 |
| --- | --- |
| `--dir <path>` | `migration.dir` を上書きするマイグレーションディレクトリ |
| `--dsn <dsn>` | アプリケーションの設定を上書きする対象データベース |
| `--yes` | 破壊的な操作をプロンプトなしで承認する |
| `--dry-run` | 何も変更せず、実行内容だけを報告する |

## マイグレーションファイル

ファイルは `migration.dir`（既定は `migrations/`）に置き、goose の形式を使います。

```sql
-- +goose Up
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE users;
```

```sh
pw migrate create add_email
```

は次の連番ファイルを書き出し、そのパスをプロジェクトルートからの相対で表示します。

## DSN の取得元

`--dsn` を指定しない場合、`pw` は設定の優先順位を再実装するのではなく**アプリケーション
自身**に DSN を尋ねます。したがってマイグレーションは常に、サーバーが開くのと同じ
データベースに対して実行されます。`APP_ENV` が選んだ TOML ファイル、それを上書きする
環境変数、さらにオプション、という順序です。

DSN はパイプ経由で返されます。プロセス一覧から見えてしまうプロセス引数に置かれることは
なく、`pw` が出力するエラーメッセージからもマスクされます。

これは、ほとんどの migrate アクションがプロジェクトを必要とする理由でもあります。
`--dir` と `--dsn` がなければ、尋ねる相手がいません。

## 開発中の扱い

[`pw dev`](/ja/pw/project/dev/) が起動時とマイグレーションディレクトリの変更時に未適用の
マイグレーションを適用するので、これらのコマンドは主に確認、ロールバック、デプロイの
ためのものです。自動適用は次で無効にできます。

```toml
[migration]
auto = false
```

## 関連

- [クエリとマイグレーション](/ja/guides/queries/) — SQL の書き方。
- [`pw seed`](/ja/pw/database/seed/) — スキーマ適用後のデータ投入。
