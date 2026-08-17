---
title: テンプレート
description: 型付き .pw.html コンポーネント。パラメータ、制御構文、スロット、エスケープ、スコープ付きスタイル。
sidebar:
  order: 4
---

通常、テンプレートの失敗はデータが届いて初めて判明します。`.pw.html` コンポーネントは
その失敗を前倒しします。`pw generate` がソースを隣の `_pw_gen.go` にコンパイルし、
アプリケーションを実行する前に値の型と HTML の挿入コンテキストを検査します。

## コード生成

`.pw.html` は実行時には一度も読まれません。`pw generate` が 1 つずつ隣の
`_pw_gen.go` にコンパイルし、アプリケーションがリンクするのはテンプレートではなく
その Go です。生成されたファイルはビルド出力で、Git は無視し、VS Code は隠し、
生成し直せば作り直されます。編集するのは `.pw.html` のほうです。

走らせ方は 3 つあります。`pw dev` はプロジェクトのソースを監視していて、変わるたびに
生成し直し、リビルドして再起動します。だからテンプレートのエラーは、保存した数秒後に
ビルドの失敗として届きます。`pw build` はコンパイルの前に生成します。
[`pw prepare`](/ja/pw/project/prepare/) はその同じ作業をコンパイラの手前で止めたもので、
TinyGo や自分で書いた `go build` がコンパイルを持つ場合に使います。手で 1 回走らせる
なら `pw generate` です。

走査の対象はモジュール全体ではありません。`popcornwave.toml` が目的ごとに
ディレクトリを挙げていて、`.pw.html` は `templates` の目的に属します。

```toml
[generate]
templates = ["handlers", "templates"]
```

2 つ挙がっているのは、ページテンプレートがそれを描画するハンドラの隣に置かれる一方で、
ドキュメントシェルとエラーページは `templates/` にあるからです。どちらも再帰的に
歩きます。そのどれにも入っていない `.pw.html` は、実行を失敗させるのではなく報告して
飛ばします。サンプルやフィクスチャをコードの隣に置いてもビルドが壊れないのは、
このためです。

```
pw: samples/home.pw.html is outside generate.templates and is not generated from; list its directory to include it
```

目的の一覧は[`pw generate`](/ja/pw/project/generate/)にあります。

## コンポーネント

```html
package handlers

export component Home(name: string): html {
<h1 class="text-3xl font-bold">Hello, {name}</h1>
}
```

ファイルの先頭には所属する Go パッケージを書きます。`export component` が名前、型付き
パラメータリスト、`html` という戻り型を宣言します。生成されるのは次のものです。

```go
type HomeParams struct {
	Name string
}

func Home(params HomeParams) pw.HTMLFragment
```

生成された API によって、ハンドラ側の呼び出しが型検査されます。

```go
pw.WriteHTML(w, r, Home(HomeParams{Name: input.Name}))
```

パラメータ名を変えればフィールド名が変わり、型を変えればフィールドの型が変わるので、
ハンドラ側を合わせるまでコンパイルできません。静かなのはパラメータを追加した場合です。
構造体リテラルはそのまま通り、新しいフィールドは呼び出し側が埋めるまでゼロ値のままになります。
`export` のないコンポーネントは非公開のままで、他のテンプレートからのみ呼べます。

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
| `html` | フラグメント |

`T[]` はスライス、`T?` はオプショナルです。独自の複合型や列挙型も宣言でき、Go の
構造体と定数になります。

```html
type User {
  name: string
  active: bool
  nickname: string?
  profileURL: url
  tags: string[]
}

enum Tone { Primary, Secondary }
```

## 制御構文

```html
{if active}
  <span class="active">active</span>
{else if score >= 80}
  <strong>A</strong>
{else}
  <span class="inactive">inactive</span>
{/if}
```

条件は `bool` でなければなりません。truthy の概念はありません。

```html
{for user, index in users}
  <li data-index={index}>{user.name}</li>
{/for}
```

インデックスは任意です。使わないなら省略できます。

## 属性

通常の属性は式を取ります。

```html
<p class="user {user.active ? 'active' : 'inactive'}">…</p>
```

`string?` が属性値**全体**を供給する場合、nil なら属性そのものが出力されません。
同じ属性の中でオプショナル値と静的テキストを混在させることはできません。

真偽値属性は true のときだけ出力されます。

```html
<article hidden={not user.active}>…</article>
```

URL 属性には `string` ではなく `url` 型が必要です。`string` を渡すと生成エラーになります
—— それが狙いです。

