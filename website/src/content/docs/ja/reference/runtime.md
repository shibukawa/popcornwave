---
title: ランタイム API 一覧
description: pw パッケージが公開するランタイム API を役割別にまとめた、関数とメソッドの一覧。
sidebar:
  order: 1
---

ランタイムを構成するパッケージは2つありますが、アプリケーションコードが使うのは
片方だけです。`pw` が安定した公開 API であり、このページに並ぶシンボルはすべて
そこにあります。`pwruntime` は生成コードがコンパイル対象とする狭い契約です。
ハンドラから `pwruntime` を直接使いたくなった場合も、まずは `pw` に同じ機能が
より簡潔な名前で公開されていないか確認してください。

節は役割で分けてあります。節の中の並びは godoc と同じで、まず型、次にその型を
手に入れる呼び出し、続いてその型が持つメソッド、そしてどの型にも属さない関数が
最後です。

もうひとつの区分が一覧の中を走っています。ここにあるものすべてを手で書くわけでは
ありません。設定構造体の登録、ドキュメントシェルの登録、ページツリーの描画は
`pw generate` が出力します。該当するものには **generated** と付けました。自分では
一度も打たなくても、自分のリポジトリを読むときには目に入るからです。

## 起動と停止

**`ServeMux`** —— 通常の Go ビルドでは `net/http` の `ServeMux` そのもの。ラッパでは
なく型エイリアスです。

| | |
| --- | --- |
| `NewServeMux() *ServeMux` | ルータを生成する |

**`Option`** —— `Run` と `Middlewares` がどちらも受け取る設定。

| | |
| --- | --- |
| `WithPublicFS(fs.FS) Option` | 埋め込み済みの public ツリーを `public` ディレクトリ基点で渡す |

**関数**

| 関数 | 役割 |
| --- | --- |
| `Run(ctx, handler, ...Option) error` | 設定の解析、フレームワークの初期化、待ち受け、グレースフルシャットダウン、資源の解放 |
| `Middlewares(handler, ...Option) (http.Handler, error)` | 同じ初期化を行い、待ち受けずにラップ済みのスタックを返す |
| `ParseConfig() error` | 待ち受けずに設定ソースだけを解析する |
| `SetConfigLoadOptions(configbind.LoadOptions)` | `ParseConfig` の前に、設定の読み込み先と方法を調整する |

`Run` はライフサイクル全体を1回の呼び出しにまとめたもので、生成された `main` が
個々の部品に触れないのはそのためです。リスナーをアプリケーション自身が持つ場合
—— サーバレスのアダプタ、テストハーネス、既存の `http.Server` —— には
`Middlewares` を使います。起動サマリが `Run` ではなく `Middlewares` から出るのは、
この時点ではまだ解決済みの設定を握っている一方、誰がどのアドレスにバインドするかを
フレームワークがまだ知らないからです。

`ParseConfig` と `SetConfigLoadOptions` が要るのは、待ち受けるかどうかを決める前に
設定が必要なバイナリです。CLI サブコマンド、マイグレーションの実行、単発のジョブが
それにあたります。

## 設定と実行環境

**関数**

| 関数 | 役割 |
| --- | --- |
| `RegisterConfig[T](prefix)` | 設定構造体を1つ、TOML の prefix に登録する（**generated**） |
| `Config[T](r) T` | このリクエストの解析済み構造体を返す |
| `ConfigContext[T](ctx) T` | ハンドラより下での同等物。リクエスト外では `nil` を渡してよい |
| `Env() string` | 解決済みの実行環境トークン |
| `RegisterSubCommand[T](name, help)` | CLI 専用の型付き入力を登録する |
| `Command[T]() (T, bool)` | `ParseConfig` 後の、選択・解析済みサブコマンド |
| `ScaffoldTOML()`, `ScaffoldEnv()` | 登録済みの全 prefix から設定のひな形を生成する |
| `WriteScaffoldTOML(w)`, `WriteScaffoldEnv(w)` | 同じひな形を writer へ書き出す |

**定数**

| 定数 | 中身 |
| --- | --- |
| `EnvVar`, `DefaultEnv` | `APP_ENV` と、未設定時に使われるトークン |
| `EnvDevelopment`, `EnvStaging`, `EnvProduction` | よく使うトークン。他の小文字トークンも有効 |

`Config` は失敗せず、エラーも返しません。登録済みだが未解析の prefix は宣言された
既定値を、未登録の型はゼロ値を返します。設定を読むハンドラはレスポンスの経路上に
いるので、そこで nil チェックを書いても、同じ「値がない」を数行あとへ先送りする
だけです。

