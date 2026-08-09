---
title: 相互運用性
description: どこまでがフレームワーク本体で、どこからが差し替え可能なヘルパーなのか。そしてそのヘルパーを他のフレームワークで使う方法。
sidebar:
  order: 1
---

Popcorn Wave はリクエストのパース、DB クエリ、HTML、レスポンスの書き出しを生成するため、
ランタイムが 4 つすべてに依存しているように見えるかもしれません。実際には違います。
フレームワークが必須にする範囲は、`net/http` の上に載るミドルウェア群と、生成・開発・
ビルドの一貫性を保つツール類だけです。生成レイヤーは差し替え可能な**ヘルパー**であり、
Popcorn Wave 本体を持ち込まずに別のフレームワークから使うこともできます。

## なぜこの層を自前で作ったのか

Go のライブラリの多くが `reflect` を使うのには、十分な理由があります。任意の構造体に
リクエストをバインドし、モデルに行をマップし、未知のデータをテンプレートが辿れるのは
リフレクションのおかげです。しかし、その柔軟性は **TinyGo** の厳しい制約にぶつかります。
サポートは部分的で、ビルドできてもバイナリサイズに跳ね返ります。TinyGo を本気の
ターゲットとして扱うなら、同じ能力を別の経路で得る必要があります。

そこで各ヘルパーを**コードジェネレータ**にしました。リフレクションならリクエストごとに
繰り返す仕事を、[`pw generate`](/ja/pw/project/generate/) が事前に一度だけ行います。
実際にバインドする型のバインダ、実際に書き出す型のエンコーダ、SQL が実際に SELECT する
カラムのスキャナ。ホットパスで型を実行時に覗くコードはありません。

性能より重要な帰結は、所有権にあります。ジェネレータが知るのはソースから読めるものだけ
ですが、フレームワークはその代償としてハンドラのシグネチャを奪いません。ハンドラは
`http.HandlerFunc` のままで、`w` と `r` も標準の型のままです。生成が合わない
エンドポイントは手で書けますし、そのビルドで TinyGo ターゲットを外れると承知のうえで、
reflect ベースのライブラリを使うこともできます。

## どこまでがフレームワーク本体か

| 部分 | 位置づけ |
| --- | --- |
| `middlewares` — リクエスト ID、リカバリ、ボディ上限、セキュリティヘッダ、タイムアウト、アクセスログ、アセット、OpenTelemetry | フレームワーク本体 |
| `pw.Middlewares` — 設定、起動時検証、DB プール、拡張、運用エンドポイント、組み上がったスタック | フレームワーク本体 |
| `pw.Run` | `pw.Middlewares` と `http.Server` のラッパー |
| `pw generate`, `pw dev`, `pw build`, `pw migrate`, `pw seed` | 開発体験 |
| `pw.Parse[T]` | 差し替え可能なヘルパー |
| `.pw.sql` ステートメント | 差し替え可能なヘルパー |
| `.pw.html` コンポーネント | 差し替え可能なヘルパー |
| `pw.WriteAPI` / `WriteHTML` / `NewStream` / `WriteProblem` | 差し替え可能なヘルパー |

ヘルパーは、よくある 4 つの仕事を少ないコードで片付け、生成される OpenAPI
ドキュメントの入力にもなります。しかし、その利点がランタイムの契約になるわけではありません。

## サーバーを自分で持つ

`pw.Run` は便利のためのラッパーであって、必須ではありません。フレームワークの初期化は
すべて `pw.Middlewares` の側にあり、返ってくるのは素の `http.Handler` です。

```go
handler, err := pw.Middlewares(handlers.Handlers())
if err != nil {
	log.Fatal(err)
}
log.Fatal(http.ListenAndServe(":8080", handler))
```

返された値がそのままエスケープハッチです。自前の `http.Server`、別のリスナー、
`httptest`、Lambda アダプタ、あるいは他のワークロードも配信するホストプロセスの
後ろに置けます。

`pw.Run` がその上で追加でやっていること、つまり自分で配信するなら自分で持つことになるものは
次のとおりです。

| `pw.Run` がやっていること | 自分で配信する場合 |
| --- | --- |
| `--generate-config` などのフレームワークフラグの処理 | 効かなくなる |
| `[server]` からポートと 4 つのタイムアウトで `http.Server` を組む | `pw.Config[pw.ServerConfig](nil)` を読んで必要な分だけ適用する |
| `SIGINT` / `SIGTERM` で `shutdown_timeout` 内にシャットダウン | 自前のシグナルハンドリングと `server.Shutdown` |
| 拡張のリソースを登録の逆順でクローズ | 現状これは公開 API になっていない。セッションや DB プールはプロセス終了で解放される |

