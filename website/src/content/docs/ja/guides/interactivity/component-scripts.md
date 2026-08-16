---
title: コンポーネントスクリプト
description: コンポーネント自身の JavaScript を、インスタンスごとに走る setup で書く。ハンドラはマークアップから名前で呼べ、領域が消えれば登録した後始末が走る。
sidebar:
  order: 6
---

[Authored islands](/ja/guides/interactivity/overview/) の段は、JavaScript は
あなたのものだと言います。これまでそれは、ドキュメントシェルに置いたサイト全体の
モジュールがセレクタで要素を探す、という意味でした。ページ内の script タグは
ドキュメントの寿命のあいだ URL ごとに一度しか評価されないからです。そして部分更新が
見張っていた領域を差し替えても、そのモジュールには知る手立てがありませんでした。

コンポーネントスクリプトは両方を直します。属するマークアップの隣に置け、`setup` は
**描画されたインスタンスごと**に走り、そこで登録した後始末はそのインスタンスが
消えるときに走ります。

このブロックは[テンプレートコンポーネント](/ja/guides/frontend/templates/)の一部で、
ライフサイクルは[部分更新](/ja/guides/cross-layer/partial-updates/)による DOM の更新にも
追従します。依存を持つ完全な例は [React の統合](/ja/guides/interactivity/react/)にあり、
同じフックから React ルートを mount・unmount しています。

```html
package shop

export component Countdown(deadline: string): html {
<script component>
  export function setup({ el, teardown }) {
    const label = el.querySelector("[data-remaining]");
    const timer = setInterval(() => {
      label.textContent = remaining(el.dataset.deadline);
    }, 1000);
    teardown(() => clearInterval(timer));
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
let count = 0;                       // 全インスタンスで共有、しかも永久に
export function setup({ el }) {
	let ownCount = 0;                  // このインスタンスのもの
}
```

インスタンスごとに走るのは export された関数で、必要なものは受け取る1つの
オブジェクトに入っています。使うものだけ分割代入して、残りは書かなくて構いません。

```js
export function setup({ el, teardown, onSignal, props }) { }
```

引数の並びではなく1つのオブジェクトにしてあるのは、後から能力が増えたときに
キーが1つ増えるだけで済むからです。第4引数は、誰も渡していない引数になります。

## 解放は差し替えの前に走る

[部分更新](/ja/guides/cross-layer/partial-updates/)や live の配信が領域を
差し替えるとき、ランタイムはまずその中のコンポーネントスクリプトをすべて解放し、
それから新しいマークアップを適用し、それから届いたものを起動します。

この順序は意図したものです。後に回した teardown は、すでに切り離されたノードを
掴みにいくことになります。`el.querySelector` が `null` を返し、`ResizeObserver` が
もう無いものを unobserve する。

裏を返せば、要素がまだそこにあることを当てにできます。

```js
export function setup({ el, teardown }) {
	const observer = new ResizeObserver(() => reflow(el));
	observer.observe(el);
	teardown(() => observer.disconnect());   // ここでは el はまだドキュメントにある
}
```

`teardown` は返すのではなく登録します。だから複数回呼べますし、ヘルパの中からでも
呼べます。走る順は登録の逆で、最後に登録したものが最初です。

差し替えずに領域を動かすだけの操作 — リストの並べ替え — は何も解放しません。
何も破棄されていないからです。インスタンスはノードと一緒に移動します。

## `onSignal`、サーバから来るもの用

`onSignal` 経由で登録したものは、インスタンスと一緒に解放されます。

```js
export function setup({ el, onSignal }) {
	onSignal("app.finished", (event) => el.classList.add("done"));
}
```

このインスタンス用の[シグナル](/ja/guides/cross-layer/signals/)テーブルへハンドラを
登録します。ランタイムはあなたが登録した後始末を走らせる前に、これらの登録を
すべて解放します。コンポーネントのハンドラはこの面に置いてください。破棄済み
インスタンスのコールバックが残り、2回、やがて20回と発火する漏れを防げます。

名前が `on` ではなく `onSignal` なのは、テンプレートの `on-click` が DOM イベントを
束縛するからです。その隣に `on()` があれば同じことをすると読めますが、実際には
別のことをします。

## マークアップから名前で呼べるハンドラ

`setup` が返すのは、このコンポーネントが公開するハンドラの集合です。テンプレートは
それを、発火させる要素の上で名前で指します。

