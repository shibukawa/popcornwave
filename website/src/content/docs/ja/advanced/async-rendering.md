---
title: 非同期レンダリング
description: 遅いセクションだけを後から届けるページを、async テンプレートパラメータと pw.Go で書く。
sidebar:
  order: 1
---

ページの表示速度は、通常もっとも遅いクエリに引きずられます。ハンドラがすべてを
待ち、テンプレートが一度だけ描画し、最後の依存が応答するまで読者には何も見えません。

非同期レンダリングはこの結びつきを断ちます。準備できた部分は即座にコミットされ、
遅いセクションはそれぞれのデータが確定した時点で自分のプレースホルダを置き換えます。
ひとつの HTTP レスポンスの中で完結し、クライアント側のデータ取得は発生しません。

## 動機

手元にあるプロフィール、900 ms のクエリの先にある注文一覧、1500 ms の呼び出しの先に
ある推薦、という3つを持つページを考えます。

通常どおり描画すると、読者は 1.5 秒間まっさらなタブを眺めたあと、すべてを一度に
受け取ります。非同期に描画すると、シェルとプロフィールが 20 ms で、注文が 0.9 秒で、
推薦が 1.5 秒で届きます。合計は 1.5 秒のままです。2つの依存が順番待ちではなく
重なって走るためですが、**ページが役に立つようになる時点は 75 倍早くなります**。

<figure>
<svg viewBox="0 0 700 210" role="img" aria-label="ひとつのストリーミングレスポンスのタイムライン。シェルと2つの fallback は 20 ミリ秒で届き、注文は 0.9 秒、推薦は 1.5 秒に到着する。両者は順番待ちではなく並行して走っている。">
  <g fill="currentColor" font-size="12" font-family="inherit">
    <text x="0" y="26" opacity="0.75">シェル + fallback</text>
    <text x="0" y="70" opacity="0.75">注文</text>
    <text x="0" y="114" opacity="0.75">推薦</text>
  </g>
  <g fill="currentColor">
    <rect x="150" y="14" width="10" height="16" rx="2"/>
    <rect x="150" y="58" width="272" height="16" rx="2" opacity="0.18"/>
    <rect x="422" y="58" width="10" height="16" rx="2"/>
    <rect x="150" y="102" width="460" height="16" rx="2" opacity="0.18"/>
    <rect x="610" y="102" width="10" height="16" rx="2"/>
  </g>
  <g fill="currentColor" font-size="11" font-family="inherit" opacity="0.75">
    <text x="172" y="27">到着</text>
    <text x="256" y="70">クエリ待ち</text>
    <text x="330" y="114">呼び出し待ち</text>
  </g>
  <line x1="150" y1="8" x2="150" y2="150" stroke="currentColor" stroke-width="1" stroke-dasharray="3 3" opacity="0.5"/>
  <text x="150" y="168" fill="currentColor" font-size="11" font-family="inherit" text-anchor="middle" opacity="0.9">読める</text>
  <line x1="150" y1="140" x2="640" y2="140" stroke="currentColor" stroke-width="1" opacity="0.35"/>
  <g stroke="currentColor" stroke-width="1" opacity="0.35">
    <line x1="150" y1="140" x2="150" y2="146"/>
    <line x1="303" y1="140" x2="303" y2="146"/>
    <line x1="457" y1="140" x2="457" y2="146"/>
    <line x1="610" y1="140" x2="610" y2="146"/>
  </g>
  <g fill="currentColor" font-size="11" font-family="inherit" text-anchor="middle" opacity="0.6">
    <text x="303" y="168">0.5s</text>
    <text x="457" y="168">1.0s</text>
    <text x="610" y="168">1.5s</text>
  </g>
  <text x="150" y="196" fill="currentColor" font-size="11" font-family="inherit" opacity="0.6">2つの依存が重なるため、合計は 2.4 秒ではなく 1.5 秒。</text>
</svg>
</figure>

重要なのは合計時間ではありません。**ステータスコード、ドキュメントの head、確定済みの
値が、遅い処理の完了を待たずにサーバを離れる**ことです。

## ハンドラの変更点

ほとんどありません。これまで完成した値を渡していた場所に、保留中の値を渡すだけです。

```go
func profile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pw.WriteHTML(w, r, Home(HomeParams{
		Profile:        Profile{Name: "Ada Lovelace", Joined: "2026-02-11"},
		Orders:         pw.Go(ctx, loadOrders),
		Recommendation: pw.Go(ctx, recommend),
	}))
}
```

呼ぶべきストリーミング API も、設定するヘッダも、仕込むフラッシュも、書くループも
ありません。`pw.WriteHTML` が「合成されたドキュメントが await 境界を開きうるか」を
問い合わせ、自分で経路を選びます。境界を持たないページは、従来どおりバッファされた
レスポンスと `Content-Length` のままです。

