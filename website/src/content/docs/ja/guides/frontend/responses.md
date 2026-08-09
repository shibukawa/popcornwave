---
title: レスポンス
description: HTML、JSON、ストリーム、RFC 9457 準拠のエラーを返す。
sidebar:
  order: 2
---

ハンドラは HTML、JSON、ストリーム、エラーのいずれも返せますが、4 つには共通する
制約があります。ボディを書き始める前に、ステータスとヘッダを確定しなければなりません。
レスポンスヘルパーは、それぞれに必要なワイヤ形式を保ちながら、この境界を守ります。

## HTML

```go
pw.WriteHTML(w, r, Home(HomeParams{Name: input.Name}))
```

`Home` と `HomeParams` は `handlers/home.pw.html` から生成されます。ハンドラが渡すのは
**リーフのフラグメント**で、`WriteHTML` がそれをアプリケーションに登録されたドキュメント
シェルの中に描画します。そのためハンドラはドキュメントを指名することも、インポート
することも、構築することもありません。

チェーン全体はバッファに描画され、コミットの**前に**検証されます。そのため
テンプレートが失敗しても、書きかけのページではなく、きれいな 500 にできます。圧縮が
有効でクライアントが受け入れる場合、同じバッファ済みボディが送出時に
エンコードされます。

ラッパーのチェーンを明示的に制御したい場合 —— あるページだけ別のシェルに入れたい場合
—— は `pw.WriteHTMLChain` を使います。

```go
pw.WriteHTMLChain(w, r,
	[]pw.HTMLWrapper{templates.BindPrintDocument(templates.PrintDocumentParams{})},
	Invoice(InvoiceParams{ID: input.ID}),
)
```

### フラグメント

htmx のような操作は、すでに存在するページの一部だけを差し替えます。必要なのは
ドキュメントではなくその領域です。`pw.WriteHTMLFragment` は指定したテンプレートだけを
描画します。

```go
pw.WriteHTMLFragment(w, r, Row(RowParams{Item: item}))
```

ドキュメントシェルも、ラッパーのチェーンも、head の合成も、フレーミングもありません。
ボディはテンプレートが書いたものそのままで、swap ライブラリがそのまま挿入できます。

マークアップを囲むドキュメントがないことから、2 つの帰結が生まれます。

まず、フラグメントはストリーミングしません。await 境界はその場で確定するため、
レスポンスにはクライアントランタイムが置き換えるべきプレースホルダも、挿入先の
ページで未確定な境界と衝突しうる境界 ID も残りません。

次に、ドキュメントの head に寄与するテンプレートは、寄与を黙って捨てるのではなく
500 で拒否します。スコープ付きスタイルが属するのは、すでに読み込まれているページの
head です。そのページが描画するコンポーネントか、共有のスタイルシートで宣言して
ください。

失敗時は HTML のエラーページではなく、`application/problem+json` と本来のステータスで
応答します。エラードキュメントを一領域に swap すると、その領域がページ丸ごとに
置き換わってしまうからです。htmx などのライブラリは 2xx 以外を swap しないため、
ステータスがそのまま判断材料になります。

`examples/htmx_fragment` はこの API だけで組み立てた完全なアプリケーションです。
ドキュメントを返すルートは 1 つだけで、絞り込み・追加フォーム・削除ボタンはそれぞれ
自分が描き直す領域だけを返します。検証に失敗したフォームの扱いも示しています。
`check` の失敗をそのまま problem レスポンスにすると swap ライブラリが無視するためです。

テンプレートの構文、スロット、エスケープ、スコープ付きスタイルは
[テンプレート](/ja/guides/frontend/templates/)で扱います。この API の上に何を組むか
—— サーバーが中身を用意するダイアログ、トースト、swap が最も安い答えでなくなる境目
—— は[フラグメントと島](/ja/guides/interactivity/fragments/)にあります。

### キャッシュポリシー

HTML のレスポンスはどれも、共有キャッシュが保持してよいかどうかを名乗ります。既定の答えは
「だめ」です。

```
Cache-Control: private, no-store
```

この答えはリクエストではなくテンプレートから来ます。そうするしかありません。`Cache-Control`
は本文の最初の1バイトより前にワイヤへ出ますが、4階層下にいる読み手依存のコンポーネントが
描画されるのはずっと後です。描画中に計算した信号はバッファ経路にしか存在できず、ページの
キャッシュポリシーが「たまたまストリーミングが有効だったか」に左右されることになります。

そこで、何かが描画される前に連鎖へ問い合わせます。スコープを何も宣言していない連鎖は private
を名乗ります。ログインを前提にしたアプリケーションが、1行も書かずに得る答えです。共有の答えを
宣言するにはドキュメントシェルにアノテーションを1つ置きます。シェルはその下のすべてを包むから
です。

```html
@cache(scope: "public")
export component Document(children: html?): html { … }
```

共有と宣言されたページには、フレームワークは `Cache-Control` を一切書きません。鮮度はデプロイ
側が決めることで、期間を持たないヘッダは経験則キャッシュを招くか、誰も頼んでいない期間を発明
するかのどちらかになります。だから弱い主張をするのではなく、主張をやめます。期間は CDN か、
自分のミドルウェアで決めてください。

private 側が `no-cache` ではなく `no-store` なのは、ドキュメントが ETag を持たないからです。
守るべき条件付きリクエストが無く、共有マシンのディスクにログイン済みのページを残さないのは
`no-store` のほうです。

