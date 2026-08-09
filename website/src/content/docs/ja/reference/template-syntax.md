---
title: テンプレート構文
description: .pw.html 言語の全体。宣言、型、式、制御構造、スロット、head への寄与、await 境界、そしてテンプレートが拒否される規則。
sidebar:
  order: 3
---

`.pw.html` は `pw generate` が Go にコンパイルする型付きテンプレート言語です。値の型と
HTML の挿入コンテキストがビルド時に検査されるので、壊れたマークアップを吐いたはずの
テンプレートはアプリケーションが動く前に失敗します。

このページは言語の全体です。ページの組み立て方——ドキュメントシェル、描画するハンドラ、
フラグメントを使う場面——は[テンプレート](/ja/guides/frontend/templates/)にあります。

## ファイルの構成

```html
package handlers

type User { name: string, active: bool }

enum Tone { Primary, Secondary }

external Decorate(value: string, tone: Tone): string

component Badge(label: string, children: html): html { … }

export component Card(user: User): html { … }
```

ファイルは、生成コードが属する Go パッケージから始まります。1つのディレクトリの
`.pw.html` はすべて1つの `_pw_gen.go` にコンパイルされるので、パッケージ名は一致して
いなければならず、`type`・`enum`・`external`・`component` の名前を重複させることも
できません。private なコンポーネントも含みます。生成された宣言が同じパッケージを
共有するからです。

生成が読むのは `popcornwave.toml` の `generate.templates` と `generate.pages` が挙げる
ディレクトリだけで、子パッケージへは降りていきません。どのディレクトリにも属さない
`.pw.html` は黙って飛ばされるのではなく報告されます。
[ビルドツール設定](/ja/reference/build-configuration/)を参照してください。

| 宣言 | 導入するもの |
| --- | --- |
| `package name` | 生成ファイルが属する Go パッケージ |
| `type Name { field: T, … }` | レコード。同名の Go 構造体になる |
| `enum Name { A, B }` | 文字列 enum。Go の名前付き型とメンバごとの定数になる |
| `component Name(…): html { … }` | 他のテンプレートからだけ呼べるコンポーネント |
| `export component Name(…): html { … }` | 同じものに加えて Go の関数 |
| `external Name(…): T` | 同じパッケージにある Go 関数。マークアップから呼ぶ |
| `external async Name(…): T` | 同じものを並行に走らせ、await する |
| `external live Name(…): T` | 境界が再描画に使うシーケンスを返す Go 関数 |

レコードのフィールドはカンマか改行で区切ります。`type User { name: string, active: bool }`
と複数行の書き方は同じ宣言です。

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
| `html` | `pw.HTMLFragment` |
| `error` | `pw.AsyncError`。`recover` 句が束縛する値 |
| `trusted_html` | `htmlbind.TrustedHTML` |
| `trusted_css` | `htmlbind.TrustedCSS` |
| `trusted_javascript` | `htmlbind.TrustedJavaScript` |
| `script_json` | `htmlbind.ScriptJSON` |
| 宣言した `type` | 生成された構造体 |
| 宣言した `enum` | 生成された名前付き文字列型 |
| `T[]` | `[]T` |
| `T?` | `*T`。ただし `html?` はフラグメントのまま——フラグメントは自分で不在を表せる |
| `async T` | `pw.Pending[T]` |

`pw` はハンドラが持つものを再公開しています。`HTMLFragment`、`HTMLWrapper`、
`Pending[T]`、`AsyncError` です。trusted 系の4つは
`github.com/shibukawa/tinybind-go/htmlbind` から直接来ますが、パラメータでこれらが要る
ことはたいていありません。テンプレートの中で組み込み関数が作るからです。

`async` はパラメータやレコードのフィールドに付く前置修飾子で、型全体を覆います。
`async Order[]` は pending なスライス1つであって、pending な値のスライスではありません。

## 生成される Go の形

```html
export component Profile(user: User, tone: Tone): html { … }
```

