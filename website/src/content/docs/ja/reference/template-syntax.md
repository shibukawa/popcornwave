---
title: HTML テンプレートフォーマット
description: .pw.html 言語の全体。宣言、型、式、制御構造、スロット、head への寄与、await 境界、そしてテンプレートが拒否される規則。
sidebar:
  order: 2
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
[ビルドツール設定一覧](/ja/reference/build-configuration/)を参照してください。

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

## `val` —— 値に名前を付ける

式は書かれた場所ごとに評価されるので、4か所で呼ばれた external は4回呼ばれます。`val` は
結果に名前を束縛し、以降のブロックはその名前を読みます。

```html
{val record = LoadRecord(id)}
<h1>{record.title}</h1>
<p>{record.summary}</p>
```

閉じタグはありません。名前はディレクティブから**囲んでいるブロックの終わり**まで読めます。
ブロックとは `if` の分岐、`for` の本体、`await` の部分木、あるいは宣言の本体です。マークアップ
の入れ子はブロックではないので、`<div>` の中に書いた束縛はその `<div>` を閉じた後もまだ読めます。

値が計算されるのはディレクティブの位置ではなく、そのブロックの先頭です。前にどれだけマーク
アップがあっても変わりません。ページ——チェインの葉——のトップレベルの束縛はチェインの組み立て
中に走り、ドキュメントのシェルが1バイトも書く前に終わります。ローダーがレスポンスを選べるのは
これがあるからです。Go 関数に末尾の `error` を持たせて `pw.NotFound(…)` を返せば、何も
コミットしないまま 404 を返します。`if`・`for`・`await` の本体の中の束縛は、その本体が走る
ときに走ります。レイアウトの束縛も同じで、ラッパーのパラメータは子のフラグメントがチェインに
据えられるまで揃わないからです。どちらもシェルが線に出た後の失敗なので、描画を終わらせはします
が、ステータスは選べません。

Go の実装が末尾に `error` を返す external は、束縛の値**そのもの**としてしか呼べません。
補間・属性・条件・別の呼び出しの引数——どれも失敗を置く場所を持たないからです。テンプレート側
の宣言はどちらでも同じで、どの関数が失敗しうるかは生成が Go のソースを読んで見ます。描画の経路
は何も包まないので、HTTP の意図を持つエラーは関数が返した値のまま呼び出し側へ届きます。

束縛された名前は、その式が置けた場所ならどこでも使える普通の型付き値です。補間、属性値、
真偽属性、`if` の条件、`for` の反復対象、コンポーネント引数、別の external の引数。右辺は
呼び出しである必要すらなく、フィールドパスでも同じように束縛できます。

1つのディレクティブで複数の名前をカンマ区切りで束縛できますが、**互いを読むことはできません**。

```html
{val user = LoadUser(id), settings = LoadSettings(id)}
```

一方が他方に依存するなら、ディレクティブを2つに分けます。ソースの順序としてもそう読めます。
この規則は `await` と同じで、あちらも束縛が同時に開始されるので互いを読めません。

`val` は不変で、キーワードがそれを述べています。この言語に可変の束縛は無く、`val` は
`val`/`var` の対の不変側です。JavaScript の `let` は再代入できる側なので採っていません。
名前は lowerCamelCase です。

`.pw.html` と `.pw.sql` の両方で使えます。クエリでは値を一度だけ正規化して、1つの文の複数の
パラメータ位置で使うために働き、文そのものには1バイトも寄与しません。

### 生成が拒否するもの

- **どこからも読まれない束縛。** 値は何かが読む前に計算されるので、読まれない束縛は
  external を呼んで結果を捨てます。external はクエリ以外であってはならないので、その呼び出し
  が意図されていたと読める解釈が存在しません。
- **書かれた位置ですでに見えている名前。** パラメータ、外側の束縛、`for` の変数、`await` の
  束縛、`recover` のエラー名、同じブロックの別の束縛——`val` は何ひとつ隠せません。評価が
  ブロックの先頭へ動く以上、外側の名前を読んでいるノードを追い越してしまい、そのノードが
  描画するものが変わってしまうからです。`for` と `await` は今も隠せます。どちらも先頭へは
  動かないからです。