ただし型は半分でしかありません。`url` であっても、ブラウザが「たどる」のではなく「実行する」
スキームを名乗れるからです。`javascript:alert(1)` は HTML エスケープが触る文字を1つも含まない
ので、エスケープしても何も変わらず、そのまま動きます。そこで URL を持つ属性はすべてスキームの
allowlist と照合されます。`http`、`https`、`mailto`、`tel`、そして相対形式——相対 URL は
ドキュメントが既に持つオリジンから出られないので常に通ります。それ以外は `#tb-blocked-url`
として出力されます。

```html
<a href={user.website}>profile</a>
```

| `user.website` | 出力 |
| --- | --- |
| `https://example.com/u` | `href="https://example.com/u"` |
| `/u/42` | `href="/u/42"` |
| `javascript:alert(1)` | `href="#tb-blocked-url"` |
| `data:text/html;base64,…` | `href="#tb-blocked-url"` |
| `data:image/png;base64,…` | `href="data:image/png;base64,…"` |

拒否された URL は削除ではなく置換されます。`href` が無いのは、テンプレートが最初から書かな
かった場合と見分けがつかないからです。誤って拒否された URL が、それを見つける手がかりを何も
残さないことになります。マーカーはフラグメントなので、たどっても現在のドキュメントに戻るだけ
です。

インラインの `data:` URL は画像については通ります。インライン画像は普通のオーサリングだから
です。ただしメディアタイプの厳密なリストに限られます。`image/svg+xml` はそこに入っていません。
SVG ドキュメントはスクリプトを持てるので、画像のメディアタイプを着たスクリプトの流し込み口だ
からです。

対象は `href` と `src` だけではなく、ブラウザが URL として解決する属性すべてです。
`xlink:href`、`data`、`cite`、`background`、`poster`、それに廃止済みのプラグイン系属性も
含みます。`srcset` と `ping` は複数の URL を持つので、1エントリずつ検査され、1つが弾かれても
残りは捨てられません。

別のスキームが必要なアプリケーションは、レンダリングする場所で一度だけ宣言します。

```go
pw.WriteHTML(w, r, page, htmlbind.WithURLSchemes("http", "https", "mailto", "tel", "ftp"))
```

このオプションはリストに追加するのではなく置き換えます。ページが使うスキームをすべて挙げて
ください。インライン画像のリストは `htmlbind.WithDataURLMediaTypes` が同じように扱います。

## 合成とスロット

`children: html` パラメータはタグの間に書かれた内容を受け取ります。

```html
component Badge(label: string, children: html): html {
<span class="badge"><strong>{label}</strong>{children}</span>
}

export component Card(user: User): html {
<Badge label={user.name}>
  <em>member</em>
</Badge>
}
```

名前付きスロットを使うと、既定値付きの挿入位置を複数持てます。

```html
component Panel(title: string, header: html?, children: html, footer: html?): html {
<section class="panel">
  <div class="head"><slot name="header"><b>{title}</b></slot></div>
  <div class="body"><slot required /></div>
  <slot name="footer" />
</section>
}
```

呼び出し側は `template` 要素で埋めます。

```html
export component Page(caption: string): html {
<Panel title={caption}>
  <template name="header"><em>Guide</em></template>
  <p>body text</p>
</Panel>
}
```

スロットはマークアップを合成しますが、通常の値ではありません。スロットパラメータを
式の中で読んだり、存在を判定したり、他へ転送したりすることはできず、`for` ループの
中にスロットを置くこともできません。

この制約は、より広い境界も支えます。**プレゼンテーショナルコンポーネントはデータを
取得しません。** コンポーネントはパラメータが運んできたものを描画し、値のロードは
ハンドラが行います。

## ドキュメントシェル

`templates/document.pw.html` が `doctype`、`html`、`head`、`body` を所有し、その body に
名前なしの `<slot />` を 1 つ持ちます。

```html
package templates

export component Document(children: html?): html {
<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>My App</title></head>
<body><slot /></body></html>
}
```

ページテンプレートはリーフの内容だけを提供し、シェルを繰り返しません。生成された
ドキュメントの成果物はパッケージ初期化時に自身を登録し、`pw generate` がその登録
パッケージを main パッケージにリンクします。ハンドラがそれを参照する必要はありません。
`pw.WriteHTML` は登録されたドキュメントを解決し、ドキュメントを最外層としてチェーンを
描画します。