テストや短命なプロセスならどれも問題になりません。長時間動くデプロイで再現する価値があるのは
シャットダウンの順序です。

## Popcorn Wave アプリの中でヘルパーを差し替える

### パースを自分で書く

```go
func createUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	// あるいは json.NewDecoder(r.Body).Decode(&input)
}
```

ハンドラはそのまま動きます。失われるのは生成が担っていた `check` ルール、フィールド
単位の problem 詳細、OpenAPI に載るリクエストスキーマです。変わったエンドポイント
1 本なら妥当な取引になりえますが、40 本では負担が大きくなります。

### sqlx、GORM、素の `database/sql` でクエリを書く

SQLite と MySQL では、プールは `*sql.DB` で、リクエストの context から取れます。

```go
db, ok := pw.DB(r.Context())
if !ok {
	pw.WriteProblem(w, r, pw.ServiceUnavailable("database unavailable"))
	return
}
users := sqlx.NewDb(db, driver) // または gorm.Open(postgres.New(postgres.Config{Conn: db}))
```

PostgreSQL では `ok` は常に `false` です。フレームワークはリクエストを pgx のネイティブ
プールで処理していて、貸し出せる `*sql.DB` が背後にありません。`*sql.DB` を要求する
ライブラリには専用のプールを与えてください。起動時に一度、同じ DSN で
`github.com/shibukawa/tinygodriver/database/pgx/stdlib` の `stdlib.Open` を呼び、
ライフサイクルも自分で持ちます。生成されたステートメントとトランザクションを共有すべき
書き込みは生成レイヤーの中に置いてください。2 つのプールが 1 つのトランザクションに
参加することはできません。

この 2 本目のプールは、`*sql.DB` でなければ困るコードのためのものです。pgx そのものが
欲しいコード — バッチ、`CopyFrom`、`LISTEN`/`NOTIFY`、SQLSTATE を読む `*pgx.PgError` —
は開く必要がありません。`postgres.WithConn` はフレームワークが既に使っている接続を
そのまま渡し、`pw.Transaction` の中ではそのトランザクションが走っている接続を渡すので、
作業は競合ではなく合流になります。[バッチ](/ja/guides/storage/batching/)を参照してください。

`pw.Transaction` はトランザクションを context に入れます。これは**生成された**ステートメントの
ためのもので、だから明示的なハンドルが要りません。他のライブラリからはそれが見えないので、
1 リクエストで両方を混ぜるときはトランザクション自体を取り出します。

```go
err := pw.Transaction(r.Context(), func(ctx context.Context) error {
	if _, err := queries.InsertUser(ctx, input.Name); err != nil {
		return err
	}
	executor, err := sqlbind.SQLExecutorFromContext(ctx)
	if err != nil {
		return err
	}
	tx, ok := executor.(*sql.Tx)
	if !ok {
		return errors.New("no transaction in context")
	}
	return audit(tx, "user.created")
})
```

代わりにプールから 2 本目のトランザクションを開いてはいけません。1 リクエスト内の
2 つのトランザクションは、コミットとロールバックの境界を共有しません。

`*sql.Tx` への型アサーションが成り立つのは SQLite と MySQL です。PostgreSQL で context に
入っているのは pgx のネイティブトランザクションなので、具体的なハンドルが必要なコードは
executor のインターフェイスに対してアサートすることになります。より良いのは生成レイヤーに
とどまることで、そこではアサーション自体が不要です。

### 別のテンプレートエンジンを使う

`pw.WriteHTML` は生成されたフラグメントを受け取り、登録済みのドキュメントシェルの中で
レンダリングします。別のエンジンを使うなら単純にその両方を通りません。テンプレートのエラーが
書きかけの 200 を残さないよう、まずバッファに書き出してください。

```go
var body bytes.Buffer
if err := tmpl.ExecuteTemplate(&body, "home.html", data); err != nil {
	pw.WriteProblem(w, r, err)
	return
}
w.Header().Set("Content-Type", "text/html; charset=utf-8")
w.Header().Set("Content-Length", strconv.Itoa(body.Len()))
w.WriteHeader(http.StatusOK)
body.WriteTo(w)
```

`html/template` と、その上に作られたテンプレートエンジンはリフレクションを使います。つまり
ここが TinyGo ビルドを諦める地点です。

### レスポンスを自分で書く

`json.NewEncoder(w).Encode(value)` も `http.Redirect` も、ふつうの `net/http` アプリケーションと
まったく同じように動きます。それでも `pw.WriteProblem` は残す価値があります。任意の `error` を
受け取り、5xx の詳細を漏らさず、すでにコミット済みのレスポンスに 2 つ目のペイロードを
足すことを拒否するからです。