- **`external async` と `external live`。** これらは `await` 節でだけ束縛できます。
- **`html` を返す external。** `html` は値ではなく、呼び出し位置で部分木として描画されます。
- **属性値の中に書かれた束縛。**

## `check` —— 描画を拒む

失敗しうる external を呼べるのは束縛の位置だけです。しかし、ページを描画してよいかどうか以外に
何も答えない呼び出しにとって、束縛は居心地の悪い場所です。`check` は `val` から束縛を引いた
ものです。

```html
external Authorize(user: User)

export component Page(user: User): html {
{check Authorize(user)}
<h1>{user.name}</h1>
}
```

```go
func Authorize(user User) error {
	if !user.MayRead() {
		return pw.Forbidden("not yours")
	}
	return nil
}
```

**結果型を宣言しない** external は、呼び出しが値を生まず、エラーがその答えのすべてだと述べて
います。この関数を呼べるのは `check` だけで、それ以外の位置はすべて関数名を挙げた生成エラーに
なります。結果型を宣言した external も `check` にかけられ、その場合は結果が捨てられます。
あるページでデータのために呼んでいるローダーを、別のページでは宣言を増やさずに門番として
使えるということです。

閉じタグはなく、スコープには何も入らず、出力には1バイトも寄与しません。巻き上げは `val` と
まったく同じで——マークアップの後に書いた `check` はそのマークアップより先に走ります——失敗の
行き先も同じです。描画が終わり、そのブロックは1バイトも書かれず、エラーは包まれないまま
呼び出し側へ届きます。ページのトップレベルに置いた `check` がレスポンスを選べるのはそのため
で、それがこの構文の眼目です。`if`・`for`・`await` の本体の中、あるいはレイアウトに置いた
ものは、走る場所で走り、描画を終わらせることしかできません。

1つのディレクティブにつき呼び出しは1つです。共有する名前が無い以上、門番が2つなら
ディレクティブも2つで、生成もそう言います。式は呼び出しでなければなりません。フィールドパスや
リテラルには検査すべきエラーがないからです。Go 側の先頭の `context.Context` は他の external
と同じように働き、`.pw.html` と `.pw.sql` の両方がこのディレクティブを受け付けます。

async な `check` はありません。結果型を持たない `external async` や `external live` の宣言は
生成に失敗します。境界の失敗が届くのはシェルがコミットされた後で、しかも `recover` 節が
それを飲み込めてしまう——描画を拒むという目的の正反対だからです。