そのためハンドラコードは、ドキュメントを選択も構築もしません。登録の欠落や重複は、
リクエストを待たずに**起動時**のエラーになります。

:::caution
プロジェクトに `document.pw.html` は **ちょうど 1 つ**です。ツリーのどこかに 2 つ以上
あると `pw generate` は `multiple default documents` で失敗します。別のシェルは名前なし
スロットを持つ通常のエクスポート済みコンポーネントとして書き、`pw.WriteHTMLChain` で
明示的に選択してください。
:::

名前なしスロットを持つエクスポート済みコンポーネントには `Bind<Name>` というラッパー
関数も生成されます。`WriteHTMLChain` が受け取るのはこれです。

```go
pw.WriteHTMLChain(w, r,
	[]pw.HTMLWrapper{templates.BindPrintDocument(templates.PrintDocumentParams{})},
	Invoice(InvoiceParams{ID: id}),
)
```

ラッパーは最外層から順に合成され、それぞれが次のものを自分の名前なしスロットに
埋めます。

## エスケープ

型検査がテンプレートエラーの一部を防ぎ、コンテキスト別エスケープが別の一部を防ぎます。
文字列は挿入先に応じて自動的にエスケープされます。

```html
<p title={message}>{message}</p>
```

エスケープはテキストと属性値に対する答えです。URL に対する答えではありません。そこでの危険は
文字ではなくスキームだからです。何が起きるかは上の [URL 属性](#属性) を参照してください。

信頼済みの内容には明示的な組み込み関数が必要です。

| 組み込み関数 | コンテキスト |
| --- | --- |
| `RawHTML(string)` | HTML の子要素 |
| `RawCSS(string)` | `<style>` の内部 |
| `RawJavaScript(string)` | `<script>` の内部 |
| `JsonForScript(value)` | `<script>` の内部、型付きデータから |

:::danger
`Raw*` はサニタイザではありません。外部由来の任意の入力を渡してはいけません。固定の
内容、または検証済みの信頼できる内容に限定してください。
:::

型付きデータをページに渡すときは `RawJavaScript` ではなく `JsonForScript` を使って
ください。エンコードまで行ってくれます。

## フォームと CSRF

何かを変更するフォームには、リクエストが自分のページから来たことを示すトークンが要ります。
これは書きません。

```html
<form method="post" action="/orders">
  <button>購入</button>
</form>
```

生成時に、トークンを運ぶ hidden フィールドがフォームの第一子として入ります。後続の
フィールドが押しのけられず、作者が覚えておく必要もありません。GET フォームには何も
入りません。フィールドがクエリ文字列になるので、URL に載ったトークンは履歴・ログ・
リファラに残ります。

次の2つは、中途半端に動くフォームを作る代わりに生成時に落ちます。

- 別オリジンに POST するフォーム。セッションの秘密値を第三者に渡すことになります。
- method が計算値のフォーム。safe か unsafe か判別できないので、トークンを外せば GET に
  漏れ、付ければ無防備なフォームが残ります。

ここから制約が1つ出ます。unsafe form を含むコンポーネントは出力キャッシュできません。
保存されたボディが、あるセッションのトークンを次の訪問者に配ってしまうからです。
対処はキャッシュできるものとトークンを持つものを分けることで、制約が正しい構成を
押し出す形になっています。

```html
@cache(ttl: "1m", scope: "public") component ProductList(rows: Product[]): html { … }
component OrderForm(): html { <form method="post">…</form> }
export component Page(rows: Product[]): html { <ProductList rows={rows} /><OrderForm /> }
```

リクエスト外で unsafe form を含むページを描画する場合 — メール本文、ゴールデンテスト — 
トークンを取るセッションがないので、空のフィールドを出す代わりに描画が失敗します。
この失敗には意味があります。空のトークンは送信され、拒否され、原因を指すものを何も
残さないからです。

トークンがテンプレートを出たあとの話は[セキュリティ](/ja/guides/architecture/security/)にあります。

## コンポーネントのスタイル

コンポーネントは静的な head の内容を提供できます。これによりスタイルを対応するマークアップ
の隣に置けます。

```html
export component Card(label: string): html {
<head>
<style>
.box { color: red }
</style>
</head>
<div class="box"><span>{label}</span></div>
}
```

宣言されたクラス名はリネームされ、対応する `class` 属性も書き換えられるため、
スタイルはコンポーネントにスコープされます。未宣言のクラスはそのまま通過し、この区別に
よって Tailwind のユーティリティとスコープ付きルールが共存できます。`:global(...)`
はセレクタをスコープ対象から外します。裸の要素セレクタは生成に失敗するため、クラスで
修飾してください。

同じ宣言の `<script component>` ブロックに、インスタンス固有の JavaScript も
マークアップの隣へ置けます。部分更新や live 更新での差し替えを含む `setup` と teardown の
振る舞いは[コンポーネントスクリプト](/ja/guides/interactivity/component-scripts/)を参照して
ください。

## 外部関数

テンプレートから呼ぶ Go は、テンプレートで宣言してその隣で実装します。

```html
external Decorate(value: string, tone: Tone): string
```

```go
func Decorate(value string, tone Tone) string {
	if tone == TonePrimary {
		return "★ " + value
	}
	return value
}
```

こういう表示用のヘルパは小さい方の用途です。同じ宣言が、コンポーネントが**データを取る**
手段でもあり、覚える価値があるのはそちらです。

```html
external LoadUser(id: string): User

export component UserCard(id: string): html {
{val user = LoadUser(id)}
<article>
  <h2>{user.name}</h2>
  <p>{user.email}</p>
</article>
}
```

`{val …}` が結果に名前を付けます。これが無いと `LoadUser(id)` は書かれた場所ごとに呼ばれる
ので、上の3フィールドは3回のロードになります。束縛ができるまでコンポーネントが正直に
データを取れなかったのは、これが理由です。

束縛に閉じタグはありません。名前は囲みブロックの終わりまで読めて、値が計算されるのは前に
どれだけマークアップがあってもそのブロックの先頭です。最後の性質がローダにレスポンスを
決めさせます。Go 側の関数に末尾の `error` を持たせて `pw.NotFound(…)` を返せば、何も
コミットされないままページが 404 を返します。

呼び出しが、ページを描画してよいかどうか以外に何も答えないこともあります。認可、あるいは
前提条件の確認です。その場合は結果型を宣言せず、`{check …}` と書きます。束縛を引いただけの
同じディレクティブです。

```html
external Authorize(id: string)
external LoadUser(id: string): User

export component UserCard(id: string): html {
{check Authorize(id)}
{val user = LoadUser(id)}
<article>
  <h2>{user.name}</h2>
</article>
}
```

```go
func Authorize(ctx context.Context, id string) error {
	if pw.RequestAuthentication(ctx).Subject != id {
		return pw.Forbidden("not yours")
	}
	return nil
}
```

末尾のエラーがレスポンスを選ぶところはローダと変わらず、そのうえ門番が持っていない値のために
結果型と読み手をでっち上げずに済みます。先頭の `context.Context` はここでも他と同じで任意
です。取ったかどうかは生成が Go のソースを読んで見ます。ただし**保存する** `@cache` の中に
門番を置いてはいけません。キャッシュにヒットすればコンポーネントの中身は丸ごと飛ばされ、
`check` もそこに含まれます。

最初から頭に置いておく価値のある帰結が2つあります。

**コンポーネントの引数はレコードではなく識別子です。** これがキャッシュを可能にします。
[`@cache`](/ja/guides/frontend/rendering-cache/#コンポーネント自身のロードをキャッシュする)
は宣言された引数でキーを決めるので、ここのアノテーション1つがロードと描画をまとめて覆い
ます。ロード済みの `User` を受け取るコンポーネントは、キーの計算にロードが要るので有用な
キャッシュになりません。

**ロード用の external は同期なので描画をブロックします。** 仕事の裏でフォールバックを画面に
出したいなら [`await`](/ja/guides/cross-layer/async-rendering/) を使ってください。そして
この2つは排他です。保存する `@cache` は await するコンポーネントを拒否します。

## 1 パッケージ内の複数ファイル

同じディレクトリの複数のテンプレートファイルは 1 つの生成 Go ファイルにまとまります。
すべて同じパッケージを宣言し、コンポーネント名を重複させてはいけません。

## エラー

生成の失敗はテンプレート上の位置を伴います。

```
profile.pw.html:12:8: html:url requires url, got string
```

よくある原因には、`url` が必要な場所への `string`、`<script>` への `string` の挿入、
静的テキストと混在したオプショナル属性値、`bool` でない条件、未宣言の参照、誤った
コンテキストの組み込み関数、互換性のないスロットマーカー、スコープ付き CSS 内の
裸の要素セレクタがあります。

宣言、演算子、スロットの規則、空白の規則、そして生成が拒むものの全一覧——言語の全体は
[テンプレート構文](/ja/reference/template-syntax/)にあります。