```html
export component Counter(label: string): html {
<script component>
  export function setup({ el }) {
    let count = 0;
    const output = el.querySelector("output");
    return {
      increment() {
        count += 1;
        output.textContent = count;
      },
    };
  }
</script>
  <div>
    <output>0</output>
    <button on-click="increment">{label}</button>
  </div>
}
```

生成時に `increment` がブロックの返り値と突き合わされるので、片方だけ名前を変えれば
ブラウザではなく属性の位置でビルドが落ちます。ハンドラはそのインスタンス自身の状態を
クロージャで掴みます。モジュールレベルの export ではなく返り値から取る理由がこれで、
ループで20行描画すればハンドラも20個、それぞれ自分の状態を見ます。

書くのは名前だけです。`on-click="increment()"` は呼び出しではありません。引数の
並びは式であり、要素ごとに変わるものは DOM から読みます。

```html
<button on-click="remove" data-id={row.ID}>削除</button>
```

```js
remove(event) {
	const id = event.currentTarget.dataset.id;
}
```

読むのは mount 時ではなくイベント発火時です。マークアップが真実の所在なので、
その行を再描画した更新のあと、次のイベントは新しい値を読みます。

`onclick` は手つかずで、インラインの JavaScript のままです。そしてこの
フレームワークの既定の `script-src 'self'` では動かないままでもあります。
ハンドラがこの経路で来るもう1つの理由がそれです。

## サーバーアクションを呼ぶ

更新する理由はジェスチャだけではありません。確認ダイアログの後、ドラッグが落ち着いた
とき、タイマーで — スクリプト自身が決めるとき、そのルート自身の
[サーバーアクション](/ja/guides/interactivity/server-actions/)を名前で呼びます。

```js
export function setup({ el, actions }) {
	return {
		async remove(event) {
			if (!confirm("削除しますか？")) return;
			await actions.delete({ id: event.currentTarget.dataset.id });
		},
	};
}
```

`actions` は、そのページ自身のルートパッケージのサーバーアクションごとに関数を1つ
持ちます。export されたハンドラと、`pw.ServerAction` で宣言したものの両方です。
書く名前は Go の関数名を lowerCamelCase にしたもので、`Delete` は `actions.delete`
になります。宣言が別の名前を公開していればそちらです。URL はどこにも書きません。
アドレスは宣言ディレクトリのダイジェストを含むので、スクリプトに計算できるもの
ではないからです。

引数は JSON で送られ、`pw.Parse` がフォームの POST と同じ入力構造体へ読み込みます。
だから1つのハンドラが両方に応えます。引数なしで呼べば、ボディは一切送られません。

返ってくるものはハンドラが何を書いたかで決まります。リージョンはジェスチャのときと
同一に適用され、それ以外は呼び出し元へ返ります。呼んだのはあなたで、受け取る先が
あるからです。

```js
const created = await actions.draft();   // ハンドラの JSON ボディ
```

やらないことが2つあります。**実行中のマーキングはしません** — 起動された要素が無く、
何を待っているかを知っているのは呼び出したあなたです。そして直接アドレスは
**パスパラメータを運びません**。必要なハンドラは、あなたが送ったものから読みます。

更新に**ゲートをかけたい**ときもこの形です。ハンドラを要素に置き、`server-action`
は置かない。両方を書いたテンプレートはハンドラを走らせてからアクションを必ず発行
します。キャンセルの channel が無いからです。JavaScript で判断すれば、その判断が
目に見えます。

## ブロックが要求するパラメータ

コンポーネントのパラメータを `props` から分割代入すると、生成がそれを要素に出力し、
ランタイムが渡し返します。

```js
export function setup({ props: { deadline } }) { }
```

渡るのは名前を書いたものだけです。つまり分割代入が、このコンポーネントがブラウザへ
何を公開するかの宣言になります。そこに `{price}` と書くことは、価格を DOM に置いて
誰でも読めて書き換えられる状態にすることだと読んでください。返ってきたものは信用
せず、変わっては困るものには署名を。

不在の optional は `null` ではなくキーごと落ちるので、判定は `"deadline" in props`
です。値は JSON の型を保ちます。`dataset` の読み出しではできないことで、数値は
数値のまま届きます。

これは mount 時点のスナップショットで、束縛ではありません。変わりうる値は、
ハンドラが発火時に読む属性に置いてください。

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
