---
title: クエリ
description: 型付き .pw.sql ステートメント、条件付き SQL、そして設定済みの接続の上で走るトランザクション。
sidebar:
  order: 1
---

SQL は SQL のまま見えますが、Go との境界には型が付きます。ステートメントを
`.pw.sql` ファイルに書くと、`pw generate` が `context.Context` を取り、宣言した
結果型を返す関数へコンパイルします。

## コード生成

`.pw.sql` の中の SQL が、リクエストのたびにパースされることはありません。
`pw generate` がファイルごとに隣の `_pw_gen.go` へコンパイルし、アプリケーションが
呼ぶのは生成された関数です。そのファイルはビルド出力で、Git は無視し、生成し直せば
作り直されます。

走らせ方は 3 つあります。`pw dev` はプロジェクトのソースを監視していて、変わるたびに
生成し直し、リビルドして再起動します。`pw build` はコンパイルの前に生成します。
[`pw generate`](/ja/pw/project/generate/) はその同じ作業をコンパイラの手前で止めた
もので、TinyGo や自分で書いた `go build` がコンパイルを持つ場合に使います。手で 1 回
走らせるときも同じコマンドです。

走査の対象はモジュール全体ではありません。`popcornwave.toml` が目的ごとに
ディレクトリを挙げていて、`.pw.sql` は `queries` の目的に属します。

```toml
[generate]
queries = ["queries"]
```

このディレクトリは再帰的に歩きます。外に置いた `.pw.sql` は、実行を失敗させるのでは
なく報告して飛ばすので、フィクスチャをコードの隣に置いておけます。

```
pw: samples/report.pw.sql is outside generate.queries and is not generated from; list its directory to include it
```

SQL を 1 つも持たないプロジェクトも、`queries = []` としてキー自体は書きます。空の
リストは次に読む人にも見える判断ですが、キーの書き忘れはエラーです。目的の一覧は
[`pw generate`](/ja/pw/project/generate/)にあります。

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

どの入力欄も任意な検索フォームが、これの出番です。条件がすべて成立したときに欲しい
ステートメントを書き、そこから条件を抜き出します。

```sql
export statement SearchUsers(
  name: string, city: string, minAge: int,
  hasName: bool, hasCity: bool, hasAge: bool
): sql.many<User> {
SELECT id, name, city, age
FROM users
WHERE
  {if hasName}name LIKE {name}{/if}
  AND {if hasCity}city = {city}{/if}
  AND {if hasAge}age >= {minAge}{/if}
ORDER BY id
}
```

`AND` を管理する必要はありません。読みながら `{if}` の囲みを消していけば、残るのが
レンダされる SQL です。それが目指す形です。`hasCity` だけを立てると `WHERE city = $1`
になります。余る演算子は出力されず、`city` は `$2` ではなく `$1` です。どれも立てなければ
`WHERE` も消えます。余らない演算子は書いた位置にそのまま出るので、いまあるステートメントの
挙動は変わりません。

### 演算子を置く場所

上の例のように、条件の内側ではなく 2 つの条件の間に置いてください。どちらも同じように
動きます——`{if hasCity}AND city = {city}{/if}` は古いテンプレートの形で、いまも正しく
レンダされます。ただ、分岐の内側の演算子は、2 つを繋ぐものがその 1 つの条件の一部として
読めてしまいます。テンプレートがその出力として読めること自体が目的なので、そこは効きます。

`WHERE 1 = 1` で節を固定して各述語に `AND` を持たせていたなら、その必要はもうありません。
むしろ外した方がよく、固定された節は空にならないので `WHERE` が落ちなくなります。

### コンマと、部分的な書き込み

同じ仕組みがコンマも管理するので、部分的な UPDATE と部分的な INSERT が普通に書けます。

```sql
export statement AddUser(id: int, name: string, city: string, withCity: bool): sql.exec {
INSERT INTO users (id, name{if withCity}, city{/if})
VALUES ({id}, {name}{if withCity}, {city}{/if})
}
```

上のように、カラムとその値は同じ条件で囲んでください。どこかの分岐で食い違いうるなら、
本番でデータベースに拒否させるのではなく生成が指摘します。

### 結果の形は変えられない

