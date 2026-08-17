---
title: SQL クエリフォーマット
description: .pw.sql 言語の全体。ステートメントの種類、パラメータの型、条件付き SQL、predicate と relation、そしてステートメントが拒否される検査。
sidebar:
  order: 3
---

`.pw.sql` は `pw generate` が Go にコンパイルする型付きクエリ言語です。中に書いた SQL は
SQL のままで、翻訳も書き換えも移植もされません。一方で Go との境界は検査されます。
パラメータの型、結果のカラム、`WHERE` 句の有無は、どれもビルド時に決まります。

このページは言語の全体です。生成された層と生の接続のどちらを使うか、ステートメントが
どうやって接続を見つけるかは[クエリー](/ja/guides/storage/queries/)と
[リレーショナルデータベース](/ja/guides/storage/rdb/)にあります。

## ファイルの構成

```sql
package queries

type User {
  id: int
  name: string
  active: bool
}

export statement GetUser(id: int): sql.one<User> {
SELECT id, name, active
FROM users
WHERE id = {id}
}
```

ファイルは、生成コードが属する Go パッケージから始まります。1つのディレクトリの `.pw.sql`
はすべて、そのディレクトリの `.pw.html` の出力と同じ `_pw_gen.go` にコンパイルされます。
生成が読むのは `popcornwave.toml` の `generate.queries` が挙げるディレクトリだけで、
どれにも属さない `.pw.sql` は黙って飛ばされるのではなく報告されます。

[コンポーネントパッケージ](/ja/guides/deployment/package/)では `generate.queries` が空で
なければなりません。生成されたクエリは1つのエンジンのプレースホルダ構文を持ちますが、
パッケージは利用側のエンジンを知りようがないからです。

| 宣言 | 導入するもの |
| --- | --- |
| `package name` | 生成ファイルが属する Go パッケージ |
| `type Name { field: T … }` | 結果の形。同名の Go 構造体になる |
| `statement name(…): kind { … }` | パッケージ内だけのステートメント |
| `export statement Name(…): kind { … }` | 同じものを Go の API として公開する |

## ダイアレクト

プレースホルダのトークンは `popcornwave.toml` の `project.database` から来ます。`postgres`
なら `$1`、`$2`、…、`mysql` と `sqlite` なら `?` です。どちらでも書くのは `{name}` で、
生成されるシグネチャも同一なので、エンジンを変えると出力される SQL のテキストだけが変わり、
呼ぶ側は何も変わりません。

ダイアレクトが変えるのはそのトークン**だけ**です。それ以外は書いたまま生成された SQL に
届きます。`||` は `CONCAT` に書き換えられず、`ON CONFLICT` は `ON DUPLICATE KEY UPDATE` に
翻訳されず、MySQL に `RETURNING` が無いことも回避されません。その種の翻訳層は正しく見えて
微妙に壊れます——`||` は PostgreSQL と SQLite では連結ですが MySQL では論理和なので、
書き換えると述語が反転しうるのです。選んだエンジンに向けて書いてください。

したがって生成されたパッケージ1つが対応するエンジンは1つです。これは、PostgreSQL の本番に
対してテストで SQLite を使おうとする前に量る価値があります。両者は `RETURNING` と
`ON CONFLICT` を共有するので素朴な CRUD はしばしば通ります。ただ、通ったことを確かめる
ものは何も無く、動かしているパッケージは出荷するパッケージではありません。

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
| `T[]` | `[]T` |
| `T?` | `*T` |

この表は Go の型で止まります。ドライバも同意する必要があります。NULL がありうるところでは
オプショナル型を使ってください。必須の `string` が NULL を読むと、空文字列ではなくエラーに
なります。

2つの行には、ドライバの同意だけでは足りないものがあります。

- `url` のカラムは双方向ともテキストとして運ばれます。`url.URL` のパラメータは文字列形式で
  バインドされ、返ってきたカラムは解析し直されます。`database/sql` は構造体をバインドする
  ことも scan することもできないからです。
- `datetime`、`date`、`time` はドライバが `time.Time` を返すことを要求します。MySQL なら
  DSN の `parseTime=true`、SQLite ならドライバとカラムの宣言型次第です。SQLite 自身は
  日付型を持ちません。いずれにせよドライバの設定であって、ダイアレクトの選択では決まりません。

