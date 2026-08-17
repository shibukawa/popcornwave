---
title: レスポンス
description: HTML、JSON、ストリーム、リダイレクト、RFC 9457 準拠のエラーを返す。
sidebar:
  order: 2
---

ハンドラは HTML、JSON、ストリーム、リダイレクト、エラーのいずれも返せます。そして
HTTP はステータス行とヘッダをボディより先に送るので、どれを返すにせよ、ボディの最初の
1 バイトが出た時点で両方とも確定しています。これはプロトコルの決まりであって、このフレームワークが
足した制約ではありません。レスポンスヘルパーは、それぞれに必要なワイヤ形式を保ちながら、
この線の内側に収めてくれます。

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
ありません。エンコーダは `json` タグの名前とオプションの両方を読みます。読み方は
`encoding/json` ではなく `encoding/json/v2` のほうです。`json:"-"` はそのフィールドを
ドキュメントから丸ごと外します。`omitempty` は空の JSON 値になるメンバ、つまり `""`、
`[]`、`{}` を落とします。数値と真偽値には空という形が無いので、`0` と `false` は線に
乗ったままです。そこへ届くのは `omitzero` のほうで、こちらは Go のゼロ値を持つものを
落とします。ネストした構造体は全フィールドがゼロのときにゼロとみなされます。消したい
のが未設定のカウントやフラグなら `omitzero`、空文字列や空のコレクションなら
`omitempty` を選んでください。どちらでもないオプションは、そこにあって効いていない
ふりをするのではなく生成を失敗させます。`omitempy` と綴り損ねたら、落としたつもりの
フィールドが黙って出力されるかわりにビルドが止まります。

nil のスライスは `[]`、nil のマップは `{}` として届きます。`null` にはなりません。Go は
nil のコレクションと空のコレクションを区別しないので、線の上でも区別しません。「要素が
無い」以外を意味しようのない null チェックを、クライアントは書かずに済みます。
ハンドラに `make([]T, 0)` を追加する必要はありません。空の結果は
それ無しでも空配列になります。

ステータスは 200 です。`pw.WriteAPI` の呼び出し箇所は生成される OpenAPI ドキュメント
にも反映されるため、JSON エンドポイントは別途アノテーションを書かなくても記述されます。
ただし `omitempty` も `omitzero` も、そのスキーマ上でフィールドを optional にはしません。
`required` を決めるのは `check:"required"` です。ときどき省くフィールドに required を
付けないでください。

## ストリーム

トークン、ログ行、キューのイベントのように時間をかけて届くレスポンスは、代わりに
`pw.WriteStream[T]` で書きます。

```go
func events(w http.ResponseWriter, r *http.Request) {
	pw.WriteStream(w, r, func(stream *pw.Stream[ChatEvent]) error {
		for event := range source {
			if err := stream.Write(event); err != nil {
				return err
			}
		}
		return nil
	})
}
```

Server-Sent Events、NDJSON、JSON 配列のどれになるかを選ぶのはクライアントで、上の
ハンドラは 3 つすべてをそのまま処理します。ネゴシエーションの詳細、フレーミング、
長時間のレスポンスが設定に求めるものは[ストリーム](/ja/guides/frontend/streams/)に
あります。

## リダイレクト

リダイレクトもここでは他と同じレスポンスの一種です。ただし書き方が 2 つあります。
ハンドラの終わり方とローダーの終わり方が違うからです。ハンドラは writer を持って
います。

```go
pw.RedirectSeeOther(w, r, "/users/"+id)
```

writer を持たない側——テンプレートが `{val}` で束縛するページローダーや、
`(T, error)` でしか外に出られない関数——は、返します。

```go
if _, ok := auth.User(ctx); !ok {
	return View{}, pw.SeeOther("/auth/login")
}
```

これらのコンストラクタが返すのは `error` です。型で誤魔化しているわけではありません。
リダイレクトは「値を作れなかった」ときの答えの 1 つですし、その手の答えを運ぶ経路は
レスポンス側にすでにあります。`pw.WriteProblem` は渡されたものから意図を読むので、
リダイレクトを渡せば 500 ではなくリダイレクトになります。

```go
if err := service.Load(r.Context(), id); err != nil {
	// err の中身が pw.NotFound なら 404、pw.SeeOther なら 303 になる。
	pw.WriteProblem(w, r, err)
	return
}
```

ステータスを決めるのはコンストラクタで、軸は 2 つあります。

| | メソッドが GET に変わりうる | メソッドを保つ |
| --- | --- | --- |
| 一時的 | `pw.SeeOther` — 303 | `pw.TemporaryRedirect` — 307 |
| 恒久的 | `pw.MovedPermanently` — 301 | `pw.PermanentRedirect` — 308 |

基本は `pw.SeeOther` です。POST のあとならリロードで再送信されるのを防げますし、
ローダーではそもそもどちらの軸も効きません。応答している描画自体が GET なので、303 と
307 の区別がつかないからです。アドレスを廃止するときは `pw.PermanentRedirect` を
選んでください。`pw.MovedPermanently` はメソッドの扱いが実運用で曖昧です。4 つの
どれでもないステータスが要る場合は、`pw.Redirect(w, r, url, status)` が直接受け取ります。

### `http.Redirect` を使わない理由

出ていく途中で 2 つのことが起きます。どちらも、ハンドラごとに書き直したくないものです。

**行き先を検査します。** リダイレクト先はリクエストから読んだ戻り先パスであることが
多く、しかも更新ランタイムはその値を `location.assign` に渡します。`javascript:` の
URL は、そこへ移動するのではなく*実行*されてしまいます。そこでブラウザに渡すのは
相対 URL と `http`、`https`、`mailto`、`tel` だけに限り、それ以外は 500 にして
追いかけません。検査していないパラメータをそのまま流したせいで、アプリケーション自身の
リダイレクトがスクリプト実行に化ける、ということが起きなくなります。

**更新リクエストには指示を返します。** 更新ランタイムが始めたリクエストは `fetch`
です。`fetch` は 303 を自分で追いかけてしまうので、戻ってきた内容が別のページ向けの
領域集合として適用されてしまいます。そこでこの経路では、ナビゲート指示を返します。
ランタイムはそれをナビゲーションとして扱います。2 つの書き方はどちらもこの経路を通る
ので、返したリダイレクトと書いたリダイレクトが食い違うことはありませんし、アクション
ハンドラの側で「いまどちらの種類のリクエストに答えているか」を気にする必要もありません。
[部分更新](/ja/guides/cross-layer/partial-updates/)を参照してください。

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
認識するコンストラクタの呼び出しは、エンドポイントのOpenAPI記述にも現れます。
`pw.TooManyRequests` と `pw.RateLimited` はそこに入らないので、それで応答するルートは
完全な429を返す一方、生成されたOpenAPIにはその429が載りません。

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
