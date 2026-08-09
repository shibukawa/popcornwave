---
title: バッチ
description: 大量のステートメントのコストを削る — SQLite ではトランザクション、PostgreSQL では pgx のパイプライン、そして MySQL が代わりに要求するもの。
sidebar:
  order: 2
---

500 行を入れるインポートは、生成されたステートメントを 500 回走らせます。1 回ずつは
十分に速いのに、ループは遅い。使っているのがクエリの時間ではないからです。

どこにコストが行っているかで打ち手が決まり、そしてそれはエンジンごとに違います。
まずトランザクションから始めてください。SQLite ではそれが答えのすべてで、
PostgreSQL ではバッチに意味を持たせるための前提になります。

## まずトランザクション

トランザクションの外にあるステートメントは、それぞれが 1 つのトランザクションです。
SQLite はその都度 fsync を払います。ループを包めば、残るのは 1 回だけです。

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
このページはここで終わりです。

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

	"github.com/shibukawa/popcornwave/database/postgres"
	"github.com/shibukawa/popcornwave/pw"
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

手を伸ばす前に代金を読んでください。MySQL にパイプラインは無いので、パッケージは
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

ステートメントが 1 つのテーブルへの INSERT なら、
`INSERT INTO items (name) VALUES ($1), ($2), ($3)` がこのページのどの選択肢にも勝ちます。
パースは N 回ではなく 1 回、往復は MySQL を含むどのエンジンでも 1 回、逃げ道も DSN の
変更も不要で、しかも `.pw.sql` のスライス展開が書いてくれます。バッチを使うのは、
ステートメントが本当に異なるときです。

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

`WithConn` が届くのはバッチだけではありません。大量投入の `CopyFrom`、`LISTEN` と
`NOTIFY`、SQLSTATE を読むための `*pgx.PgError` への `errors.As` も、同じコールバックの
先にあります。PostgreSQL のデプロイが届く範囲の残りは
[相互運用](/ja/appendix/interoperability/)に、ここでの例が外へ出た生成レイヤーは
[クエリ](/ja/guides/storage/queries/)にあります。