## ステートメントの種類

| 種類 | 契約 | 生成される結果 |
| --- | --- | --- |
| `sql.exec` | 行の結果を持たない | `sql.Result` |
| `sql.one<T>` | ちょうど1行 | `T`。0行は `sql.ErrNoRows`、複数行はエラー |
| `sql.optional<T>` | 0行または1行 | `*T`。0行は `nil, nil`、複数行はエラー |
| `sql.many<T>` | 0行以上 | `iter.Seq2[T, error]`。ためずに流す |
| `sql.predicate` | 再利用できる条件 | 無し——他のステートメントからだけ使える |
| `sql.relation<T>` | 型付きのサブクエリ | 無し——他のステートメントからだけ使える |

`sql.many` は1行ずつ scan して渡します。イテレータの裏にスライスはたまりません。range を
break すると背後の `sql.Rows` が閉じ、クエリ・scan・反復のエラーはすべてエラー値として
届きます。

```go
for user, err := range queries.ListActiveUsers(ctx, true) {
	if err != nil {
		return err
	}
	consume(user)
}
```

## パラメータ

本文の `{name}` はすべて、宣言されたパラメータを運ぶプリペアドステートメントのプレース
ホルダです。テンプレートの式が SQL のテキストへ連結されることはないので、値のバインドから
インジェクションの余地は生まれません。

```sql
export statement RenameUser(id: int, name: string): sql.exec {
UPDATE users
SET name = {name}
WHERE id = {id}
}
```

```go
statement, err := queries.BuildRenameUser(42, "Ada")
// statement.SQL  == "... SET name = $1 WHERE id = $2 ..."
// statement.Args == []any{"Ada", 42}
```

この保証は絶対で、その分の代償があります。手書きの `$1` や `?` は生成エラーですし、値の
パラメータは構造的な要素——テーブル名、カラム名、演算子、ソート方向——の代わりには決して
なれません。

### スライスの展開

スライスのパラメータは値のリストへ展開されます。

```sql
export statement FindUsers(ids: int[]): sql.many<User> {
SELECT id, name, active
FROM users
WHERE id IN ({ids})
ORDER BY id
}
```

空のスライスには妥当な描き方が無いので、ビルダは `IN ()` を出す代わりにエラーを返します。
空の場合を呼び出し側で扱うか、条件で別の SQL 構造を選んでください。

## 結果の型と SELECT のカラム

結果のフィールドの順序は SELECT や RETURNING のカラム順と一致し、各カラム名か別名は
フィールド名に対応している必要があります。生成が両方を検査するので、結果の型から離れた
SELECT リストはクエリではなくビルドを失敗させます。

```sql
type UserSummary {
  id: int
  displayName: string
}

export statement ListUsers(): sql.many<UserSummary> {
SELECT id, display_name AS displayName
FROM users
ORDER BY id
}
```

この検査が成り立つのは形が静的に分かる場合だけです。だから実行時の条件で SELECT や
RETURNING のカラムを増減させることはできません。どの分岐でも結果の形は同じにしてください。

## 条件付き SQL

```sql
export statement SearchUsers(name: string, activeOnly: bool): sql.many<User> {
SELECT id, name, active
FROM users
WHERE name = {name}
{if activeOnly}
  AND active = TRUE
{/if}
ORDER BY id
}
```

`{else}` も使えます。条件は `bool` である必要があります。プレースホルダを消費するのは
残った分岐だけなので、どの分岐が生き残っても番号と `Args` は揃ったままです。

## predicate と relation

private な `sql.predicate` は再利用できる条件です。

```sql
statement minimumID(id: int): sql.predicate {
id >= {id}
}

export statement FindRecentUsers(minimum: int): sql.many<User> {
SELECT id, name, active
FROM users
WHERE {minimumID(minimum)}
ORDER BY id
}
```

private な `sql.relation<T>` は `FROM subquery` や `JOIN subquery` で使える型付きの
サブクエリです。