```go
type ProfileParams struct {
	User User
	Tone Tone
}

func Profile(params ProfileParams) pw.HTMLFragment
```

どのコンポーネントも引数はちょうど1つ——宣言順に、パラメータ1つにつき1つの公開
フィールドを持つ `{Name}Params` 構造体です。パラメータが0個でも1個でも複数でも同じ
規則なので、パラメータの無いコンポーネントも空の構造体を取ります。

| 宣言 | 生成されるもの |
| --- | --- |
| `export component Name(…)` | `type NameParams struct{…}` と `func Name(NameParams) pw.HTMLFragment` |
| `export component Name(children: html, …)` | 加えて `func BindName(NameParams) pw.HTMLWrapper` |
| `component name(…)` | 非公開の params 構造体だけで、アプリケーション向けの関数は無い |
| `external Name(…)` | 何も生成されない——Go 関数は自分で書く |

`Bind<Name>` を受け取るのは**無名スロット**を持つコンポーネントだけです。だから葉を
ラッパとして使うことはできず、ラッパの連結は実行時ではなくコンパイル時の検査になります。

`Fragment` は不変で共有しても安全なので、パラメータの無いラッパは起動時に1回作れば
足ります。

## 式

式は `{…}` の中、属性値、コンポーネントの引数に現れます。

| 形 | 例 |
| --- | --- |
| 識別子 | `{name}` |
| メンバアクセス | `{user.profile.name}` |
| インデックス | `{items[0]}` |
| リテラル | `{"text"}`, `{42}`, `{true}`, `{null}` |
| 呼び出し | `{Decorate(value, tone)}` |
| 単項 | `{not active}`, `{!active}`, `{-count}` |
| 条件 | `{active ? 'on' : 'off'}` |

| 演算子 | 被演算子 | 結果 |
| --- | --- | --- |
| `and`, `&&`, `or`, `\|\|` | どちらもオプショナルでない `bool` | `bool` |
| `==`, `!=` | 代入互換で比較可能な2つの値 | `bool` |
| `<`, `<=`, `>`, `>=` | 数値2つ | `bool` |
| `+` | `string` 2つ、または同じ型の数値2つ | その型 |
| `-`, `*`, `/`, `%` | 同じ型の数値2つ | その型 |
| `not`, `!` | `bool` | `bool` |
| 単項の `+`, `-` | 数値 | その型 |

真偽値への暗黙変換も、型の暗黙変換もありません。演算にとって `int` と `float` は別の型
であり、オプショナルな値は `bool` ではなく、`null` はオプショナルとしか比較できません。

## 制御構造

```html
{if score >= 80}
  <strong>A</strong>
{else if score >= 60}
  <strong>B</strong>
{else}
  <strong>C</strong>
{/if}
```

```html
{for user, index in users}
  <li data-index={index}>{user.name}</li>
{/for}
```

`if` の条件は `bool` である必要があります。ループのインデックスは省略できます。ループが
回すのは配列で、マップの反復もレンジ形式もありません。

`{{` … `}}` はテンプレートのどこでも使える波括弧のエスケープです。`{` … `}` を1組だけ
出力し、中身は解析されません。

## 属性

```html
<p title={user.nickname} class="user {user.active ? 'active' : 'inactive'}">…</p>
<article hidden={not user.active}>…</article>
<a href={link.destination}>{link.label}</a>
<input disabled>
```

| 種類 | 規則 |
| --- | --- |
| 通常 | 任意の式を取る。文字列は属性コンテキスト向けにエスケープされる |
| オプショナル | `string?` が値**全体**を与える場合、nil なら属性ごと省かれる |
| 混在 | 1つの属性の中でオプショナルな値と静的テキストを混ぜると生成エラー |
| 真偽値 | 式が真のときだけ出力される。裸の属性は書いたまま出る |
| URL | `href`, `src` などは `url` を要求し、`string` は受け付けない |

