---
title: ランタイム API
description: pw パッケージがランタイムに公開しているものを、役割ごとにまとめた一覧。
sidebar:
  order: 1
---

ランタイムを構成するパッケージは2つありますが、アプリケーションコードが使うのは
片方だけです。`pw` が安定した公開 API であり、このページに並ぶシンボルはすべて
そこにあります。`pwruntime` は生成コードがコンパイル対象とする狭い契約です。
ハンドラがそちらへ手を伸ばしたくなったときは、たいてい `pw` がより短い名前で
再公開しているものを探しています。

もうひとつの区分が一覧の中を走っています。ここにあるものすべてを手で書くわけでは
ありません。設定構造体の登録、ドキュメントシェルの登録、ページツリーの描画は
`pw generate` が出力します。該当するものには **generated** と付けました。自分では
一度も打たなくても、自分のリポジトリを読むときには目に入るからです。

## 起動と停止

| シンボル | 役割 |
| --- | --- |
| `Run(ctx, handler, ...Option) error` | 設定の解析、フレームワークの初期化、待ち受け、グレースフルシャットダウン、資源の解放 |
| `Middlewares(handler, ...Option) (http.Handler, error)` | 同じ初期化を行い、待ち受けずにラップ済みのスタックを返す |
| `WithPublicFS(fs.FS) Option` | 埋め込み済みの public ツリーを `public` ディレクトリ基点で渡す |
| `NewServeMux() *ServeMux` | ルータを生成する |
| `ServeMux` | 通常の Go ビルドでは `net/http` の `ServeMux` そのもの。ラッパではなく型エイリアス |
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

| シンボル | 役割 |
| --- | --- |
| `RegisterConfig[T](prefix)` | 設定構造体を1つ、TOML の prefix に登録する（**generated**） |
| `Config[T](ctx) T` | 解析済みの構造体を返す。リクエスト外では `nil` を渡してよい |
| `Env() string` | 解決済みの実行環境トークン |
| `EnvVar`、`DefaultEnv` | `APP_ENV` と、未設定時に使われるトークン |
| `EnvDevelopment`、`EnvStaging`、`EnvProduction` | よく使うトークン。他の小文字トークンも有効 |
| `RegisterSubCommand[T](name, help)` | CLI 専用の型付き入力を登録する |
| `Command[T]() (T, bool)` | `ParseConfig` 後の、選択・解析済みサブコマンド |
| `ScaffoldTOML()`、`ScaffoldEnv()` | 登録済みの全 prefix から設定のひな形を生成する |
| `WriteScaffoldTOML(w)`、`WriteScaffoldEnv(w)` | 同じひな形を writer へ書き出す |

`Config` は失敗せず、エラーも返しません。登録済みだが未解析の prefix は宣言された
既定値を、未登録の型はゼロ値を返します。設定を読むハンドラはレスポンスの経路上に
いるので、そこで nil チェックを書いても、同じ「値がない」を数行あとへ先送りする
だけです。

`SubCommand` は `RegisterSubCommand` の非推奨エイリアスとして残っています。

フレームワークの設定構造体もすべて公開されています。`ServerConfig`、
`MiddlewareConfig`、`SecurityConfig`、`SessionConfig`、`ObservabilityConfig`、
`HTMLConfig`、`RDBConfig` と、その下にネストする型です。フィールドと既定値は
[アプリケーション設定](/ja/reference/configuration/)に、解説は[設定](/ja/guides/architecture/configuration/)に
あります。

## リクエストを読む

| シンボル | 役割 |
| --- | --- |
| `Parse[T](r) (T, error)` | パス、クエリ、ボディ、ヘッダ、Cookie、メソッドをひとつの構造体へバインドする |
| `RequestAuthentication(ctx) Authentication` | 検証済みの認証結果 |
| `Authenticated(ctx) bool` | 検証済みの identity を持つリクエストかどうか |
| `IsBot(r) bool` | クライアントが境界ランタイムを実行するかどうか |

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