```sql
statement activeUsers(minimumID: int): sql.relation<ActiveUser> {
SELECT id, name
FROM users
WHERE id >= {minimumID} AND active = TRUE
}

export statement ListActiveUsers(minimumID: int, name: string): sql.many<ActiveUser> {
SELECT active_users.id, active_users.name
FROM subquery activeUsers(minimumID) AS active_users
WHERE active_users.name = {name}
ORDER BY active_users.id
}
```

組み立ててもパラメータのリストは分断されません。サブクエリと外側の引数は1つのプレース
ホルダ列を共有し、最終的な SQL に現れる順に並びます。別名は明示で、小文字のスネークケース
です。再帰する relation は拒否されます。

どちらも export できず、どちらも自分の関数を生成しません。

## 2つの安全規則

### UPDATE と DELETE には WHERE 句が要る

句が空になりうるかどうかは実行時のデータではなくテンプレートの性質なので、証明は丸ごと
生成時に走り、生成コードにガードは入りません。

```sql
-- 拒否される: ある呼び出し経路が全行を消す
export statement UnsafeDelete(id: int, enabled: bool): sql.exec {
DELETE FROM users
{if enabled}WHERE id = {id}{/if}
}

-- 受理される: 句が空になる経路が無い
export statement SafeDelete(id: int, name: string, byID: bool): sql.exec {
DELETE FROM users WHERE {if byID}id = {id}{else}name = {name}{/if}
}
```

キーワードはステートメント自身のものである必要があります。サブクエリ、CTE の本体、文字列
リテラル、コメントの中の `WHERE` は要件を満たしません。同じ証明が動的な `SET` リストも
覆い——代入がすべて条件付きの UPDATE はエラーです——どの結果種別にも適用されるので、
`sql.one<T>` と宣言された `DELETE … RETURNING` も同じように証明されます。`sql.predicate`
が要件を満たすのは、それ自身がどの経路でも空にならないときだけです。

意図した全行 UPDATE・DELETE のための抜け道はありません。それは
[マイグレーション](/ja/productivity/migrations/)として書いてください。

### SELECT のカラムは結果の型と一致する

上で述べたとおりで、これは同じ考えのもう半分です。結果カラムを条件で変えられない規則と
合わせて、生成された構造体を、そのステートメントが返しうるすべての行の正確な記述に保ちます。

## `export` と名前の大小

`export` が決めるのは、そのステートメントがパッケージの公開 Go API に加わるかどうかです。
生成される関数の名前は宣言そのままなので、Go が読むのは名前自身の大小であり、それは
`export` と一致していなければなりません。

| 宣言 | 生成 | |
| --- | --- | --- |
| `export statement FindUser(…)` | `func FindUser(…)` | 公開 API |
| `statement findUser(…)` | `func findUser(…)` | パッケージ内。どこからでも呼べる |
| `export statement findUser(…)` | — | エラー: `export` は非公開の名前を公開できない |
| `statement FindUser(…)` | — | エラー: `export` 無しでその名前は公開になってしまう |

`sql.predicate` と `sql.relation` は例外です。実行されるのではなく他のステートメントの
ビルダに埋め込まれるので、自分の名前の関数を生成せず、大小の制約もありません。

## 生成されるシグネチャ

Popcorn Wave が生成するのは、宣言された名前での**コンテキスト解決**形です。公開された関数は
`*sql.DB` も `*sql.Tx` も取りません。実行子はコンテキストから来ます。だから同じ関数が
トランザクションの中でも外でも動きます。

```go
func Name(ctx context.Context, p ...P) (sql.Result, error)   // sql.exec
func Name(ctx context.Context, p ...P) (T, error)            // sql.one<T>
func Name(ctx context.Context, p ...P) (*T, error)           // sql.optional<T>
func Name(ctx context.Context, p ...P) iter.Seq2[T, error]   // sql.many<T>

func BuildName(p ...P) (sqlbind.Statement, error)            // export された全ステートメント
```

`p ...P` は対応づけられたテンプレートのパラメータです。private なステートメントは同じ組を
`name` と `buildName` で受け取ります。

`Statement` は生成パッケージごとではなく
`github.com/shibukawa/tinybind-go/sqlbind` に1回だけ宣言されているので、値はそのまま
パッケージ境界を越えます。