`SubCommand` は `RegisterSubCommand` の非推奨エイリアスとして残っています。

フレームワークの設定構造体もすべて公開されています。`ServerConfig`、
`MiddlewareConfig`、`SecurityConfig`、`SessionConfig`、`ObservabilityConfig`、
`HTMLConfig`、`RDBConfig` と、その下にネストする型です。フィールドと既定値は
[アプリケーション設定一覧](/ja/reference/configuration/)に、解説は
[アプリケーション設定](/ja/guides/architecture/configuration/)にあります。

## リクエストを読む

**`Authentication`** —— 認証ミドルウェアが出した検証済みの結果。ミドルウェアが
無ければゼロ値です。

| | |
| --- | --- |
| `RequestAuthentication(r) Authentication` | 検証済みの認証結果 |
| `Authenticated(r) bool` | 検証済みの identity を持つリクエストかどうか |
| `RequestAuthenticationContext(ctx)`, `AuthenticatedContext(ctx)` | ハンドラより下での同等物 |

**関数**

| 関数 | 役割 |
| --- | --- |
| `Parse[T](r) (T, error)` | パス、クエリ、ボディ、ヘッダ、Cookie、メソッドをひとつの構造体へバインドする |
| `IsBot(r) bool` | クライアントが境界ランタイムを実行するかどうか |
| `Context(r) context.Context` | ハンドラより下の層に渡すための、リクエストのコンテキスト |

`Context` は 2 つの通貨をまたぐ、サポートされた 1 本の橋です。ハンドラが持つのは
リクエストで、生成 SQL やサービス関数、その他リクエストなしで呼べるものが取るのは
`context.Context`。`r.Context()` も同じ値を返しますし、今後も使えます。ただしそれは
`net/http` のリクエスト型のメソッドで、第二のトランスポートが追えない唯一の読み方
でもあります。fasthttp 向けのビルドがきれいに書き換えられるのは `pw.Context` の方です。

認可が消費すべきなのは `RequestAuthentication` であって、Cookie の有無ではありません。
認証ミドルウェアを通っていないリクエストも、匿名のリクエストも、同じ「明示的に
未認証」のゼロ値を返します。おかげでチェックの形はひとつで済みます。

`IsBot` は `User-Agent` ヘッダだけを見て、何も検証しません。それが許されるのは、
これが決めるのが描画ブランチだけだからです。どちらのブランチも同じデータで同じ
チェーンを描画するので、ヘッダを偽装して得られるのは最初の1バイトが遅くなること
だけです。アクセス制御の入力にしてはいけませんし、ページの**内容**を変える判断にも
使ってはいけません。同じ内容の配信方法を変えるのはクローキングではありませんが、
内容を変えればクローキングです。

## レスポンスを書く

ストリーミングするかどうかをハンドラに尋ねるものは、ここにひとつもありません。
await 境界を開けるチェーンはストリーミングし、開けないチェーンはバッファされて
まとめてコミットされます。その判断は合成されたテンプレートの性質であって、合成した
ハンドラの決定ではありません。だからこそ、非同期レンダリングの導入で変わるのは
ハンドラの引数であって、`Write` の呼び出しではないのです。

### HTML

**`HTMLFragment`** —— パラメータをバインド済みの生成テンプレート。作るのは生成
コードで、以下はそれを受け取る側です。

| | |
| --- | --- |
| `WriteHTML(w, r, leaf)` | フラグメント1つを、登録済みのドキュメントシェルの中に描画する |
| `WriteHTMLFragment(w, r, fragment)` | テンプレート1つをレスポンス全体として描画する。シェルなし、head のマージなし |

**`HTMLWrapper`** —— 生成テンプレートのラッパ。レイアウトか、ドキュメントシェルです。

| | |
| --- | --- |
| `WriteHTMLPage(w, r, wrappers, leaf, ...HTMLOption)` | ページをレイアウトとドキュメントシェルの中に描画する（**generated**） |
| `WriteHTMLChain(w, r, wrappers, leaf, ...HTMLOption)` | 明示的なラッパチェーンを描画する |
| `RegisterHTMLDocument(wrapper)` | アプリケーションのドキュメントシェルを差し込む（**generated**） |

**`HTMLOption`** —— 1回の描画を調整する。`HTMLConfig` が渡すオプションへの追加に
なります。

**関数**

| 関数 | 役割 |
| --- | --- |
| `RegisterHTMLErrorPage(resolve)` | エラーページのリゾルバを差し込む。未登録なら最小限の組み込みページ |