一番よく驚かれるのが最後の行で、これは意図したものです。`href` の中の `string` は
`javascript:` のペイロードがページに届く経路であり、`url.URL` はすでに解析を通った値です。

## コンポーネントとスロット

コンポーネントは、宣言そのままの名前の要素として呼びます。子を取らないコンポーネントは
自己終了タグで書けます。

```html
<Badge label={user.name}><em>member</em></Badge>
<Avatar user={user} compact={true} />
```

`children: html` はタグの間にあるものを受け取ります。`<slot>` はそれが入る位置を示し、
`slot` 要素自体は決して出力されません。

```html
component Panel(title: string, header: html?, children: html, footer: html?): html {
<section class="panel">
  <div class="head"><slot name="header"><b>{title}</b></slot></div>
  <div class="body"><slot required /></div>
  <slot name="footer" />
</section>
}
```

| 書き方 | 束縛するもの |
| --- | --- |
| `<slot />` | 予約された `children` パラメータ |
| `<slot name="header" />` | `header` パラメータ。型は `html` か `html?` |
| `<slot>default</slot>` | 同じで、引数が無ければ子要素を描画する |
| `<slot required />` | 必須スロット。`required` は `html`、無い場合は `html?` を要求する |

呼び出し側は、名前付きスロットをその名前を持つ `template` 要素で埋め、無名スロットを
それ以外のすべてで埋めます。

```html
<Panel title={caption}>
  <template name="header"><em>Guide</em></template>
  <p>body text</p>
</Panel>
```

スロットが拒否される規則は次のとおりです。

- 埋め込みブロックの間の空白は、無名スロットの内容になりません。
- `name` の無い `template` 要素はただのマークアップで、書いたまま出力されます。
- スロットは `if` の中に置けます。片方しか走らないので、両方の分岐に現れてもかまいません。
- スロットは `for` の本体、`await` ブロックの中、そして1つの描画経路で2回、いずれも
  **置けません**。
- スロットの引数は値ではありません。式で読むことも、存在を検査することも、転送することも、
  2回挿入することもできません。呼び出し側が渡したかどうかを調べる代わりに、既定の内容を
  宣言してください。
- 既定を持たないオプショナルなスロットが空のとき、何も残りません。要素もラッパもマーカーも。

## head への寄与

`<html>` の**外**で宣言された `head` 要素は、書いた位置ではなくドキュメントの head へ
持ち上げられます。

```html
export component Card(label: string): html {
<head>
<link rel="stylesheet" href="/shared.css" />
<style>
.box { color: red }
</style>
</head>
<div class="box"><span class="label">{label}</span></div>
}
```

| 規則 | 内容 |
| --- | --- |
| 使えるタグ | `link`, `meta`, `style`, `script`, `title`, `noscript` |
| 入れ子 | 要素を持てるのは `noscript` だけで、中身は `link`, `style`, `meta` のみ |
| 内容 | 静的なマークアップだけ。統合された head は body の最初のバイトより前に書かれるので、リクエストのデータに依存できない |
| 本文 | ここの `style` と `script` の本文は生テキストで、波括弧の規則は適用されない |
| 重複排除 | 同一のタグは1回だけ出力される。単位はコンポーネントではなくタグ |
| 範囲 | 描画される連鎖から到達できるコンポーネントはすべて寄与する。body から呼ばれたものも含む |

行き先はドキュメントシェル——`html`・`head`・`body` を持つコンポーネント——です。シェルの
無いレスポンスには統合先がありません。`pw.WriteHTMLFragment` が head への寄与を黙って
捨てずにエラーにするのはそのためです。

### スコープ付きスタイル

コンポーネントの `style` ブロックは、そこで宣言されたクラス名を改名し、同じコンポーネント
内の対応する `class` 属性を書き換えることでスコープされます。

- ブロックが宣言していないクラスはそのまま通ります。Tailwind のユーティリティが
  スコープ付きの規則と共存できるのはそのためです。