| シンボル | 役割 |
| --- | --- |
| `WriteHTML(w, r, leaf)` | 生成済みフラグメント1つを、登録済みのドキュメントシェルの中に描画する |
| `WriteHTMLPage(w, r, wrappers, leaf, ...HTMLOption)` | ページをレイアウトとドキュメントシェルの中に描画する（**generated**） |
| `WriteHTMLChain(w, r, wrappers, leaf, ...HTMLOption)` | 明示的なラッパチェーンを描画する |
| `WriteHTMLFragment(w, r, fragment)` | テンプレート1つをレスポンス全体として描画する。シェルなし、head のマージなし |
| `WriteAPI[T](w, r, value)` | ネゴシエートした形式で型付きレスポンスを書く |
| `WriteProblem(w, r, err)` | エラーを RFC の problem レスポンスへ写す |
| `NewStream[T](w, r) *Stream[T]` | `Accept` から SSE、NDJSON、JSON 配列のいずれかを選び、ストリーミングを開始する |
| `Stream.Send(value)`、`Stream.Close()` | 値を1つ書く。レスポンスを完結させる |
| `RegisterHTMLDocument(wrapper)` | アプリケーションのドキュメントシェルを差し込む（**generated**） |
| `RegisterHTMLErrorPage(resolve)` | エラーページのリゾルバを差し込む。未登録なら最小限の組み込みページ |
| `RuntimeScriptURL() string` | 境界ランタイムモジュールの絶対パス |

ストリーミングするかどうかをハンドラに尋ねるものは、ここにひとつもありません。
await 境界を開けるチェーンはストリーミングし、開けないチェーンはバッファされて
まとめてコミットされます。その判断は合成されたテンプレートの性質であって、合成した
ハンドラの決定ではありません。だからこそ、非同期レンダリングの導入で変わるのは
ハンドラの引数であって、`Write` の呼び出しではないのです。

`WriteHTMLFragment` は意図的な例外で、常にバッファします。これが答えるのは htmx
系ライブラリによる差し替えで、対象のドキュメントはすでに存在し、ブラウザのパーサは
レスポンスの到着を見ません。ボディはライブラリが受け取って挿入するため、
フレームワークが書いたマーカーでは、確定した境界を元のプレースホルダへ結びつけ
られないのです。head への寄与を持つフラグメントは黙って捨てられるのではなく
エラーになります。ここには受け取る head が存在しません。

## エラー

| シンボル | 役割 |
| --- | --- |
| `Problem` | アプリケーション向けの problem 値。`Status`、`Title`、`Code`、`Message`、`Fields`、`Cause` |
| `BadRequest`、`Unauthorized`、`Forbidden`、`NotFound`、`Conflict`、`PayloadTooLarge`、`InternalServerError`、`ServiceUnavailable` | よく使うステータスのコンストラクタ |
| `Validation(...FieldError) Problem` | 検出したフィールド失敗をすべて載せた 400 |
| `Field(field, location, message) FieldError` | フィールド単位の失敗1件 |
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

| シンボル | 役割 |
| --- | --- |
| `Pending[T]` | ハンドラが描画前に開始し、テンプレートが await する値 |
| `Go[T](ctx, work) Pending[T]` | 専用の goroutine で処理を開始し、ハンドルを返す |
| `Resolved[T](value) Pending[T]` | すでに値で確定しているハンドル |
| `Failed[T](err) Pending[T]` | すでにエラーで確定しているハンドル |
| `HTMLFragment` | パラメータをバインド済みの生成テンプレート |
| `HTMLWrapper` | 生成テンプレートのラッパ |
| `HTMLOption` | 1回の描画を調整する。`HTMLConfig` が渡すオプションへの追加になる |

`async T` と宣言したテンプレートパラメータは、生成される `Params` 構造体では
`Pending[T]` フィールドになります。`Go` に渡したコンテキストは処理の寿命を縛り、
キャンセルの権利は呼び出し側に残ります。描画が縛るのは待ち時間だけです。処理中の
panic はハンドルのエラーになり、プロセスを落とさずに境界の recover 節から表に
出ます。

テストで goroutine を起こさずに済ませたいときに渡すのが `Resolved` です。
[非同期レンダリング](/ja/guides/cross-layer/async-rendering/)を参照してください。

## データベース

| シンボル | 役割 |
| --- | --- |
| `DB(ctx) (*sql.DB, bool)` | 有効な接続グループのプール |
| `DBDriver(ctx) (string, bool)` | そのプールのドライバスキーム |
| `SelectDB(ctx, group) context.Context` | 名前付き接続グループを固定する |
| `SelectWriteDB(ctx) (context.Context, error)` | フレームワークの書き込みが使うグループを固定する |
| `SelectSessionDB(ctx) (context.Context, error)` | セッションテーブルを持つグループを固定する |
| `Transaction(ctx, fn) error` | 生成 SQL がそのトランザクションを使うコンテキストで `fn` を実行する |

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

