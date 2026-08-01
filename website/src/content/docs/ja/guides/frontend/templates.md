---
title: テンプレート
description: 型付き .pw.html コンポーネント。パラメータ、制御構文、スロット、エスケープ、スコープ付きスタイル。
sidebar:
  order: 4
---

通常、テンプレートの失敗はデータが届いて初めて判明します。`.pw.html` コンポーネントは
その失敗を前倒しします。`pw generate` がソースを隣の `_pw_gen.go` にコンパイルし、
アプリケーションを実行する前に値の型と HTML の挿入コンテキストを検査します。

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
| `string`、`decimal` | `string` |
| `bool` | `bool` |
| `int` | `int` |
| `float` | `float64` |
| `bytes` | `[]byte` |
| `datetime`、`date`、`time` | `time.Time` |
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

## 外部関数

表示専用の変換はテンプレートで宣言し、Go で実装します。

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
