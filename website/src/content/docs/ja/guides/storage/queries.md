---
title: クエリ
description: 型付き .pw.sql ステートメント、条件付き SQL、そして設定済みの接続の上で走るトランザクション。
sidebar:
  order: 1
---

SQL は SQL のまま見えますが、Go との境界には型が付きます。ステートメントを
`.pw.sql` ファイルに書くと、`pw generate` が `context.Context` を取り、宣言した
結果型を返す関数へコンパイルします。

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

context が運ぶのはキャンセルだけではありません。通常のリクエストではプールを、
`pw.Transaction` の中では実行中のトランザクションを保持します。だからこそ同じ
生成関数が両方の場所で動きます。

## 型

| テンプレートの型 | Go の型 |
| --- | --- |
| `string`, `decimal` | `string` |
| `bool` | `bool` |
| `int` | `int` |
| `float` | `float64` |
| `bytes` | `[]byte` |
| `datetime`, `date`, `time` | `time.Time` |
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
したがって、値のバインディングからインジェクションに弱いクエリは作れません。

```sql
export statement FindUser(id: int): sql.one<User> {
SELECT id, name FROM users WHERE id = {id}
}
```

この保証は厳密な境界に支えられています。パラメータがバインドするのは**値**であり、
SQL の構造ではありません。テーブル名、カラム名、演算子、ソート方向は差し替えられません。

ジェネレータが出力するプレースホルダの構文 — PostgreSQL なら `$1`、MySQL と SQLite
なら `?` — は `popcornwave.toml` の `project.database` で決まります。書くのはどちらでも
`{name}` で、変わるのはコンパイル結果だけです。
[データベースを選ぶ](/ja/pw/project/init/#データベースを選ぶ)を参照。

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

型付きの結果から 1 つの制約が生まれます。**結果の形は変えられません。** 条件付きの
SELECT や RETURNING の列は、すべての分岐を記述できる単一の生成型がなくなるため
拒否されます。

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

トランザクション境界は常に明示的で、フレームワークがリクエストを自動的に包むことは
ありません。それでもネストは可能です。内側の `pw.Transaction` はセーブポイントを開き、
失敗時には内側の作業だけをロールバックして、外側のトランザクションを利用可能なまま
保ちます。セーブポイント対応が確認できないドライバでは、ネストを暗黙に潰さず
`ErrSavepointUnsupported` を返します。

生成レイヤーに収まらないクエリのために、生のアクセスも用意されています。

```go
db, ok := pw.DB(r.Context())
```

SQLite と MySQL では、これがプールそのものを返します。PostgreSQL では `ok` は `false`
です。リクエストは pgx のネイティブプールで走っていて、その背後に `*sql.DB` は存在
しません。`database/sql` のロックをクエリ経路から外すというのは、そういうことです。
生成されたステートメントと `pw.Transaction` はどのエンジンでも同じに動くので、まず
そちらを使ってください。サードパーティのライブラリが自前のハンドルを必要とする場合は
[相互運用](/ja/appendix/interoperability/)にあります。

## どの接続で走るのか

ここまでのどこにもデータベースは出てきません。どこで実行するかを言わないステートメントは
既定の接続グループへ行きます。書き込みや、リーダー・ライター構成に対するトランザクションが
グループを固定する手段が `pw.SelectDB` です。接続そのものと一緒に
[リレーショナルデータベース](/ja/guides/storage/rdb/)にあります。`[middleware.rdb]` セクション、
DSN のスキーム、エンジンごとに必要なインポートもそちらです。

生成された関数はトポロジを知りません。だからそれについて何も言わずに済みます。開発用の
SQLite ファイル 1 つはどのグループ名にも応えるので、上のコードはクラスタに対しても、その
ファイル 1 つしか無い環境に対しても、同じまま動きます。

## 実行されたクエリーを見る

`dev` では、生成されたステートメントがすべて所要時間とともに記録されます。しきい値を
超えたものには実行計画と、貼り付けて再実行できるスニペットが付きます。コードは 1 行も
変える必要がありません。

```
level=WARN msg="sql executed" sql="SELECT name FROM items WHERE name = $1"
  duration=240ms operation=query driver=sqlite outcome=ok args=[alpha] slow=true
  explain="id=2 parent=0 detail=SCAN items"
  reproduction=".parameter set $1 'alpha'\nSELECT name FROM items WHERE name = $1;"
```

[スロークエリー診断](/ja/productivity/query-diagnostics/)を参照してください。


ステートメントの種類、生成されるシグネチャ、`export` と名前の大小の規則、JOIN 行を
まとめる `ScanRows`——言語の全体は[SQL テンプレート](/ja/reference/sql-templates/)に
あります。

スキーマと初期データは、ここまでのステートメントとは別の関心事です。開発支援の側に
[データベースマイグレーション](/ja/productivity/migrations/)と
[シードデータ](/ja/productivity/seed-data/)としてまとまっています。
