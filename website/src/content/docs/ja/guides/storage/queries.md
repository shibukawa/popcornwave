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
[`pw prepare`](/ja/pw/project/prepare/) はその同じ作業をコンパイラの手前で止めたもので、
TinyGo や自分で書いた `go build` がコンパイルを持つ場合に使います。手で 1 回走らせる
なら `pw generate` です。

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

### 接続詞は自分で持つ

偽になった分岐は、そのテキストを落とすだけです。上の例で `AND` がどこにあるかを見て
ください。ブロックの内側にあるので、繋いでいる述語と一緒に消えます。これを外に出したり、
*最初の*述語を条件付きにしたりすると、条件が偽のときに接続詞だけが residue として
残ります。

```sql
WHERE
{if byTitle}
  title = {title}
{/if}
{if onlyDone}
  AND done = TRUE
{/if}
```

`byTitle` が偽なら、これは `WHERE AND done = TRUE` を組み立てます。両方偽なら、`WHERE`
の直後に `ORDER BY` が続きます。どちらも捕まりません。生成は成功し、ビルドも通り、
書いたとおりの文がそのまま送られるので、返ってくるのはリクエスト時のデータベースの
構文エラーです。先頭の `AND` を削るパスはありませんし、あったとしても、書きかけの条件が
どの演算子を意図していたかを推測することになります。

これを避ける習慣は、最初の例がやっていることそのものです。**接続詞は、その述語を持つ
ブロックの内側に書いてください。**残るのは最初の述語をどうするかで、答えは 2 つあります。
必ず存在する条件があるなら——所有アカウント、テナント、論理削除フラグ——それを無条件で
先頭に置き、任意の述語には全部 `AND` を持たせます。すべての述語が本当に任意なら、
節そのものを固定します。

```sql
WHERE 1 = 1
{if byTitle}
  AND title = {title}
{/if}
{if onlyDone}
  AND done = TRUE
{/if}
```

`1 = 1` はプランナにとって無償ですし、すべての分岐が同じ形になります。3 つ目の条件が
増えたときに読みやすいのも、この形です。

UPDATE と DELETE だけは例外で、こちらは厳しくなります。WHERE が空になりうる更新系は
生成時に拒否されます。そこでの失敗は構文エラーではなく、全行更新だからです。

```
queries/todos.pw.sql:41:1: UPDATE and DELETE statements require a WHERE clause that is non-empty on every branch
```

この規則は非対称さで覚えるのが早いはずです。SELECT では余った接続詞がデータベースまで
届き、更新系ではコンパイラまで届きません。

### 結果の形は変えられない

型付きの結果から生まれる制約です。条件付きの SELECT や RETURNING の列は、すべての分岐を
記述できる単一の生成型がなくなるため拒否されます。

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
まとめる `ScanRows`——言語の全体は[SQL クエリフォーマット](/ja/reference/sql-templates/)に
あります。

スキーマと初期データは、ここまでのステートメントとは別の関心事です。開発支援の側に
[データベースマイグレーション](/ja/productivity/migrations/)と
[シードデータ](/ja/productivity/seed-data/)としてまとまっています。
