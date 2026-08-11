---
title: コンポーネントスクリプト
description: コンポーネント自身の JavaScript を、インスタンスごとに走る setup と領域が消えるときに走る teardown で書く。一度きり評価されて何も解放しないスクリプトの代わりに。
sidebar:
  order: 6
---

[Authored islands](/ja/guides/interactivity/overview/) の段は、JavaScript は
あなたのものだと言います。これまでそれは、ドキュメントシェルに置いたサイト全体の
モジュールがセレクタで要素を探す、という意味でした。ページ内の script タグは
ドキュメントの寿命のあいだ URL ごとに一度しか評価されないからです。そして部分更新が
見張っていた領域を差し替えても、そのモジュールには知る手立てがありませんでした。

コンポーネントスクリプトは両方を直します。属するマークアップの隣に置け、`setup` は
**描画されたインスタンスごと**に走り、返したものはそのインスタンスが消えるときに
走ります。

```html
package shop

export component Countdown(deadline: string): html {
<script component>
  export function setup(el) {
    const label = el.querySelector("[data-remaining]");
    const timer = setInterval(() => {
      label.textContent = remaining(el.dataset.deadline);
    }, 1000);
    return () => clearInterval(timer);
  }

  function remaining(iso) {
    const seconds = Math.max(0, (Date.parse(iso) - Date.now()) / 1000);
    return Math.floor(seconds) + "秒";
  }
</script>
  <p class="countdown" data-deadline={deadline}>
    <span data-remaining>—</span>
  </p>
}
```

このコンポーネントを3回描画すれば `setup` は3回走り、それぞれ自分の要素を受け取り
ます。そのうち1つを含む領域を差し替えれば、そのインターバルだけが止まり、残り2つは
動き続けます。

## ブロックの位置と、なぜ印が要るのか

宣言の先頭、`head` ブロックの隣、マークアップの前。裸の `component` 属性が必須です。
これがないと、その要素はマークアップの中のただの `<script>` として現在の意味を
保ちます。`<script>{RawJavaScript(code)}</script>` はテンプレートが実際にやっている
ことなので、この区別は要ります。印が新しい読み方を選ぶので、この機能の追加で既存の
テンプレートは1つも変わりませんでした。

ブロックの中身は逐語で読まれます。波括弧は補間ではなく JavaScript です。body に
スクリプトを書けるようにしているのがこれです。

生成器が強制する規則が3つあり、どれも「そうしないと静かに壊れる」から存在します。

**1コンポーネントに1ブロック。** 2つあると、何も宣言していない順序が必要になります。

**ルート要素は1つ。** 宣言を名指しする印がそこに書かれるので、ルートが2つある
コンポーネントには置き場所がありません。

**モジュールであること、import は絶対。** ブロックは public ツリー配下の
内容ハッシュ付きファイルに抽出されるので、相対指定はもうそこに無いディレクトリを
基準に解決されてしまいます。URL で import してください。2つのコンポーネントが
コードを共有する方法もそれです。

## `setup` はインスタンスごと、モジュールは一度きり

この機能全体がここに乗っているので、正確に書きます。頭の中のモデルが取り違えるのが
まさにここだからです。

抽出されたファイルは ES モジュールです。そのトップレベルは**ドキュメントの寿命の
あいだ、URL ごとに一度**走ります。評価済みの URL に対して2つめの
`<script type="module">` を置いても再実行されませんし、タグを外して付け直しても
同じです。だからモジュールスコープは、1インスタンスや1回の訪問に属するものの
置き場所ではありません。

```js
let count = 0;              // 全インスタンスで共有、しかも永久に
export function setup(el) {
	let ownCount = 0;         // このインスタンスのもの
}
```

インスタンスごとに走るのは export された関数です。teardown を第2の export では
なく `setup` の戻り値にしているのもそれが理由で、teardown はたいてい `setup` の
ローカルを必要とします。export を2つにすればモジュールスコープ越しに受け渡す
ことになり、そこはインスタンスより長生きするスコープです。

## 解放は差し替えの前に走る

[部分更新](/ja/guides/cross-layer/partial-updates/)や live の配信が領域を
差し替えるとき、ランタイムはまずその中のコンポーネントスクリプトをすべて解放し、
それから新しいマークアップを適用し、それから届いたものを起動します。

この順序は意図したものです。後に回した teardown は、すでに切り離されたノードを
掴みにいくことになります。`el.querySelector` が `null` を返し、`ResizeObserver` が
もう無いものを unobserve する。

裏を返せば、要素がまだそこにあることを当てにできます。

```js
export function setup(el) {
	const observer = new ResizeObserver(() => reflow(el));
	observer.observe(el);
	return () => observer.disconnect();   // ここでは el はまだドキュメントにある
}
```

差し替えずに領域を動かすだけの操作 — リストの並べ替え — は何も解放しません。
何も破棄されていないからです。インスタンスはノードと一緒に移動します。

## 第2引数、シグナル用

`setup` は要素と一緒にスコープを受け取ります。そこ経由で登録したものは、
インスタンスと一緒に解放されます。

```js
export function setup(el, scope) {
	scope.on("app.finished", (event) => el.classList.add("done"));
}
```

`scope.on` は[`registerEvent`](/ja/guides/cross-layer/signals/)をこのインスタンスに
束ねたものです。`window.popcornwave.registerEvent` を直接呼んでもよく、その場合
解放は自分で覚えることになります。コンポーネントより長生きさせたいハンドラには
妥当な取引で、それ以外には悪い取引です。後始末を忘れると、いまは**破棄された
インスタンスごとに**漏れます。症状はハンドラが2回発火し、やがて20回発火することです。

## コスト

ブロックを宣言したコンポーネントは自分のファイルに抽出され、マージされた head から
普通のモジュールとして参照されます。静的アセットと同じようにキャッシュされ、
参照はインスタンスが何個描画されても一度だけ読み込まれます。

インスタンスあたりのコストは、コンポーネントのルート要素に宣言を名指しする属性が
1つと、`setup` の呼び出しが1回。20行なら静的マークアップの中の定数が20コピーで、
これは圧縮すればほぼ消えます。それと呼び出しが20回。

## 書かないほうがよい場面

ブラウザがすでにやってくれるなら書かない。開閉は `<details>`、モーダルは
`<dialog>`、ツールチップは `popover`。どれもスクリプトなしで動き、あなたのコードが
throw しても動き続けます。[段](/ja/guides/interactivity/overview/)の規則はそのままで、
この機能はそれらの1つ上に乗るもので、置き換えるものではありません。

変わるのが領域の内容なら、それは[部分更新](/ja/guides/cross-layer/partial-updates/)か
[live 境界](/ja/guides/cross-layer/live-rendering/)であって、サーバが送ったばかりの
DOM を書き換えるスクリプトではありません。

そしてページはスクリプトなしでも使えなければなりません。コンポーネントスクリプトは
サーバが描画したマークアップへの上乗せです。スクリプトが無効なら何も再実行されず、
`setup` が走ったことに正しさを依存しているページは、リロードひとつで壊れます。