| シンボル | 役割 |
| --- | --- |
| `Logger(ctx) Log` | リクエスト、その固定属性、有効なスパンに結びついたロガー |
| `Log` | コンテキストに結びついたロガーの型 |
| `WithLogAttributes(ctx, ...Attribute) context.Context` | 返したコンテキストから取るすべてのレコードに属性を足す |
| `String`、`Int`、`Int64`、`Float64`、`Bool`、`Duration`、`Err` | 属性のコンストラクタ |
| `Attribute` | スカラーのキーと値の組。スパン属性と同じ型 |
| `Level`、`LevelTrace`〜`LevelOff` | 重要度。`slog` が名前を持たない trace が debug の1段下にある |

`Logger` が呼び出せないものを返すことはないので、ハンドラ側に nil チェックは要り
ません。子スパンの中で取り直せば、レコードがそのスパンに紐づきます。

```go
ctx, span := pw.StartSpan(ctx, "load-user")
defer span.End()
pw.Logger(ctx).Info("loaded", pw.Int("rows", n))
```

`Fatal` も `Panic` もありません。ログは起きたことを報告するものであり、プロセスを
続けるかどうかを決めるものではないからです。

属性のコンストラクタはスカラーしかありません。レコードのエンコードは決して失敗して
はならず、構造を必要とする値はそれ自身の属性へ分けるべきだからです。

## トレーシング

| シンボル | 役割 |
| --- | --- |
| `StartSpan(ctx, name, ...Attribute) (context.Context, *Span)` | 有効なスパンの子を開く |
| `StartSpanKind(ctx, name, kind, ...Attribute)` | internal でない処理向けの同等物 |
| `Span`、`SpanKind`、`SpanKindInternal`〜`SpanKindConsumer` | スパンの型と種別 |
| `StatusUnset`、`StatusOK`、`StatusError` | スパンのステータスコード |
| `TraceID(ctx) string`、`SpanID(ctx) string` | 現在の識別子。トレース外では空文字列 |
| `Traced(ctx) bool` | 有効なスパンコンテキストを持つかどうか |

トレース外、あるいはトレーシングが無効なとき、`StartSpan` は何も記録せず終了コストも
ないスパンを返します。`defer span.End()` にガードは不要です。リクエストのルートスパンは
フレームワークが作るので、ハンドラが開くのは自分の処理を表すスパンだけです。

`TraceID` はエラーページで利用者に見せる値です。利用者からの報告と、サーバに残った
レコードを突き合わせるのがこの値だからです。

## OpenAPI と API ドキュメント

| シンボル | 役割 |
| --- | --- |
| `SetOpenAPIInfo(OpenAPIInfo) error` | ドキュメントのタイトル、バージョン、説明を設定する |
| `AssembleOpenAPI() ([]byte, error)` | 登録済みの全オペレーションからドキュメントを組み立てる |
| `OpenAPIJSON(w, r)` | そのドキュメントをハンドラとして配信する |
| `ScalarUI(specURL) http.Handler` | `specURL` のドキュメントに対する Scalar のリファレンスページ |
| `SwaggerUI(specURL) http.Handler` | 同じものに対する Swagger UI のページ |

どちらの UI も、アセットは公開 CDN から読み込みます。バイナリには何も埋め込まれ
ません。手でマウントするのは例外的な使い方で、通常は `server.openapi` と
`server.api_doc` がルート登録なしに両方を配信します。
[API ドキュメント](/ja/productivity/api-documentation/)を参照してください。

## 拡張

| シンボル | 役割 |
| --- | --- |
| `RegisterExtension(Extension)` | フレームワークのチェーンに機能を1つ追加する |
| `Extension` | `Name`、`Slot`、`Setup`、`Close` |
| `Slot`、`SlotSession`、`SlotAuthentication`、`SlotGuard` | リクエストチェーン上の位置。小さいほど先に走る |
| `Middleware` | `func(http.Handler) http.Handler` |

インポートされたパッケージが `init` から `RegisterExtension` を呼ぶため、設定と
コードに寄与するのはリンクされた機能だけです。`Setup` はフレームワーク初期化の中で
—— 設定の解析とデータベース起動のあとで —— 一度だけ走り、差し込むミドルウェアを
返します。nil を返せば何も差し込まれません。無効化された拡張は、他のどこにも分岐を
足さずにこうして降りるわけです。

スロットの順序があるのは、ガードが必ず自分より前に確立された状態を見られるように
するためです。セッションが 10、認証が 20、ガードが 30 に並びます。
