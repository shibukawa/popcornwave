---
title: クエリとマイグレーション
description: 型付き .pw.sql ステートメント、条件付き SQL、トランザクション、goose マイグレーション、シードデータ。
sidebar:
  order: 4
---

SQL は専用のソースファイルに書き、`pw generate` が Go にコンパイルします。生成される
関数は `context.Context` を取り、型付きの結果を返します。

## ステートメント

```sql
package queries

type AccessCounter {
  count: int
}

export statement IncrementAccess(): sql.one<AccessCounter> {
INSERT INTO access_counter (id, count)
VALUES (1, 1)
ON CONFLICT(id) DO UPDATE SET count = access_counter.count + 1
RETURNING count
}
```

- `type` が結果の形を宣言します。
- `export statement` が関数名、型付きパラメータ、結果の種別を宣言します。
- SQL 本体の `{name}` が宣言済みパラメータをバインドします。

```go
counter, err := queries.IncrementAccess(r.Context())
```

context は飾りではありません。プールを運び、`pw.Transaction` の中ではトランザクションを
運びます。だからこそ同じ生成関数が両方の場所で動きます。

## 型

| テンプレートの型 | Go の型 |
| --- | --- |
| `string`、`decimal` | `string` |
| `bool` | `bool` |
| `int` | `int` |
| `float` | `float64` |
| `bytes` | `[]byte` |
| `datetime`、`date`、`time` | `time.Time` |
| `url` | `url.URL` |

`T[]` はスライス、`T?` はオプショナルです。

## ステートメントの種別

| 種別 | 戻り値 |
| --- | --- |
| `sql.exec` | `sql.Result`。INSERT / UPDATE / DELETE 用 |
| `sql.one<T>` | `T`。0 行は `sql.ErrNoRows`、複数行はエラー |
| `sql.optional<T>` | `*T`。0 行は `nil, nil` |
| `sql.many<T>` | `iter.Seq2[T, error]`。蓄積せずストリーミング |
| `sql.predicate` | 再利用可能な非公開の条件。公開関数は生成されない |
| `sql.relation<T>` | 型付きの非公開サブクエリ。公開関数は生成されない |

`sql.many` がイテレータを返すのは大きな結果集合で効いてきます。行がいったんスライスに
集められることはありません。

```go
for user, err := range queries.ListUsers(ctx) {
	if err != nil {
		return err
	}
	// ...
}
```

## パラメータ

すべての `{name}` はプリペアドステートメントのプレースホルダになります。テンプレートの
式が SQL テキストに連結されることはなく、手書きのプレースホルダは拒否されます。
つまり構造上、クエリがインジェクションに弱くなりません。

```sql
export statement FindUser(id: int): sql.one<User> {
SELECT id, name FROM users WHERE id = {id}
}
```

パラメータがバインドするのは**値**であって構造ではありません。テーブル名、カラム名、
演算子、ソート方向を差し替えることはできません。

### スライスの展開

スライスのパラメータは `IN` のリストに展開されます。

```sql
export statement FindUsers(ids: int[]): sql.many<User> {
SELECT id, name, active
FROM users
WHERE id IN ({ids})
ORDER BY id
}
```

空のスライスはビルダーのエラーになります。呼び出し側で空の場合を処理するか、条件付き
SQL でクエリ自体を組み替えてください。

## 条件付き SQL

```sql
export statement SearchUsers(name: string, onlyActive: bool): sql.many<User> {
SELECT id, name, active
FROM users
WHERE name LIKE {name}
{if onlyActive}
  AND active = TRUE
{/if}
ORDER BY id
}
```

`{else}` も使えます。条件は `bool` でなければなりません。含まれた分岐だけがプレース
ホルダを消費するので、番号のずれは起きません。

厳しい制約が 1 つあります。**結果の形は変えられません。** 条件付きの SELECT や
RETURNING の列は拒否されます。生成された型がすべての分岐を記述できなくなるからです。

## predicate と relation

`sql.predicate` は再利用可能な WHERE 断片です。

```sql
statement MinimumID(id: int): sql.predicate {
  id >= {id}
}

export statement FindRecentUsers(minimum: int): sql.many<User> {
SELECT id, name, active
FROM users
WHERE {MinimumID(minimum)}
ORDER BY id
}
```

`sql.relation<T>` は FROM や JOIN で使える型付きサブクエリです。

```sql
statement ActiveUsers(minimumID: int): sql.relation<ActiveUser> {
SELECT id, name
FROM users
WHERE id >= {minimumID} AND active = TRUE
}

export statement ListActiveUsers(minimumID: int, name: string): sql.many<ActiveUser> {
SELECT active_users.id, active_users.name
FROM subquery ActiveUsers(minimumID) AS active_users
WHERE active_users.name = {name}
ORDER BY active_users.id
}
```

サブクエリと外側の引数は、最終的な SQL の順序で 1 つのプレースホルダ列を共有します。
エイリアスは明示的な小文字の snake_case です。再帰的な relation は拒否されます。

## 2 つの安全規則

**UPDATE と DELETE には WHERE が必須です。** まったく書かれていなければ生成が失敗し、
WHERE が条件付きで実行時に消えうる場合はビルダーが実行を拒否します。意図的な全行更新の
ためのオプトインはありません。それはマイグレーションとして書いてください。

**SELECT の列は結果型と一致しなければなりません。** 順序も、名前またはエイリアスも
一致が必要です。条件付き SELECT 列の禁止と組み合わさることで、生成された構造体は
そのステートメントが返しうるすべての行を正確に記述したものになります。

## トランザクション

```go
err := pw.Transaction(r.Context(), func(ctx context.Context) error {
	if _, err := queries.InsertUser(ctx, name); err != nil {
		return err
	}
	return queries.RecordAudit(ctx, "user.created")
})
```

境界は常に明示的です。フレームワークがリクエストを勝手にトランザクションで包むことは
ありません。ネストも可能で、内側の `pw.Transaction` はセーブポイントを開くため、その
失敗は自分の作業だけをロールバックし、外側のトランザクションは使える状態のままです。
セーブポイントに対応していないドライバでは、暗黙にネストを潰さず
`ErrSavepointUnsupported` で失敗します。

生成レイヤーに収まらないクエリのために、生のアクセスも用意されています。

```go
db, ok := pw.DB(r.Context())
```

## マイグレーション

マイグレーションは `migrations/` に置き、goose の形式を使います。

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
pw migrate status
pw migrate up
```

`pw dev` は起動時に未適用のマイグレーションを適用するので、日常の開発でこれらを直接
使う場面はあまりありません。アクションの一覧は
[pw migrate](/ja/pw/database/migrate/) にあります。

## データベースの設定

プールは `[middleware.rdb]` にあり、既定では**無効**です。

```toml
[middleware.rdb]
enabled = true
dsn = "sqlite://myapp.db"
connect_timeout = "5s"
max_open_conns = 1
max_idle_conns = 1
```

`dsn` は秘密情報として扱われ、設定ログでもエラーメッセージでもマスクされます。
[設定](/ja/guides/configuration/)を参照。

## シードデータ

シードファイルは `testdata/seed/` に置き、CLI とテストヘルパーの両方が使います。
そのためフィクスチャが両者の間でずれることはありません。

```yaml
member:
- { id: 1, name: Frank }
- { id: 2, name: Grace }
```

```sh
pw seed
```

[pw seed](/ja/pw/database/seed/) と[テスト](/ja/guides/testing/)を参照してください。