- `@keyframes` の名前も改名され、そこへ届く `animation` と `animation-name` の参照も
  一緒に書き換わります。
- `font-family` の名前と CSS カスタムプロパティはグローバルのままです。`@font-face` と
  テーマがコンポーネントをまたいで働き続けます。
- `:global(...)` はセレクタ1つをスコープから外します。
- `p { … }` のような裸の要素セレクタは生成エラーです。改名すべき名前を持たないからです。
  `.card p { … }` のように限定してください。
- 式で与えられたクラスは書き換えられないので、生成エラーになります。

接尾辞はテンプレートのパスとコンポーネント名から導かれるので、無関係な編集で生成される
クラス名は変わりません。

### 切り出されるファイル

インラインの内容を持つ `style` と `script` のブロックは、レスポンスに一度も現れません。
生成がファイルとして書き出し、統合された head には参照タグを置きます。バイト列がキャッシュ
でき、Content Security Policy がインラインスクリプトを禁じられるようにするためです。

```html
<link rel="stylesheet" href="/public/generated/card.style.1f0a3c9d4b21.css">
<script src="/public/generated/card.script.7c62e0b1d938.js" defer></script>
```

1つのテンプレートファイルの style ブロックは1つのスタイルシートにまとまり、コンポーネント
ごとのスクリプトはそれぞれ別のファイルになります。だから `defer`、`async`、`type` など
著者が書いた属性はタグに残ります。名前が内容のハッシュを持つので、変更の無いプロジェクトは
同じ名前を再生成します。すでに外部 URL を指している `script` や `link` はタグをそのまま
寄与し、ファイルを作りません。

## エスケープと信頼済みの内容

文字列は、それが着地する位置——HTML テキスト、属性値、URL——に応じてエスケープされます。
信頼済みの内容を挿入するには、明示的な組み込み関数が要ります。

| 組み込み関数 | 引数 | 結果の型 | 許されるコンテキスト |
| --- | --- | --- | --- |
| `RawHTML(string)` | 文字列 | `trusted_html` | HTML の子要素の位置 |
| `RawCSS(string)` | 文字列 | `trusted_css` | `<style>` の中 |
| `RawJavaScript(string)` | 文字列 | `trusted_javascript` | `<script>` の中 |
| `JsonForScript(value)` | JSON 化できる値 | `script_json` | `<script>` の中 |

`JsonForScript` は直列化できない値を拒否します。`async` フィールドを持つものも含みます。
pending な値は確定するまで表現を持たないからです。

:::danger
`Raw*` はサニタイザではありません。任意の外部入力を渡さないでください。型付きのデータを
ページへ渡すときは `RawJavaScript` ではなく `JsonForScript` を使います。
:::

### `<script>` と `<style>` の中の波括弧

`<script>` と `<style>` の本文は著者が書いた JavaScript や CSS で、そこでは `{` は普通の
構文です。この2つの要素の中で波括弧がテンプレートの挿入を開くのは、内容に密着して書かれ、
なおかつ次のいずれかの形をしているときだけです。

| 形 | 例 |
| --- | --- |
| 裸の値 | `{js}` |
| メンバアクセス | `{cfg.js}` |
| 呼び出し | `{JsonForScript(payload)}` |
| 括弧で囲んだ式 | `{(ready ? on : off)}` |
| 制御ブロック | `{if ready} … {/if}` |

それ以外の波括弧はすべて内容です。両者を分けるのは先頭の空白なので、`{ name }` は内容で
`{name}` は挿入になります。オブジェクトリテラル、圧縮された関数、入れ子の at-rule、
`${name}` のテンプレートリテラルがバイト単位で生き残るのはこのおかげです。この形で表せない
ものは括弧で囲みます。`{items[0]}` ではなく `{(items[0])}` です。

密着して書かれるために形に一致してしまう書き方が2つあり、どちらも黙って置換されるのでは
なく捕まります。