型付きの結果から生まれる制約です。条件付きの SELECT や RETURNING の列は、すべての分岐を
記述できる単一の生成型がなくなるため拒否されます。

これを前提に設計する前に知っておく価値のある限界がもう 1 つあります。`CASE` のアームには、
何も出力しえない断片を置けません。一緒に出力を控えられるキーワードもセパレータも無いから
です。条件に `{else}` を与えるか、`CASE` 全体を条件の内側に入れてください。

条件ではなく**構造**が変わるとき——JOIN の組み合わせが違う、結果の形が違う——は別の手を
使ってください。正直な名前を持つ 2 つのステートメントの方が、フラグ 6 個の 1 つより優れて
いますし、そもそもここでカラムリストを変えることはできません。どの句が管理されるかの網羅的な
一覧は[リファレンス](/ja/reference/sql-templates/#条件の間の演算子とコンマ)にあります。

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

**UPDATE と DELETE には WHERE が必須です。** 句が空になりうるかどうかはテンプレートの
性質で、実行時のデータの性質ではありません。だから証明はすべて生成時に走ります。どれか
一つでも WHERE が空になる分岐があるステートメントは `pw generate` が拒否し、実行時に
もう一度確かめるコードは生成物に何も入りません。条件付きの WHERE そのものは問題なく、
すべての分岐がそれを埋めるなら通ります。`{if}`/`{else}` の対がそれです。片方の分岐が
空のままにするなら拒否されます。

```
queries/todos.pw.sql:41:1: UPDATE and DELETE statements require a WHERE clause that is non-empty on every branch
```

前述の句の省略は更新系には及びません。そこでの失敗はデータベースが報告する構文エラーでは
なく、受理されてしまう全行更新だからです。意図的な全行更新のためのオプトインはありません。
それは[マイグレーション](/ja/productivity/migrations/) として書いてください。

**SELECT の列は結果型と一致しなければなりません。** 順序も、名前またはエイリアスも
一致が必要です。条件付き SELECT 列の禁止と組み合わさることで、生成された構造体は
そのステートメントが返しうるすべての行を正確に記述したものになります。

## トランザクション

```go
err := pw.Transaction(r, func(ctx context.Context) error {
	if _, err := queries.InsertUser(ctx, name); err != nil {
		return err
	}
	return queries.RecordAudit(ctx, "user.created")
})
```

トランザクション境界は常に明示的で、フレームワークがリクエストを自動的に包むことは
ありません。リクエストの開始でトランザクションを開き、終了でコミットするフレームワークは、
まれな用途のために、通常の処理にも余分なコストを発生させます。1行だけ読むページや、
1行だけ書くハンドラでも不要な `BEGIN` と `COMMIT` が実行され、接続もステートメントの
実行中だけでなく、リクエスト全体にわたって占有されます。ここでは、明示的にトランザクションを
開始した場所でだけ、このコストが発生します。

理由のもう半分は、ベンチマークより長持ちします。トランザクションは、データベースが本当に
得意なことを見せてくる場所でもあるからです。分離レベル、レプリカが引き受けられる読み取り
専用トランザクション、セーブポイント、遅い外部呼び出しの前にコミットするか後にするかの
判断。境界を開け閉めしてくれるレイヤーは、それら全部について 1 つの振る舞いを選ばなければ
ならず、選ばれるのは安全側の既定値です。境界をアプリケーションに残しておけば、これらの
選択肢は手の届く場所にあり続けます。

それでもネストは可能です。内側の `pw.Transaction` はセーブポイントを開き、
失敗時には内側の作業だけをロールバックして、外側のトランザクションを利用可能なまま
保ちます。セーブポイント対応が確認できないドライバでは、ネストを暗黙に潰さず
`ErrSavepointUnsupported` を返します。

生成レイヤーに収まらないクエリのために、生のアクセスも用意されています。

```go
db, ok := pw.DB(r)
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
まとめる `ScanRows`——言語の全体は[SQL クエリフォーマット](/ja/reference/sql-templates/)に
あります。

スキーマと初期データは、ここまでのステートメントとは別の関心事です。開発支援の側に
[データベースマイグレーション](/ja/productivity/migrations/)と
[シードデータ](/ja/productivity/seed-data/)としてまとまっています。