```go
type Statement struct {
	SQL  string
	Args []any
}
```

`BuildName` を使うのは、SQL のテスト、ログ行、独自のデータベース抽象です。

```go
statement, err := queries.BuildGetUser(42)
log.Printf("sql=%s args=%v", statement.SQL, statement.Args)
```

## どこで走るか

`.pw.sql` の中にデータベースを名指すものは何もありません。コンテキストが、通常のリクエスト
では有効な接続グループのプールを、`pw.Transaction` の中では進行中のトランザクションを運びます。

```go
err := pw.Transaction(r.Context(), func(ctx context.Context) error {
	if _, err := queries.InsertUser(ctx, name); err != nil {
		return err
	}
	return queries.RecordAudit(ctx, "user.created")
})
```

どこで走るかを何も言わないステートメントは既定のグループへ行きます。`pw.SelectDB`、
`pw.SelectWriteDB` が1つを固定します。単一のステートメントでも `pw.Transaction` 全体でも
同じです。生成された関数がトポロジを知ることは無いので、開発用の SQLite ファイル1つが
どのグループ名にも答えられます。
[リレーショナルデータベース](/ja/guides/storage/rdb/)と
[ランタイム API](/ja/reference/runtime/)を参照してください。

## JOIN の行をまとめる

JOIN は子ごとに親の行を返し直すので、どの結果種別の宣言でもその平坦化は元に戻せません。
`sqlbind.ScanRows[T]` があとから木を組み立て直します。対象は任意のクエリで、SQL テンプレート
は関係しません。

```go
type Organization struct {
	ID    int    `db:"organization_id" groupkey:""`
	Name  string `db:"organization_name"`
	Users []User
}

type User struct {
	ID   int    `db:"user_id" groupkey:""`
	Name string `db:"user_name"`
}
```

```go
rows, err := db.QueryContext(ctx, `
SELECT o.id AS organization_id, o.name AS organization_name,
       u.id AS user_id,         u.name AS user_name
FROM organizations o
LEFT JOIN users u ON u.organization_id = o.id
ORDER BY o.id, u.id`)
if err != nil {
	return nil, err
}
defer rows.Close()
return sqlbind.ScanRows[Organization](rows)
```

| 規則 | 内容 |
| --- | --- |
| `groupkey:""` | まとめる構造体の各段にスカラのフィールドをちょうど1つ |
| `db:"alias"` | スカラのフィールドが読むカラム別名。タグが無ければ Go のフィールド名のスネークケース |
| ルートキーが同じ | 行が1つのルートオブジェクトへまとまる |
| 子のキーが同じ | 行が1つの子オブジェクトへまとまる |
| 子のキーが NULL | その子は不在。外部結合が意味するとおり |
| ルートキーが NULL | エラー |

使いどころを決めるのは2つの制約です。`ScanRows` は `database/sql` を使うホストの Go を
対象としていて、**TinyGo のビルドからは除外されます**。そして木を組み立てるために結果の
行をすべて消費するので、非常に大きな結果はメモリに載ります。普通のクエリ——行が1つずつ
流れていくもの——には `sql.one`・`sql.optional`・`sql.many` を、JOIN が同じ親を繰り返し、
その親を丸ごと受け取りたいときには `ScanRows` を使ってください。

## よくあるエラー

生成時:

- 手書きの `$1` や `?` のプレースホルダ
- 結果の型と食い違う SELECT のカラム数や名前
- 条件で増減する SELECT や RETURNING のカラム
- 証明できる `WHERE` を持たない UPDATE や DELETE
- `{if …}` の `bool` でない条件
- 再帰する `sql.relation`
- ステートメント名の大小と食い違う `export`
- コンポーネントパッケージの中の `.pw.sql`

実行時:

- 展開される値のリストに渡された空のスライス
- `sql.one` に0行または複数行、`sql.optional` に複数行
- `sql.many` を range しながら無視されたクエリエラー

`dev` では生成されたステートメントがすべて所要時間つきで記録され、しきい値より遅いものは
クエリプランと貼り付けて再実行できる断片を伴います。
[クエリー診断](/ja/productivity/query-diagnostics/)を参照してください。
