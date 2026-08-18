---
title: バッチ
description: 大量のステートメントを効率よく実行する方法。SQLite のトランザクション、PostgreSQL の pgx Batch と COPY、MySQL での代替手段を説明します。
sidebar:
  order: 2
---

500行を読み込むインポート処理では、生成されたステートメントが500回実行されます。
各ステートメントは十分に速くても、ループ全体は遅くなることがあります。時間を使っているのが、
クエリの実行そのものとは限らないためです。

どこにコストが行っているかで打ち手が決まり、そしてそれはエンジンごとに違います。
まずはトランザクションを試してください。SQLite ではそれだけで解決し、
PostgreSQL ではバッチに意味を持たせるための前提になります。

## まずトランザクション

トランザクションの外にあるステートメントは、それぞれが 1 つのトランザクションです。
SQLite はその都度 fsync を実行します。ループ全体をトランザクションで囲めば、fsync は1回で済みます。

```go
err := pw.Transaction(r.Context(), func(ctx context.Context) error {
	for _, name := range names {
		if _, err := queries.InsertItem(ctx, name); err != nil {
			return err
		}
	}
	return nil
})
```

ドライバ側の計測では、SQLite ファイルへの 200 件の INSERT が、トランザクション無しで
およそ 50 ミリ秒、有りでおよそ 1 ミリ秒。どのバックエンドでも同じです。以下のどの手も
これを上回りません。償却すべきネットワークが無いからで、SQLite のバッチとは、
キューを前に置いたトランザクションのことです。データベースが SQLite なら、
SQLite を使う場合は、これ以上の最適化は必要ありません。

サーバエンジンでは、トランザクションは依然として開く価値があり、そして十分ではなく
なります。500 往復は 500 往復のまま残ります。

## PostgreSQL — 往復 1 回

pgx は返信を 1 つも読まないうちにステートメントの束を送り出せて、サーバはそれを
1 つの暗黙のトランザクションとして実行します。それには pgx の接続が要りますが、
`pw.DB` には渡せるものがありません。リクエストは pgx のネイティブプールで走っていて、
背後に `*sql.DB` が無いからです。そこへ届く道が `postgres.WithConn` です。

```go
package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornweb/database/postgres"
	"github.com/shibukawa/popcornweb/pw"
	"github.com/shibukawa/tinygodriver/database/pgx"
)

func ImportItems(w http.ResponseWriter, r *http.Request, names []string) {
	ctx, span := pw.StartSpanKind(r.Context(), "import-items", pw.SpanKindClient)
	defer span.End()

	err := postgres.WithConn(ctx, func(conn *pgx.Conn) error {
		batch := &pgx.Batch{}
		for _, name := range names {
			batch.Queue("INSERT INTO items (name) VALUES ($1)", name)
		}
		results := conn.SendBatch(ctx, batch)
		for range names {
			if _, err := results.Exec(); err != nil {
				_ = results.Close()
				return err
			}
		}
		return results.Close()
	})
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	pw.Logger(ctx).Info("imported", pw.Int("rows", len(names)))
}
```

キューに積んだ読み取りも同じ形で、`results.Query` と `results.QueryRow` を使います。
プリペアドステートメントは影響を受けません。pgx はバッチの中でも外と同じように
キャッシュして再利用します。

`pw.Transaction` の中で呼ぶと、コールバックはそのトランザクションが既に実行されている
接続を受け取ります。バッチはトランザクションに合流し、一緒にロールバックします。外で
呼べば、この呼び出しのためにプールから接続が 1 本借りられ、終わったら返されます。
どちらの場合も、接続から派生した値をコールバックより長く生かしてはいけません。行は
読み切り、結果は閉じてから返してください。返った瞬間に接続はプールへ戻ります。

PostgreSQL でないグループを渡すと、見つけたエンジンの名前を含むエラーが返ります。
pgx 向けに書かれたハンドラが SQLite のデプロイで静かに壊れるのではなく、はっきり
失敗するということです。

### PostgreSQL COPY — 1つのテーブルへ大量に入れる

バッチは往復を減らします。しかし PostgreSQL は、キューに積まれた `INSERT` を依然として
1文ずつパースし、実行します。行の投入先と列構成がすべて同じなら、その先に COPY が
あります。pgx の `CopyFrom` は、`COPY items (name, price_cents) FROM STDIN BINARY` に
相当する処理を組み立て、PostgreSQL の copy プロトコルで行の値を流します。

```go
rows := make([][]any, len(items))
for i, item := range items {
	rows[i] = []any{item.Name, item.PriceCents}
}

var copied int64
err := postgres.WithConn(ctx, func(conn *pgx.Conn) error {
	var err error
	copied, err = conn.CopyFrom(
		ctx,
		pgx.Identifier{"items"},
		[]string{"name", "price_cents"},
		pgx.CopyFromRows(rows),
	)
	return err
})
if err != nil {
	return err
}
pw.Logger(ctx).Info("copied items", pw.Int64("rows", copied))
```

`pgx.Identifier` はテーブル名を識別子としてクォートします。列リストの文字列も値を SQL
へ埋め込むものではなく、列の識別子です。入力全体を大きな `[][]any` にしてから渡したく
ない場合は、`pgx.CopyFromSource` を実装するか、`CopyFromSlice` または `CopyFromFunc` で
行を順に生成できます。