落とし穴は [`@cache`](#cache) です。`check` はキャッシュされる部分木の中に座り、キーは宣言
されたパラメータから導かれます。つまりヒットすれば、そのコンポーネントがするはずだった他の
すべてと一緒に `check` も飛ばされます。読み手に依存する出力を再利用の対象から外すのと同じ
規則です。キーが運んでいないものを門番が読むなら、そのコンポーネントに保存する形の `@cache`
を付けてはいけません。

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
@cache(ttl: "5m", scope: "public")
export component ProductList(rows: Product[]): html { … }
```

引数は2つあり、どちらを書いたかでアノテーションの意味が変わります。`ttl` は保存を頼み、
`scope` は出力が誰のものかを宣言します。どちらも書かないものは生成エラーです。それでは
何も頼んでいないことになります。

### `ttl` — 保存するか、宣言だけするか

`ttl` があると、コンポーネントは描画したバイト列を保存し、その期間だけ再利用します。生成時に
解析されるので、不正な duration や 0 以下の値はビルドを失敗させます。

`ttl` がないと、スコープを宣言するだけで何も保存しません。この形はどこにでも置けます。普通の
コンポーネントにも、レイアウトにも、ドキュメントシェルにも。下に並ぶ制限はすべて保存された
バイト列を守るためにあり、この形にはそのバイト列がないからです。逆にレイアウトやシェルへの
`ttl` は生成エラーになります。起こりえない失効を期間で語ることになるので。

### `scope` — 出力が誰のものか

`scope` は `"private"` か `"public"` を取り、既定は `"private"` です。

private なコンポーネントのキーには、描画した相手の識別子が前置されます。2人の読み手が同じ
エントリに届くことはありません。Popcorn Wave はその値を
`pw.RequestAuthentication(ctx).Subject` から渡します。セッションログインもパスキーもベアラ
トークンも、ハンドラが走る前に1つのローカルアカウント識別子へ収束するからです。匿名リクエスト
はそれを持ちません。識別子のないまま描画された private なコンポーネントは何も保存しません。
空の識別子で保存すれば、private の札を下げた共有エントリになってしまいます。

public なコンポーネントはパラメータだけでキーを作ります。コンポーネントのパッケージとファイル、
生成されたプランの指紋、そして宣言されたすべてのパラメータです。

同じ宣言がレスポンスのキャッシュ可否も決めます。そしてそちらでは、状態が2つではなく3つに
なります。

| 宣言 | キャッシュキー | レスポンスが名乗るもの |
| --- | --- | --- |
| なし | パラメータ | private |
| `scope: "private"` | 読み手の識別子 + パラメータ | private。外側の `public` を拒否する |
| `scope: "public"` | パラメータ | 共有可。連鎖の他が private を宣言していない限り |

宣言のないものは private になります。これはアノテーションの性質ではなく、フレームワークの
既定です。共有扱いにした per-reader なページは、ある読み手のマークアップを別の読み手に渡し
ます。per-reader 扱いにした共有ページが失うのはキャッシュミス1回です。釣り合いません。だから
共有の答えが欲しいプロジェクトが、ドキュメントシェルに1度だけ書きます。

意識して書く価値があるのは真ん中の行です。宣言のないコンポーネントは連鎖の主張を継承します。
そうでなければ何ひとつ public にできません。だから `scope: "private"` は、生成が見られない
ものを言う唯一の手段になります。`ctx` から読み手を読む external な Go 関数を呼ぶコンポーネント
は、どちらの側が書ける検査から見ても共有に見えます。そこでアノテーションが、作者の知識を
呼び出しグラフの運ぶ事実に変えます。

呼び出しグラフが private 宣言に届くコンポーネントへの `@cache(scope: "public")` は、
アノテーションの位置で生成エラーになり、private を宣言したコンポーネントの名前を告げます。
ワイヤに何が出るかは[レスポンス](/ja/guides/frontend/responses/#キャッシュポリシー)にあります。

### 保存する形にだけかかる制限

以下は `ttl` を持つ形にだけ効きます。保存したバイト列から再生できないコンポーネントは、生成が
拒否します。

- `html` パラメータを宣言するもの。スロットの引数は値ではなく束縛された継続だからです。
- `async` パラメータを宣言するもの、または `async` フィールドへ届くレコードを持つもの。
- 直接またはそれが呼ぶコンポーネント経由で `await` 境界へ到達するもの。
- ドキュメントの `head` を持つもの。統合された head はパラメータではなく連鎖に依存します。
- 直接またはそれが呼ぶコンポーネント経由で危険な `<form>` へ到達するもの。
- 出力が provider から来る builtin 要素へ到達するもの。保存された本体は、あるリクエストの値を
  次に尋ねた者へ渡すことになります。

裏側のストアはプロセス内にあり、既定でオンです。`html.cache.enabled` で切り、
`html.cache.max_entries` で上限を決めます。どちらも[アプリケーション設定一覧](/ja/reference/configuration/#html)
にあります。private なキーは1プロセスが持つエントリ数を読み手の数だけ倍にするので、public な
キーを前提に決めた上限は、何かをスコープした時点で見直す価値があります。再描画はページ自身の
オプションで描画して同じストアに届くので、ページ上でキャッシュされたコンポーネントは、それを
置き換えるレスポンスでもキャッシュされたままです。

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

## コンポーネントスクリプト内の `on-<event>`

[スクリプトブロック](/ja/guides/interactivity/component-scripts/)を宣言している
コンポーネントは、そのブロックが返すハンドラを束縛できます。

```html
<button on-click="increment" on-blur="settle">+1</button>
```

生成は各名前をブロックの `setup` の返り値に対して解決し、すべてのペアを属性1つへ
降ろします。

```html
<button data-tb-on="click:increment,blur:settle">+1</button>
```

ランタイムはコンポーネントの mount 時にこれらを束縛するので、ハンドラはその
インスタンス自身の状態をクロージャで掴みます。ブロックが返していない名前は、それを
参照した属性の位置で生成エラーになります。

属性そのものに関する規則が4つあります。

- 予約されるのは**スクリプトブロックを宣言しているコンポーネントの中だけ**です。
  それ以外の場所では、`on-` で始まるハイフン付き属性は読まれずに出力されます。
- 2つめのハイフンはマッチしません。`on-my-event` は普通のカスタム要素属性のままです。
- 値は名前であって呼び出しではありません。`on-click="increment()"` は生成エラーです。
  引数の並びは、何も解決できない式だからです。
- 同じイベントを1要素に2つ書くとエラーです。2つめが失われるので。

`onclick` は手つかずで、インラインの JavaScript のままです。

## DOM に出るコンポーネントパラメータ

`setup` が `props` を分割代入すると、生成はそのパラメータをコンポーネントのルート
要素へ JSON で出力します。

```html
<div data-tb-component="shop.card.Card" data-tb-props="{&#34;label&#34;:&#34;hi&#34;}">
```

出力されるのはブロックが分割代入した名前だけです。だからその分割代入が、その
コンポーネントがブラウザへ何を公開するかの宣言になります。

## ページツリーの `server-action`

[ページツリー](/ja/guides/cross-layer/discovered-routing/)の中では、要素が URL の代わりに
Go のハンドラを名指せます。

```html
<button server-action="Rename" data-target="#name">rename</button>
```

生成はその名前をツリーのハンドラに対して解決します。何へ降ろされるかは要素によって違います。
フォームは自力で送信できて、ボタンはできないからです。

フォーム以外では、ランタイムが読む属性1つ。

```html
<button data-tb-action="/_action/00369cf962b6/Rename" data-target="#name">rename</button>
```

フォームでは、その属性に加えてブラウザが単体で必要とするマークアップ。`action` は書かれない
ので、ネイティブな送信はドキュメント自身の URL — このページのパスパラメータが既に入っている
URL — へ行きます。

```html
<form data-tb-action="/_action/d71506d06c1e/Retire" method="post">
  <input type="hidden" name="_action" value="d71506d06c1e/Retire" />
  <input type="hidden" name="_csrf" value="…" />
```

フォームがあると、生成はそのページ自身のパターンに `POST` も登録し、生成されたディスパッチャ
が hidden のセレクタで分岐します。

どの要素を選ぶか、ハンドラが何を負うかは
[サーバーアクション](/ja/guides/interactivity/server-actions/)にあります。

その要素の他の属性はすべて読まれずに残ります。だからクリックがそこへ POST する以外に何を
するかは、自分のマークアップのままです。どのハンドラにも解決しない名前は、死んだ要素では
なく生成エラーになります。[型付きアクション](/ja/guides/interactivity/server-actions/#スクリプトが呼ぶ関数)
に解決する名前も同じです。返すのはフォームに置き場所のない値なので、エラーがそう言い、
スクリプトから呼ぶときの名前を示します。

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
- 失敗しうる external の、`val` の値そのもの以外での呼び出し。結果型を持たない external の、
  `check` 以外での呼び出し
- すでに見えている名前を使う `val`、1つの `check` に書かれた2つの呼び出し
- `fallback` の無い `await` ブロック
- live 境界の主部分木の中のフォーム部品
- `html` や `async` を宣言する、境界・危険なフォーム・リクエスト毎の builtin 要素・ドキュメント
  の head へ到達するコンポーネントへの、保存する形の `@cache`
- `ttl` も `scope` も持たない `@cache`、レイアウトやシェルへの `ttl`、private を宣言した
  コンポーネントへ到達する `scope: "public"`
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