```js
const o = {name};      // unknown identifier name
if(x){render()}        // unknown function render
```

逃げ道は `{{name}}` で、`const o = {name};` を出力します。1つだけ静かに通る場合があります。
挿入可能な型のパラメータと名前が一致する、密着した省略記法です。著者のコードがはるかに
よく書く空白付きの形が内容として扱われるのは、そのためです。

`<html>` の外で宣言された `<head>` は head への寄与で、その本文はそのまま出力されるので、
ここまでの規則は一切適用されません。

## 空白

静的なマークアップの中の空白の連なりは、生成時にすべて空白1つへ潰されます。ブラウザが
そう描画するのと同じ結果です。消さずに1つ残すのは、2つのインラインボックスの間の空白は
見えるからです。

次のものは書いたままのバイト列を保ちます。

- `<pre>` と `<textarea>`。入れ子になっているものすべてを含む
- `<script>` と `<style>` の本文
- `preserve-whitespace` を付けた部分木

```html
<div id="log" preserve-whitespace>
  first line
  second line
</div>
```

`preserve-whitespace` は予約された裸の属性で、出力には現れません。
`preserve-whitespace="false"` は黙って無視されるのではなく生成エラーです。

空白だけの連なりが丸ごと取り除かれるのは、HTML パーサ自身が捨てる場所だけです。`<html>`・
`<head>`・table 系要素の直下と、ドキュメント全体を描画するコンポーネントの doctype の
周りです。

## 外部関数

```html
external Decorate(value: string, tone: Tone): string
```

```go
func Decorate(value string, tone Tone) string { … }
```

対応するシグネチャの Go 関数を同じパッケージに実装します。先頭に `context.Context`
パラメータを置くかどうかは Go を書く側の判断で、テンプレートの宣言はどちらでも変わりません。
生成がパッケージを読んで、どの関数がそれを取るかを見ます。

```go
func RequestID(ctx context.Context) string { … }
```

ページではなくリクエストに属する値——リクエスト ID、nonce、ロケール——が、それを必要とする
すべてのページのパラメータ構造体を通らずにマークアップへ届くのはこの経路です。この呼び方を
する関数はレスポンスに書き込んではいけません。

`: html` と宣言された external はフラグメントを返し、部分木として描画されます。

## async と await

`external async` の関数は、ページの描画中に並行して走ります。Go の実装は普通のブロッキング
関数のままで、エラーの戻り値が増えるだけです。

```html
external async LoadUser(id: string): User
```

```go
func LoadUser(id string) (User, error)
func LoadPosts(ctx context.Context, id string) ([]Post, error) // context は上と同じく任意
```

async の結果は、それを待つ境界の中にしか存在しません。だから `await` の束縛以外の場所で
呼ぶと生成エラーになります。

```html
{await user = LoadUser(id), posts = LoadPosts(id)}
  <h1>{user.name}</h1>
  <ul>{for post in posts}<li>{post.title}</li>{/for}</ul>
{fallback}
  <p class="pending">loading…</p>
{recover err}
  <p class="failed">{err.message}</p>
{/await}
```

| 句 | 必須 | 有効範囲 |
| --- | --- | --- |
| `{await a = f(), b = g()}` | はい | 束縛が見えるのは主部分木の中だけ |
| `{fallback}` | **はい** | 束縛は見えない |
| `{recover name}` | いいえ | `name` は `error`。`code`, `message`, `retryable`, `timeout` を持つ |

`await` の後ろの束縛は同時に始まるので、1つの句にある遅い呼び出し2つは、合計ではなく
遅いほうの時間で済みます。`fallback` が必須なのは、それが最初にレスポンスへ確定するもの
だからです。

`recover` を持たないブロックが失敗すると、そこに何も描かれないのではなく、ページ全体の
失敗経路に乗ります。

### 呼び出し側が始める値

`external async` の呼び出しは境界が到達した時点で始まります。代わりにパラメータを `async`
と宣言すれば、仕事は好きな場所で始められ、テンプレートには待つことだけが残ります。