`WriteHTMLFragment` は上のストリーミング規則に対する意図的な例外で、常にバッファ
します。これが答えるのは htmx 系ライブラリによる差し替えで、対象のドキュメントは
すでに存在し、ブラウザのパーサはレスポンスの到着を見ません。ボディはライブラリが
受け取って挿入するため、フレームワークが書いたマーカーでは、確定した境界を元の
プレースホルダへ結びつけられないのです。head への寄与を持つフラグメントは黙って
捨てられるのではなくエラーになります。ここには受け取る head が存在しません。

### 型付きレスポンスと problem

| 関数 | 役割 |
| --- | --- |
| `WriteAPI[T](w, r, value)` | ネゴシエートした形式で型付きレスポンスを書く |
| `WriteProblem(w, r, err)` | エラーを RFC の problem レスポンスへ写す |
| `LifecycleHeaders(Lifecycle) (Middleware, error)` | RFC 9745 DeprecationとRFC 8594 Sunsetのミドルウェア |

`WriteProblem` が受け取る problem 値は[エラー](#エラー)にあります。

### ストリーム

**`Stream[T]`** —— コールバックが書き込む、開いたままのレスポンス。

| | |
| --- | --- |
| `WriteStream[T](w, r, fn)` | `Accept` から SSE、NDJSON、JSON 配列のいずれかを選んで開き、`fn` をそれに対して実行する |
| `Stream.Write(value) error` | 値を1つ書いて flush する。`fn` が戻るとランタイムが閉じる |

**関数**

| 関数 | 役割 |
| --- | --- |
| `SetStreamErrorHandler(fn)` | ステータス送信後に起きたストリームまたはソケットの失敗を受け取る |

### WebSocket

**`Socket[In, Out]`** —— コールバックが相手にする、アップグレード済みの接続。

| | |
| --- | --- |
| `WebSocket[In, Out](w, r, fn) error` | リクエストをアップグレードし、`fn` をそれに対して走らせる。返すのはハンドシェイクのエラーだけ |
| `WebSocketWith[In, Out](w, r, opts, fn) error` | 呼び出しごとの `SocketOptions` を取る同等物 |
| `Socket.Read() (In, error)` | メッセージを1つ読み、`In` にデコードする（**generated**）。呼ぶのは1つの goroutine から |
| `Socket.Write(Out) error` | `Out` からエンコードしてメッセージを1つ書く（**generated**）。どの goroutine からでも安全 |
| `Socket.Close() error` | クローズハンドシェイクで終了する。`fn` が戻ったときもランタイムが行う |
| `Socket.Subprotocol() string` | ハンドシェイクが合意したサブプロトコル。無ければ `""` |

**`SocketOptions`** —— ソケット1本が従う上限・デッドライン・オリジンポリシー。

| | |
| --- | --- |
| `SocketDefaults() SocketOptions` | 未設定のフィールドを解決した実効デフォルト |
| `SetSocketDefaults(SocketOptions)` | プロセス全体の上限・デッドライン・オリジンポリシーを設定する |

### アセットの URL

| 関数 | 役割 |
| --- | --- |
| `RuntimeScriptURL() string` | 境界ランタイムモジュールの絶対パス |
| `PublicAssetURL(name) string` | このビルドが静的アセット1つを配信する URL。リビジョンセグメント込み |

## エラー

**`Problem`** —— アプリケーション向けの problem 値。`Status`, `Title`, `Code`,
`Message`, `Fields`, `Cause`, `RateLimit` を持ちます。

| | |
| --- | --- |
| `BadRequest`, `Unauthorized`, `Forbidden`, `NotFound`, `Conflict`, `PayloadTooLarge`, `TooManyRequests`, `InternalServerError`, `ServiceUnavailable` | よく使うステータスのコンストラクタ |
| `RateLimited(RateLimit, ...any) Problem` | `Retry-After`と`X-RateLimit-*`メタデータを持つ429 |
| `Validation(...FieldError) Problem` | 検出したフィールド失敗をすべて載せた 400 |

**`FieldError`** —— `Problem` の中に入る、フィールド単位の失敗1件。

| | |
| --- | --- |
| `Field(field, location, message) FieldError` | フィールド単位の失敗を1件作る |

**型**

| 型 | 中身 |
| --- | --- |
| `RateLimit` | 429 が運ぶ再試行のメタデータ |
| `HTMLErrorPage` | `func(Problem) HTMLFragment`。`RegisterHTMLErrorPage` が受け取る形 |
| `PublicError` | 安全な射影を自前で提供するエラーが実装する |
| `AsyncError` | `recover` 節が描画する、表示して安全な失敗 |
| `UnsetPendingError` | ハンドラが設定しなかった必須の非同期値 |

サーバが知っていることと、ページが見せることの境界は、規律ではなく型で引かれて
います。`HTMLErrorPage` が受け取るのは元のエラーではなく写像済みの `Problem` なので、
テンプレートはサーバが伏せたはずの原因を描画できません。`PublicError` でないエラーは、
メッセージのない内部コードとして recover 節に届きます。元のエラーはサーバ側に留まり、
ロガーへ回ります。

`UnsetPendingError` は1バイトもコミットされる前に発生するため、途中で切れたページ
ではなく通常の problem レスポンスになります。

## 非同期レンダリング

**`Pending[T]`** —— ハンドラが描画前に開始し、テンプレートが await する値。

| | |
| --- | --- |
| `Go[T](ctx, work) Pending[T]` | 専用の goroutine で処理を開始し、ハンドルを返す |
| `Resolved[T](value) Pending[T]` | すでに値で確定しているハンドル |
| `Failed[T](err) Pending[T]` | すでにエラーで確定しているハンドル |

`async T` と宣言したテンプレートパラメータは、生成される `Params` 構造体では
`Pending[T]` フィールドになります。`Go` に渡したコンテキストは処理の寿命を縛り、
キャンセルの権利は呼び出し側に残ります。描画が縛るのは待ち時間だけです。処理中の
panic はハンドルのエラーになり、プロセスを落とさずに境界の recover 節から表に
出ます。

テストで goroutine を起こさずに済ませたいときに渡すのが `Resolved` です。
[非同期レンダリング](/ja/guides/cross-layer/async-rendering/)を参照してください。

## データベース

| 関数 | 役割 |
| --- | --- |
| `DB(r) (*sql.DB, bool)` | 有効な接続グループのプール |
| `DBDriver(r) (string, bool)` | そのプールのドライバスキーム |
| `SelectDB(r, group) context.Context` | 名前付き接続グループを固定する |
| `SelectWriteDB(r) (context.Context, error)` | フレームワークの書き込みが使うグループを固定する |
| `SelectSessionDB(r) (context.Context, error)` | セッションテーブルを持つグループを固定する |
| `Transaction(r, fn) error` | 生成 SQL がそのトランザクションを使うコンテキストで `fn` を実行する |
| `DBContext`・`DBDriverContext`・`SelectDBContext`・`SelectWriteDBContext`・`SelectSessionDBContext`・`TransactionContext` | 上記それぞれのハンドラより下の形。`context.Context` を取り、`SelectDB` で固定済みのものも渡せる |

`Transaction` のネストは、2つめのトランザクションではなくセーブポイントを開きます。
内側の失敗がロールバックするのは内側の作業だけで、外側はそのまま使えます。
トランザクションはコンテキストの有効グループで走ります。単一のステートメントと同じく
`SelectDB` がトランザクション全体のグループを決め、内部の未固定な SQL は既定グループへ
戻らず、そのグループに留まります。

`SelectDB` が知らないグループ名を報告するのは、返されたコンテキストを使う最初の
ステートメントであって、呼び出しの時点ではありません。`SelectWriteDB` はレプリカを
決して選ばないので、書き込みを行う側はデプロイ構成を知らずに済みます。
[リレーショナルデータベース](/ja/guides/storage/rdb/)を参照してください。

## ロギング

**`Log`** —— コンテキストに結びついたロガー。

| | |
| --- | --- |
| `Logger(r) Log` | リクエスト、その固定属性、有効なスパンに結びついたロガー |
| `LoggerContext(ctx) Log` | ハンドラより下、および子スパンの中での同等物 |

**`Attribute`** —— スカラーのキーと値の組。スパン属性と同じ型です。

| | |
| --- | --- |
| `String`, `Int`, `Int64`, `Float64`, `Bool`, `Duration`, `Err` | 属性のコンストラクタ |
| `WithLogAttributes(ctx, ...Attribute) context.Context` | 返したコンテキストから取るすべてのレコードに属性を足す |

**定数**

| 定数 | 中身 |
| --- | --- |
| `Level`, `LevelTrace`〜`LevelOff` | 重要度。`slog` が名前を持たない trace が debug の1段下にある |

`Logger` が呼び出せないものを返すことはないので、ハンドラ側に nil チェックは要り
ません。子スパンの中で取り直せば、レコードがそのスパンに紐づきます。

```go
ctx, span := pw.StartSpan(r, "load-user")
defer span.End()
pw.LoggerContext(ctx).Info("loaded", pw.Int("rows", n))
```

`Fatal` も `Panic` もありません。ログは起きたことを報告するものであり、プロセスを
続けるかどうかを決めるものではないからです。

属性のコンストラクタはスカラーしかありません。レコードのエンコードは決して失敗して
はならず、構造を必要とする値はそれ自身の属性へ分けるべきだからです。

## トレーシング

**`Span`** —— 開始済みのスパン。`End` で閉じます。

| | |
| --- | --- |
| `StartSpan(r, name, ...Attribute) (context.Context, *Span)` | リクエストのスパンの子を開く |
| `StartSpanKind(r, name, kind, ...Attribute)` | internal でない処理向けの同等物 |
| `StartSpanContext(ctx, …)`・`StartSpanKindContext(ctx, …)` | ハンドラより下での同等物。コンテキストが運ぶスパンの下にネストする |

**定数**

| 定数 | 中身 |
| --- | --- |
| `SpanKind`, `SpanKindInternal`〜`SpanKindConsumer` | そのスパンがどんな処理を表すか |
| `StatusUnset`, `StatusOK`, `StatusError` | スパンのステータスコード |

**関数**

| 関数 | 役割 |
| --- | --- |
| `TraceID(r) string`, `SpanID(r) string` | 現在の識別子。トレース外では空文字列 |
| `Traced(r) bool` | 有効なスパンコンテキストを持つリクエストかどうか |
| `TraceIDContext(ctx)`・`SpanIDContext(ctx)`・`TracedContext(ctx)` | ハンドラより下での同等物 |

トレース外、あるいはトレーシングが無効なとき、`StartSpan` は何も記録せず終了コストも
ないスパンを返します。`defer span.End()` にガードは不要です。リクエストのルートスパンは
フレームワークが作るので、ハンドラが開くのは自分の処理を表すスパンだけです。

`TraceID` はエラーページで利用者に見せる値です。利用者からの報告と、サーバに残った
レコードを突き合わせるのがこの値だからです。

## OpenAPI と API ドキュメント

**`OpenAPIInfo`** —— ドキュメントのタイトル、バージョン、説明。

| | |
| --- | --- |
| `SetOpenAPIInfo(OpenAPIInfo) error` | それらをドキュメントに設定する |

**関数**

| 関数 | 役割 |
| --- | --- |
| `AssembleOpenAPI() ([]byte, error)` | 登録済みの全オペレーションからドキュメントを組み立てる |
| `OpenAPIJSON(w, r)` | そのドキュメントをハンドラとして配信する |
| `ScalarUI(specURL) http.Handler` | `specURL` のドキュメントに対する Scalar のリファレンスページ |
| `SwaggerUI(specURL) http.Handler` | 同じものに対する Swagger UI のページ |

どちらの UI も、アセットは公開 CDN から読み込みます。バイナリには何も埋め込まれ
ません。手でマウントするのは例外的な使い方で、通常は `server.openapi` と
`server.api_doc` がルート登録なしに両方を配信します。
[API ドキュメント](/ja/productivity/api-documentation/)を参照してください。

## 拡張

**`Extension`** —— `Name`, `Slot`, `Setup`, `Close` を持ちます。

| | |
| --- | --- |
| `RegisterExtension(Extension)` | フレームワークのチェーンに機能を1つ追加する |

**`Middleware`** —— `func(http.Handler) http.Handler`。

| | |
| --- | --- |
| `RegisterMiddleware(slot, name, Middleware)` | アプリケーションのミドルウェアをそのスロットに1つ足す。チェーンが組まれる前に `main` から呼ぶ |

**型**

| 型 | 中身 |
| --- | --- |
| `Slot`, `SlotSession`, `SlotAuthentication`, `SlotGuard` | リクエストチェーン上の位置。小さいほど先に走る |

インポートされたパッケージが `init` から `RegisterExtension` を呼ぶため、設定と
コードに寄与するのはリンクされた機能だけです。`Setup` はフレームワーク初期化の中で
—— 設定の解析とデータベース起動のあとで —— 一度だけ走り、差し込むミドルウェアを
返します。nil を返せば何も差し込まれません。無効化された拡張は、他のどこにも分岐を
足さずにこうして降りるわけです。

スロットの順序があるのは、ガードが必ず自分より前に確立された状態を見られるように
するためです。セッションが 10、認証が 20、ガードが 30 に並びます。