つまり**ストリーミングするかどうかはテンプレートの性質**であって、ハンドラごとに
繰り返す判断ではありません。

## 保留値を作る

`pw.Go` は独立したゴルーチンで処理を開始し、ハンドルを返します。

```go
func loadOrders(ctx context.Context) ([]Order, error) {
	return store.Orders(ctx, customerID)
}

orders := pw.Go(ctx, loadOrders)
```

渡したコンテキストが処理を束縛し、キャンセルの責任は呼び出し側に残ります。
レンダリングが束縛するのは「どれだけ待つか」だけです。

| コンストラクタ | 用途 |
| --- | --- |
| `pw.Go(ctx, work)` | 独立したゴルーチンで今すぐ開始する |
| `pw.Resolved(v)` | すでに手元にある値、およびテスト |
| `pw.Failed(err)` | すでに判明している失敗 |

知っておく価値のある性質が3つあります。

**ハンドルは一度だけ確定し、読み続けられます。** レイアウトとその内側のページが同じ値を
持てます。両方の境界が同じ結果を見て、背後の処理は一度しか走りません。

**チャネルを受け取るコンストラクタはありません。** すでにチャネルを返すサービスは
`pw.Go` のクロージャの中で受信して取り込みます。これによりすべてのハンドルは
フレームワークが開始したゴルーチンに属し、その中の panic はプロセス終了ではなく
そのハンドルのエラーになります。

**早く始めることに意味があります。** 処理は `pw.Go` を呼んだ場所で始まるため、
リクエストの解析、認可、そしてその上にあるすべての描画と重ねられます。

## テンプレートでの宣言

パラメータに `async` を付け、`await` ブロックの中で読みます。

```html
package handlers

type Order {
  id: string
  total: string
}

export component Home(profile: Profile, orders: async Order[]): html {
<h1>{profile.name}</h1>

{await list = orders}
  <ul>{for order in list}<li>{order.id} — {order.total}</li>{/for}</ul>
{fallback}
  <p class="pending">注文を読み込んでいます…</p>
{/await}
}
```

`async T` は任意のパラメータやレコードフィールドに付く前置修飾子で、生成される
params 構造体では `pw.Pending[T]` になります。呼び出し可能な値ではなく、読める場所は
`await` の束縛だけです。

修飾子は型全体にかかります。`async Order[]` は「保留中のスライス」ひとつであって、
「保留値のスライス」ではありません。行ごとに個別に到着させたい場合は、行の型自身に
`async` フィールドを持たせ、ループの中で await します。

レコードは確定済みメンバーと保留中メンバーを同時に持てます。上の例で注文がまだ
飛行中でも `profile.name` をすぐ描画できるのは、この性質のためです。

### 3つの節

```html
{await user = LoadUser(id), posts = LoadPosts(id)}
  ...主サブツリー...
{fallback}
  ...何も判明する前に、最初にコミットされる...
{recover err}
  ...束縛が失敗したとき、代わりに描画される...
{/await}
```

- `await` の後ろの束縛は**同時に開始します**。ひとつのブロックにある2つの遅い呼び出しは
  合計ではなく、遅いほうの時間で済みます。
- `fallback` は**必須**です。最初にレスポンスへコミットされるものであり、遅い依存が
  ページの残りを遅らせないための要です。
- `recover` は**省略可能**で、`code`、`message`、`retryable`、`timeout` を持つ安全な
  エラー値を束縛します。

束縛は主サブツリーでのみ、エラー名は `recover` でのみ可視です。したがってどの節も
「描画時点で存在しない値」を読むことはできません。

`<slot>` は `await` ブロックの中に置けません。fallback と置換の両方が描画してしまう
ためです。これは同時に、ドキュメント・レイアウト・ページの境界が入れ子ではなく
**兄弟**になる理由でもあります。すべて最初のパスで開始し、並行に確定します。

## 失敗したとき

境界が `recover` を宣言しているかどうかが、失敗の代償を決めます。

**`recover` 節がある場合**、失敗はそこで封じ込められます。その節が当該セクションの
代わりに描画され、ページの他の部分は影響を受けません。レスポンスは 200 のままで、
これは正直です。大部分は成功しているのですから。

**ない場合**、そのページは諦められます。テンプレートは「待っている間に何を見せるか」と
「成功したら何を見せるか」を書きましたが、失敗については何も言っていません。
fallback を残せば、ページは永久に「読み込み中」と主張し続けることになります。
そこでフレームワークが、ドキュメントシェルより下をエラーページで置き換えます。

そのページは一度だけ登録します。