```html
export component Profile(customer: Customer, headline: async string?): html {
<h1>{customer.name}</h1>
{await orders = customer.orders}
  <ul>{for order in orders}<li>{order.id}</li>{/for}</ul>
{fallback}
  <p>loading {customer.name}…</p>
{/await}
}
```

```go
customer := Customer{
	Name:   "ada",
	Orders: pw.Go(ctx, func(ctx context.Context) ([]Order, error) { return store.Orders(ctx, id) }),
}
```

ハンドルを作るのは `pw.Go`、`pw.Resolved`、`pw.Failed` の3つです。`async` パラメータは
呼び出せず、読める場所は `await` の束縛だけで、そこでは同じ句の async 呼び出しと並びます。
レコードは確定済みのメンバと pending なメンバを同時に持てます。上の例の `fallback` が
`customer.name` を描けて、境界の向こうの注文だけが待ちに残るのはそのためです。

オプショナルな型の未設定ハンドルは、即座に「不在」として確定します。必須の型の未設定
ハンドルは呼び出し側のバグで、`pw.UnsetPendingError` として表面化します。
[非同期レンダリング](/ja/guides/cross-layer/async-rendering/)を参照してください。

### live なソース

```html
external live WatchMetrics(id: string): Point
```

```go
func WatchMetrics(ctx context.Context, id string) iter.Seq2[Point, error]
```

live なソースは普通の `await` ブロックで束縛します。2つ目の句のキーワードはありません。
先頭の `context.Context` はここでは任意ではなく**必須**です。終わりの無いソースには、
それを返らせる何かが要るからです。

各値は差分ではなく領域の状態そのものを運び、値ごとに主部分木が描き直されます。1つの句が
1回で確定する呼び出しと live なソースを同時に束縛することもできます。

live な領域が描くのは出力であって入力ではありません。主部分木の中の `<form>`、`<input>`、
`<textarea>`、`<select>` は生成エラーです。読み手が入力している最中に配信が届けば、打った
ものを捨ててしまうからです。この規則は境界に従うので、live な束縛が1つあればブロック全体に
適用されます。`fallback` と `recover` は対象外です。配信がそれらを置き換えることはないから
です。[ライブレンダリング](/ja/guides/cross-layer/live-rendering/)を参照してください。

## フォームと CSRF

メソッドが `post`・`put`・`patch`・`delete` のフォームは、CSRF の hidden フィールドを
フォームの最初の子として生成して持ちます。書くことは何もありません。

| 場合 | 起きること |
| --- | --- |
| `method="get"`、またはメソッド無し | トークンは付かない。GET フォームのフィールドはクエリ文字列になり、URL のトークンは履歴・ログ・リファラに届く |
| `action` が静的な絶対 URL | **生成エラー**——トークンを入れると、セッションの秘密を別のオリジンへ渡すことになる |
| `method` が式 | **生成エラー**——生成時に安全か危険かを判定できない |
| 同名のフィールドがすでにある | そのまま。手書きのトークンが引き続き動く |

危険なフォームに到達するコンポーネントは `@cache` できず、この規則は呼び出しグラフを
たどります。リクエストの外——メール本文、ゴールデンテスト——で描画すると、空のフィールドを
出す代わりに失敗します。

## `@cache`

```html
@cache(ttl: "5m")
export component Sidebar(userId: string, tone: Tone): html { … }
```

`ttl` 引数は必須で、生成時に解析されるので、不正な duration はビルドを失敗させます。キーが
覆うのはコンポーネントのパッケージとファイル、生成されたプランの指紋、そして宣言された
すべてのパラメータです。それ以外は覆いません。だからリクエストの同一性、認可、ロケールは
パラメータとして届けるか、そのコンポーネントをキャッシュしないかのどちらかになります。

保存したバイト列から再生できないコンポーネントは、生成が拒否します。