ドキュメントでないレスポンスは、それぞれの形が要求するポリシーを持ち続けます。ナビゲーション
差分と live 配信は `no-store`。再描画は `private, no-cache` で、自分が持つ ETag のための条件付き
リクエストを残します。シーケンス —— フラグメントの静的な半分で、読み手ではなくテンプレートに
由来するもの —— は `public, max-age=31536000, immutable` です。

公開サイトの前に CDN を置く前に知っておいてください。シェルが宣言するまで何も共有されないので、
マーケティングページはアノテーションを書くまでエッジを素通りします。これは見落としではなく
意図した向きです。書き忘れて失うのはキャッシュミス1回、防いでいるのは読み手に他人のアカウント
ページを見せることです。

アノテーション自体、特に private なスコープがコンポーネントのキーに何をするかは
[`@cache`](/ja/reference/template-syntax/#cache) にあります。

## JSON

```go
pw.WriteAPI(w, r, user)
```

呼び出し箇所からレスポンス型のエンコーダを生成するため、実行時リフレクションは
ありません。エンコーダは `json` タグの名前部分を使いますが、`omitempty` や除外指定は
解釈しません。したがって、宣言する型を送りたい形に合わせる必要があります。

ステータスは 200 です。`pw.WriteAPI` の呼び出し箇所は生成される OpenAPI ドキュメント
にも反映されるため、JSON エンドポイントは別途アノテーションを書かなくても記述されます。

## ストリーム

トークン、ログ行、キューのイベントのように時間をかけて届くレスポンスは、代わりに
`pw.NewStream[T]` で書きます。

```go
func events(w http.ResponseWriter, r *http.Request) {
	stream := pw.NewStream[ChatEvent](w, r)
	defer stream.Close()

	for event := range source {
		if err := stream.Send(event); err != nil {
			return
		}
	}
}
```

Server-Sent Events、NDJSON、JSON 配列のどれになるかを選ぶのはクライアントで、上の
ハンドラは 3 つすべてをそのまま処理します。ネゴシエーションの詳細、フレーミング、
長時間のレスポンスが設定に求めるものは[ストリーム](/ja/guides/frontend/streams/)に
あります。

## エラー

`pw.WriteProblem` は **RFC 9457 Problem Details** のレスポンスを
`application/problem+json` として書き出します。

```go
pw.WriteProblem(w, r, pw.NotFound("no such user"))
```

```json
{
  "type": "about:blank",
  "title": "Not Found",
  "status": 404,
  "detail": "no such user",
  "code": "not_found"
}
```

### コンストラクタ

| コンストラクタ | ステータス |
| --- | --- |
| `pw.BadRequest` | 400 |
| `pw.Forbidden` | 403 |
| `pw.NotFound` | 404 |
| `pw.TooManyRequests` | 429 |
| `pw.InternalServerError` | 500 |

いずれも`error`、`string`、別の`pw.Problem`、あるいは引数なしを受け付けます。生成器が
対応するコンストラクタの呼び出しは、エンドポイントのOpenAPI記述にも現れます。ただし、
TinyBind v0.5.0は新しい429ヘルパーをまだ推論しないため、実行時レスポンスは完成していても、
生成されたOpenAPIには429が入りません。

割り当て済みのリクエスト量を超えた場合は、`pw.RateLimited`で同じ429 Problemに再試行情報を
付けられます。

```go
pw.WriteProblem(w, r, pw.RateLimited(pw.RateLimit{
	Limit:      100,
	Remaining:  0,
	Reset:      resetAt,
	RetryAfter: 30 * time.Second,
}, "request quota exceeded"))
```

レスポンスには`Retry-After`、`X-RateLimit-Limit`、`X-RateLimit-Remaining`、
`X-RateLimit-Reset`が付きます。`X-RateLimit-*`は互換用の慣用名であり、標準HTTP
フィールドではありません。標準の再試行通知は`Retry-After`です。メタデータを持たない
`pw.TooManyRequests()`を含め、429レスポンスには常に`Cache-Control: no-store`が付きます。

コンストラクタのないステータスは値を直接組み立てます。

```go
pw.WriteProblem(w, r, pw.Problem{
	Status:  http.StatusConflict,
	Title:   "Conflict",
	Code:    "already_registered",
	Message: "that email is already registered",
})
```

### エラーをそのまま流す

`pw.WriteProblem` は任意の `error` を受け取ってマッピングします。

- `pw.Problem`（`%w` でラップされたものを含む）はそのまま使われる
- バインディングやバリデーションのエラーは自身のステータスとフィールド情報を保つ
- それ以外は 500 になる

このマッピングにより、ハンドラは別の変換レイヤーを増やさず、サービスのエラーを
そのまま転送できます。

```go
if err := service.Register(r.Context(), input); err != nil {
	pw.WriteProblem(w, r, err)
	return
}
```

### 2 つの安全動作

**5xx の詳細は漏れません。** ステータスが 500 以上の場合、リクエストスコープのロガーで
完全な内容を記録したうえで、クライアントには `internal error`（code は `internal`）
としてのみ報告します。

**コミット済みのレスポンスは壊しません。** すでにボディの書き込みが始まっている場合、
`WriteProblem` は矛盾する 2 つ目のペイロードを追記せず、エラーをログに記録します。

### HTML のエラーページ

スキャフォールドされたプロジェクトには、`templates/429.pw.html`を含む400から500までの
ステータステンプレートが用意されます。どれも他のページと同じ通常のコンポーネントです。
生成済みのエラーリゾルバは、`Accept`がHTMLを優先すると対応するテンプレートを選び、API
クライアントには同じProblemを`application/problem+json`で返します。どちらの表現でも
ステータスとレスポンスメタデータは変わりません。