```go
pw.RegisterHTMLErrorPage(func(problem pw.Problem) pw.HTMLFragment {
	return Error500(Error500Params{Title: problem.Title})
})
```

受け取るのはマップ済みの problem であって元のエラーではないため、テンプレートが
サーバ側に留めるべき原因を出力することはできません。リゾルバ未登録なら最小限の
組み込みページが使われるので、エスカレーションがアプリ側の設定に依存することはありません。

### エラーはサーバ側に留まる

`recover` サブツリーが生の Go エラーを見ることはありません。既定では失敗は
メッセージなしの `code: "internal"` に、タイムアウトは `code: "timeout"` になります。
より具体的な内容を公開したい場合は、エラー自身に安全な射影を持たせます。

```go
func (e UpstreamError) PublicError() pw.AsyncError {
	return pw.AsyncError{Code: "upstream", Message: "しばらくしてからお試しください。", Retryable: true}
}
```

いずれの場合も、元のエラーは発生した境界とともにログへ届きます。

```
ERROR await boundary failed with no recover clause boundary=tb-1 error="order service returned 503"
```

### 未処理の失敗が 200 のままである理由

ステータスがシェルとともに、失敗が判明するはるか前に送り出されているからです。
これはストリーミングの正直な代償であり、これらのページの監視をステータスコードに
頼る前に知っておく価値があります。

ストリーミングを切れば、同じ失敗が本物の **500** になります。その場合は何もコミット
されていない状態で描画が失敗するので、レスポンスはまだそう言えるのです。2つの経路の
うちステータス行で真実を語れるのは片方だけであり、その片方は実際に語ります。

## 設定

```toml
[html]
streaming = true
async_timeout = "3s"
async_concurrency = 0
```

| キー | 意味 |
| --- | --- |
| `streaming` | `false` にすると、ストリーミング可能なページでもバッファ経路を強制する |
| `async_timeout` | await 境界ひとつを束縛する。`0` ならリクエストのコンテキストが唯一の期限 |
| `async_concurrency` | 1回の描画で同時に走る境界処理の上限。`0` は無制限 |

期限切れの境界は `code: "timeout"` で `recover` を描画し、`recover` がなければ
エスカレーションします。処理そのものが止まるかどうかは関数次第です。
`context.Context` を受け取る関数はキャンセルを見られますが、受け取らない関数は
放棄されます。自力で完走し、その結果は破棄されます。

`streaming = false` は、レスポンスをバッファしてしまうプロキシへの避難口です。
同じテンプレートが、すべての境界が確定するまでブロックする単一のバッファされた
レスポンスとして描画されます。ページは依然として正しく、完全です。

## ブラウザ側で起きること

ドキュメントシェルが、完了した内容を差し込む小さな ES モジュールをひとつ読み込みます。

```html
external RuntimeScriptURL(): url

export component Document(children: html?): html {
<!doctype html>
<html><head>...<script type="module" src={RuntimeScriptURL()}></script></head>
<body><slot /></body></html>
}
```

```go
// templates/templates.go
func RuntimeScriptURL() *url.URL { return &url.URL{Path: pw.RuntimeScriptURL()} }
```

テンプレートがリテラルのパスではなく関数を呼ぶのは、この URL がスクリプト自身の
バイト列から導出したリビジョンを含むためです。ランタイムを変えるアップグレードが
あっても、誰もテンプレートを編集せずに URL が変わり、レスポンスは
`Cache-Control: immutable` を正直に名乗れます。

`pw init` は両方を雛形として生成するので、新規プロジェクトには最初から入っています。

完了内容がインラインスクリプトを運ぶことはないため、`script-src 'self'` で足ります。
nonce も `unsafe-inline` も不要です。

JavaScript を無効にしたクライアントは、シェルとすべての fallback を受け取り、それが
置き換わることはありません。ストリーミングされるセクションは、**それ自体で意味のある
内容に対する上乗せ**として扱ってください。そこへ到達する唯一の手段にはしないことです。

## 制約

- `async` パラメータを読めるのは `await` 束縛の中だけです。
- `await` ブロックには `fallback` 節が必要です。
- `<slot>` を `await` ブロックの中に置くことはできません。
- `@cache` コンポーネントは `async` パラメータを宣言できず、`async` を持つレコードに
  到達することもできません。保存されたバイト列が新しい描画の代わりになるのに対し、
  保留値はそれを開始した唯一のリクエストに属するためです。

いずれも生成時エラーなので、リクエスト時ではなく `pw generate` の時点で判明します。

## 完全な例

リポジトリの `examples/async_render` が、成功・封じ込められた失敗・未処理の失敗を
それぞれ1ページずつリンクしています。どの経路も運任せではなく、狙って到達できます。