- `html` パラメータを宣言するもの。スロットの引数は値ではなく束縛された継続だからです。
- `async` パラメータを宣言するもの、または `async` フィールドへ届くレコードを持つもの。
- 直接またはそれが呼ぶコンポーネント経由で `await` 境界へ到達するもの。
- ドキュメントの `head` を持つもの。統合された head はパラメータではなく連鎖に依存します。
- 直接またはそれが呼ぶコンポーネント経由で危険な `<form>` へ到達するもの。

裏側のストアはプロセス内にあり、既定でオンです。`html.cache.enabled` で切り、
`html.cache.max_entries` で上限を決めます。どちらも[設定](/ja/reference/configuration/#html)
にあります。再描画はコンポーネントを別の入口から描画するためストアを参照しません。
つまり単体で再描画された `@cache` コンポーネントは毎回本体を実行します。

## ハイフン付きの要素

ハイフンは HTML 自身のカスタム要素の目印なので、ハイフン付き要素の空間は宣言された
ホワイトリストです。Popcorn Wave は今のところそこに何も宣言していないので、
**`.pw.html` の中のハイフン付き要素はすべて生成エラー**になります。

```
probe.pw.html:4:6: undeclared element <my-widget> no hyphenated element is
declared, so every one is undeclared; a framework registers a builtin entry and
an application registers a passthrough entry for each Web Component it uses
```

`<svg>` と `<math>` の中のハイフン付きの名前は標準の外部名前空間の要素で、そもそも
ホワイトリストの外です。

ストリーミングされたページが運ぶ `<tb-boundary>` と `<tb-apply>` はランタイムが書くもので
あってテンプレートが書くものではないので、この規則の影響を受けません。

## ページツリーの `server-action`

[ページツリー](/ja/guides/cross-layer/discovered-routing/)の中では、要素が URL の代わりに
Go のハンドラを名指せます。

```html
<button server-action="Rename" data-target="#name">rename</button>
```

生成はその名前をツリーのハンドラに対して解決し、属性1つへ降ろします。

```html
<button data-pw-action="/_action/00369cf962b6/Rename" data-target="#name">rename</button>
```

その要素の他の属性はすべて読まれずに残ります。だからクリックがそこへ POST する以外に何を
するかは、自分のマークアップのままです。どのハンドラにも解決しない名前は、死んだ要素では
なく生成エラーになります。

## よくある生成エラー

- `href`・`src` などの URL 属性への `string`
- `<script>` や `<style>` への普通の `string` の挿入
- 1つの属性の中でのオプショナルな値と静的テキストの混在
- `if` の `bool` でない条件、`and`・`or`・`not` への `bool` でない被演算子
- 異なる数値型どうしの演算
- 未宣言の識別子・フィールド・関数・コンポーネント
- 誤ったコンテキストで使われた組み込み関数
- `required` マーカーとパラメータの型が食い違うスロット、対象が宣言していないスロット、
  `for` や `await` の中のスロット
- スコープ付きスタイルブロックの裸の要素セレクタ
- `await` の束縛の外での `external async` の呼び出し、それ以外の場所での `async` 値の読み取り
- `fallback` の無い `await` ブロック
- live 境界の主部分木の中のフォーム部品
- `html` や `async` を宣言する、境界・危険なフォーム・ドキュメントの head へ到達する
  コンポーネントへの `@cache`
- ハイフン付きの要素すべて

診断はテンプレート上の位置を持ちます。`<script>` や `<style>` の中で出たものは、要素名と
逃げ道を併せて示します。

```
tasks.pw.html:13:65: unknown identifier name; this is inside <script> content,
where {...} is a template insertion. Write {{...}} to keep a literal brace,
insert a value with RawJavaScript or JsonForScript, or move the script to a file
under the public asset directory
```

テンプレートを変えたら毎回 `pw generate` を走らせてください。走らせるまで、Go のビルドは
前のプランを見ています。テンプレートではもう直した診断も含めて。