COPY は大量投入の経路であって、INSERT より多機能な文ではありません。行ごとの
`RETURNING` も `ON CONFLICT` もありません。デフォルト値を使う列は列リストから外します。
upsert や行単位の照合が必要なら、ステージングテーブルへ COPY したあと、同じ
`pw.Transaction` の中で `INSERT ... ON CONFLICT` を実行します。COPY が失敗すれば、同じ
`WithConn` コールバックから呼んだバッチと同様に、トランザクションと一緒にロールバック
されます。

アップロードされたファイルに対して、`STDIN` をファイルパスへ置き換えてはいけません。
`COPY FROM '/path'` が読むのはデータベースサーバー側のファイルシステムで、サーバー側の
権限も必要です。`\copy` は psql のコマンドであり、アプリケーションが送れる SQL では
ありません。アップロードはアプリケーションで解析・検証し、その行を `CopyFrom` へ
渡します。

### ここでの作業はクエリログに載らない

生成されたステートメントは、所要時間と実行計画と貼り付け可能な再実行コードつきで
記録されます。`WithConn` を通した作業はそうなりません。診断が取り付いている
エグゼキュータを迂回していて、ここを計測するにはコールバックが呼びうる pgx の
呼び出しを残らず包むことになるからです。

記録は自分で書きます。`pw.Logger` はコンテキスト上でアクティブなスパンを読むので、
上の例のスパンの中でログを 1 行書けば、それだけで相関します。配線はこれで全部で、
例がバッチより先にスパンを開いているのはそのためです。書く内容は、バッチにすると
後から復元しにくくなるもの — 何件送ったか、やり取りにどれだけかかったか。
ステートメントごとの時間は、手放した側です。

## MySQL — できる、ただし取引

MySQL では `pw.DB` がプールを返すので、ドライバ自身のバッチパッケージには
フレームワークの助けなしで届きます。

```go
db, ok := pw.DB(ctx)
if !ok {
	return errors.New("no pool on this connection")
}
batch := &sqlbatch.Batch{}
for _, name := range names {
	batch.Queue("INSERT INTO items (name) VALUES (?)", name)
}
return sqlbatch.Send(ctx, db, batch)
```

利用する前に、トレードオフを確認してください。MySQL にはパイプラインがないため、パッケージは
ステートメントを 1 つのマルチステートメントコマンドに連結します。それにはフレーム
ワークが意図的に既定へ入れていない DSN 設定が 2 つ要ります。`multiStatements=true`
と、引数を持つステートメントがあるときの `interpolateParams=true` です。前者は、
SQL テキストに届いた injection にできることを、そのデプロイの全接続にわたって広げ
ます。後者はバッチを成立させている当のもので、マルチステートメントコマンドは
prepare できないため、ドライバは引数をサーバ側でバインドせず SQL そのものに
埋め込みます。

細かさも手放します。バッチにできるのは書き込みだけ、サーバはコマンド全体に対して
1 つのエラーを返すので失敗したステートメントは通常特定できず、そして既に開いている
トランザクションには合流できません。

両方の DSN 設定に運用者が同意している、書き込みの重いインポートなら取る価値が
あります。そうでなければトランザクションのままで。こちらは何も要求せず、どこでも
使えます。

## バッチにしない方がいいとき

ステートメントが 1 つのテーブルへの INSERT なら、1 行につき 1 文をキューへ積むのは
やめます。件数がそれほど多くなければ、`INSERT INTO items (name) VALUES ($1), ($2), ($3)`
はパースが 1 回で、MySQL を含む全エンジンで使え、逃げ道も DSN の変更も不要です。
`.pw.sql` のスライス展開もこの形を書けます。PostgreSQL で同じ形の入力が多く、大量投入の
性能が問題になるなら `CopyFrom`。文が本当に異なるときに、バッチを選びます。

| 処理の形 | PostgreSQL での選択 |
| --- | --- |
| 行数がそれほど多くない、または `RETURNING` / `ON CONFLICT` が必要 | 複数行 `INSERT` |
| 1つのテーブルと列構成へ大量の行を投入 | `CopyFrom` |
| 結果を受け取りたい異なるステートメントの束 | 必要に応じてトランザクション内の `Batch` |

除外される場面がもう 2 つあります。PostgreSQL のバッチでは、サーバがどれも実行しない
うちにキュー全体をパースすることがあります。`CREATE TABLE` に続けてそのテーブルへの
INSERT を積むと、`pw.Transaction` の中なら通る同じ組み合わせが失敗します。DDL は
バッチに入れないでください。もう 1 つ、暗黙のトランザクションもトランザクションなので、
`VACUUM`、`CREATE DATABASE`、`CREATE INDEX CONCURRENTLY` は 2 文以上のバッチでは
拒否されます。

バッチが約束しないことが 1 つあります。中の読み取りが同じスナップショットを見るとは
限りません。PostgreSQL は `READ COMMITTED` でステートメントごとに新しいものを取り、
SQLite と MySQL は囲んでいるトランザクションで 1 つを共有します。バッチが約束するのは
順序と原子性です。ビューの一貫性はトランザクションの仕事で、素のバッチが走る暗黙の
トランザクションには分離レベルを設定できません。強い分離が要るなら、バッチを
`pw.Transaction` の中に置いてください。

`WithConn` が届くのはバッチだけではありません。`LISTEN` と `NOTIFY`、SQLSTATE を読む
ための `*pgx.PgError` への `errors.As` も、同じコールバックの先にあります。
PostgreSQL のデプロイが届く範囲の残りは
[相互運用](/ja/appendix/interoperability/)に、ここでの例が外へ出た生成レイヤーは
[クエリ](/ja/guides/storage/queries/)にあります。