## フレームワークなしでヘルパーだけを使う

生成される層はこのリポジトリのものではありません。このフレームワークのために作られ、単体でも
使える [`tinybind-go`](https://github.com/shibukawa/tinybind-go) が実体です。

| パッケージ | 提供するもの |
| --- | --- |
| `httpbind` | リクエストバインド、バリデーション、JSON レスポンス、ストリーミング、OpenAPI 3.1 |
| `htmlbind` | 型付き HTML コンポーネントとレンダーチェーン |
| `sqlbind` | 型付き SQL ステートメントと結果スキャン |
| `configbind` | 設定バインド、スキャフォールド、CLI サブコマンド |

生成はパッケージ単位で、プロジェクト構成ではなくディレクティブで駆動します。

```go
package api

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir . -openapi
```

Popcorn Wave プロジェクトの外では、テンプレートの拡張子は `.tb.html` と `.tb.sql` です
（`-html-template-pattern` と `-sql-template-pattern` で変更できます）。出力先は
`tinybind_gen.go` と `tinybind_templates_gen.go` です。

### 例: Echo の中でヘルパーを使う

```go
func createUser(c echo.Context) error {
	r, w := c.Request(), c.Response()

	// パスパラメータは Echo のルータが持つ。httpbind は r.PathValue を読む。
	r.SetPathValue("org_id", c.Param("org_id"))

	input, err := httpbind.Bind[CreateUserRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err) // RFC 9457、フィールド単位の詳細つき
		return nil
	}

	ctx := sqlbind.WithSQLExecutor(r.Context(), db)
	user, err := store.InsertUser(ctx, input.Name)
	if err != nil {
		return err
	}

	return httpbind.Write(w, r, user)
}
```

これが成立するポイントは 3 つです。

- `c.Request()` と `c.Response()` はただの `*http.Request` と `http.ResponseWriter` なので、
  どのヘルパーもそのまま受け取れます。
- `path:` タグは `r.PathValue` を読みます。これを埋めるのは `net/http` 自身の mux だけなので、
  他のルータではパラメータごとに `SetPathValue` を 1 回呼べば橋渡しできます。
- context から executor を解決するクエリ関数を使うには `-sql-context-api` での生成が必要です
  （Popcorn Wave 自身が使っているのは `-sql-context-only-api` の方です）。

HTML も同じで、ステータスとヘッダの制御はハンドラ側に残ります。

```go
w.Header().Set("Content-Type", "text/html; charset=utf-8")
err := htmlbind.Render(w, pages.Hello(pages.HelloParams{Name: input.Name}))
```

### ミドルウェアも持ち出せる

[`middlewares`](https://github.com/shibukawa/popcornwave/tree/main/middlewares)
の中身はすべて素の `func(http.Handler) http.Handler` で、依存はパッケージグローバルではなく
オプションで渡します。標準ライブラリ互換のスタックならそのまま組み込めますし、Echo なら
ラップできます。

```go
e.Use(echo.WrapMiddleware(middlewares.RequestID()))
e.Use(echo.WrapMiddleware(middlewares.MaxRequestBody(10 << 20)))
```

## 持ち出せないもの

| Popcorn Wave 側に残るもの | 理由 |
| --- | --- |
| 暗黙のドキュメントシェル | `pw.WriteHTML` は登録済みのラッパーチェーンを解決する。`htmlbind.RenderChain` はチェーンを渡す必要がある |
| 階層化された設定と `--generate-config` | `configbind` がやるのは構造体へのバインドまで。ファイルの探索順、環境の選択、スキャフォールドのマージはフレームワーク側 |
| 運用エンドポイント（`server.health`, `server.readiness`, `server.openapi`） | 設定されたパスに `pw.Middlewares` がマウントする |
| プロジェクト全体の OpenAPI マージ | `tinybind-gen` はパッケージごとにフラグメントを出す。決定的にマージするのは `pw generate` |
| `pw dev`、マイグレーション、シード、Tailwind、開発用 IdP | ランタイムではなくツール |

## TinyGo の境界線

境界はここで具体的になります。単体の生成ヘルパーは実行時にリフレクションを使わないため、
TinyGo 対応を維持します。reflect ベースのクエリビルダ、ORM、`html/template`、手書きの
`encoding/json` デコードは、それを import したビルドを TinyGo ターゲットから外します。
通常の Go ビルドなら、どちらも選べます。Popcorn Wave が制約するのは自分が生成するもので
あって、アプリケーションが何を import してよいかではありません。
