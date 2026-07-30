---
title: ブラウザ標準の部品
description: JavaScript を必要としないダイアログ・ポップオーバー・開閉ウィジェットと、.pw.html コンポーネントに置いたときだけ現れる 4 つの規則。
sidebar:
  order: 6
---

モーダル、メニュー、ツールチップ、アコーディオンは、10 年のあいだライブラリの
領分でした。それらはいま要素と属性になっています —— つまりマークアップであり、
マークアップはコンポーネントが吐き出すものです。サーバー描画のテンプレートは、
それを書く場所として結局のところ申し分ありません。

主役は 3 つです。`<dialog>`、popover 系の属性、そして `<details>`。ただし
`.pw.html` コンポーネントの中に書くと、素の HTML ファイルにはない 4 つの規則が
効いてきます。

## ダイアログ

`<dialog>` はトップレイヤー、バックドロップ、`Esc` での閉じ、そして
`showModal()` を使えばダイアログ内へのフォーカス移動と、それ以外のドキュメントの
不活性化まで面倒を見ます。どれも自分で実装するものではありません。

```html
export component DeleteButton(id: string, title: string): html {
<button type="button" command="show-modal" commandfor="confirm-delete">削除</button>

<dialog id="confirm-delete" class="confirm">
  <form method="dialog">
    <p>「{title}」を削除しますか？</p>
    <button value="cancel">キャンセル</button>
  </form>
  <form method="post" action="/tasks/delete">
    <input type="hidden" name="id" value={id}>
    <button type="submit">削除</button>
  </form>
</dialog>
}
```

フォームが 2 つ、結果も 2 つです。`method="dialog"` は何も送信せずにダイアログを
閉じ、押されたボタンの `value` を `returnValue` に記録します。もう一方は普通の
POST でページを離れます。戻ってきたときにダイアログが消えているのは、ドキュメント
そのものが新しいからです。確認フローに状態もスクリプトも要りません。

id をパスではなく hidden フィールドで運んでいるのは、`action` が URL 属性だから
です。`string` を差し込むと生成エラーになり、求められる型は `url` です。ハンドラで
その値を組み立てる手もありますが、すでにあるフォームには hidden フィールドのほうが
たいてい手数が少なく済みます。

`command="show-modal"` と `commandfor` は、これを宣言的に開きます。この組は 2025 年に
各エンジンへ入ったばかりなので、まだ持っていないブラウザが実際に使われています。
どこでも動く必要があるボタンなら、従来のやり方で開いてください。

```html
<button type="button" data-opens="confirm-delete">削除</button>
```

```js
// public/dialogs.js
document.addEventListener('click', (event) => {
  const id = event.target.closest('[data-opens]')?.dataset.opens;
  if (id) document.getElementById(id)?.showModal();
});
```

委譲したリスナ 1 つで、サイト上のすべてのダイアログを —— あとから swap で入って
きたものも含めて —— 賄えます。読み込み時に要素へ結びつけるのではなく、クリック時に
対象を解決するからです。

非モーダルのダイアログが欲しいのでない限り、`show()` ではなく `showModal()`
（あるいは `command="show-modal"`）を使ってください。フォーカスを移し、背景を
不活性にするのはモーダル形式だけです。

## ポップオーバー

ポップオーバーは同じトップレイヤーの仕組みからモーダル性を抜いたもので、
スクリプトはまったく要りません。

```html
<button popovertarget="account-menu">アカウント</button>
<nav id="account-menu" popover class="menu">
  <a href="/profile">プロフィール</a>
  <a href="/settings">設定</a>
  <form method="post" action="/logout"><button type="submit">ログアウト</button></form>
</nav>
```

既定の `popover`（`popover="auto"` と同じ）はライトディスミスします。外側の
クリックや `Esc` で閉じ、別の auto ポップオーバーが開けばこれも閉じます。メニューが
欲しがるのはまさにこの振る舞いで、ドロップダウンライブラリがコードの大半を費やして
いるのもここです。

| 値 | 閉じる条件 | 向いているもの |
| --- | --- | --- |
| `popover` / `popover="auto"` | 外側クリック、`Esc`、別の auto が開く | メニュー、ドロップダウン、開閉パネル |
| `popover="manual"` | 明示的に閉じたときだけ | トースト、他所のクリックで消えては困るもの |
| `popover="hint"` | 別のポップオーバーが開く、トリガーがフォーカスを失う | ツールチップ、ホバーカード |

トリガーの動作は `popovertargetaction="show"` / `"hide"` / `"toggle"` で選びます。
既定は `toggle` です。

ポップオーバーはフォーカスを**閉じ込めません**し、ページの残りを不活性にもしません。
メニューにとっては正しい取引で、破壊的な確認にとっては誤った取引です。答えが重い
ときは `<dialog>` を使ってください。

### 位置合わせ

放っておくとポップオーバーはビューポートの中央に出ます。CSS アンカー
ポジショニングは、位置決めライブラリなしでトリガーに結びつけます。

```html
<head>
<style>
.trigger { anchor-name: --account }
.menu { position-anchor: --account; position-area: bottom span-right }
</style>
</head>
```

アンカーポジショニングはまだ全エンジンにはないので、強化として扱ってください。
それなしでも使える位置を与えたうえで —— メニューバーの項目なら
`position: relative` のラッパー内で普通に `position: absolute` すれば十分です
—— 対応環境では配置がよくなる、という関係にします。

## 開閉とアコーディオン

```html
<details name="faq" class="entry">
  <summary>マイグレーションはどう適用されますか？</summary>
  <p>…</p>
</details>
<details name="faq" class="entry">
  <summary>設定はどこから来ますか？</summary>
  <p>…</p>
</details>
```

`name` を共有すると排他になります。1 つ開けば他が閉じる —— アコーディオンとは
それだけのことです。`name` がなければ各 `<details>` は独立で、サイドバーが欲しいのは
たいていこちらです。

パラメータは `bool` なので、既定で開いているセクションも他と同じくデータです。

```html
export component Section(label: string, expanded: bool, children: html): html {
<details class="entry" open={expanded}>
  <summary>{label}</summary>
  <slot />
</details>
}
```

真偽属性は true のときだけ出力されるので、`expanded` が false なら `open` 属性は
そもそも書き出されません。

## ここで効いてくるテンプレートの規則

ここまではすべて普通の HTML で、規則の説明を先に必要としたものはありません。それが
変わるのは、このマークアップに欠けている 2 つ —— 自前のスタイルと、繰り返すための
リスト —— が加わったときです。以下の 4 つのうち 2 つは生成エラー、1 つは 500、
最後の 1 つは無言で、注意を払うべき順序もそのとおりです。

### スコープ付きセレクタにはクラスが要る

コンポーネントのスタイルは、宣言したクラス名を書き換えることでスコープされます。
クラスを含まないセレクタにはスコープする手がかりがありません。これは生成に
失敗します。

```
selector "dialog::backdrop" has no class to scope; add a class or wrap it in :global()
```

直し方はどちらも 1 行です。スコープを保てる前者を勧めます。

```html
<head>
<style>
.confirm::backdrop { background: rgb(0 0 0 / 0.4) }
.confirm[open] { animation: pop 120ms ease-out }
.menu:popover-open { display: grid }
:global(dialog::backdrop) { background: rgb(0 0 0 / 0.4) }
</style>
</head>
```

`::backdrop`、`[open]`、`:popover-open`、`:modal` はいずれも使えます。自分が宣言した
クラスにぶら下げる必要があるだけです。`@media` や `@supports` のブロックも中身が
同じようにスコープされます。一般規則は[スタイリング](/ja/guides/frontend/styling/)にあります。

### swap される領域のスタイルはページ側に置く

コンポーネントの `<head>` ブロックがドキュメントに届くのはページ経路だけです。
`pw.WriteHTMLFragment` には合成先の head がないので、寄与を黙って捨てるのではなく
500 で答えます。

したがって swap で届くダイアログは自分のスタイルを連れてこられません。すでにページが
描画したコンポーネントか、共有のスタイルシートで宣言してください。詳細は
[フラグメントと島](/ja/guides/frontend/fragments/)にあります。

### スクリプト内の中括弧が挿入になることがある

`<script>` の中の普通の JavaScript はそのまま通ります。クラス本体も、関数本体も、
オブジェクトリテラルも、ブラウザが実行せずに読むだけの JSON ブロックもです。挿入に
なるのは、中身が式として読める中括弧だけ。そして JavaScript には、それにそっくりな
構文が 1 つあります。

```html
<script type="module">
const shorthand = {label};
</script>
```

エラーは位置と逃げ道の両方を教えます。

```
unknown identifier label; this is inside <script> content, where {...} is a
template insertion. Write {{...}} to keep a literal brace, insert a value with
RawJavaScript or JsonForScript, or move the script to a file under the public
asset directory
```

よく踏むのはショートハンドプロパティで、もう 1 つは文字列リテラルの中の `{name}`
です。中括弧を二重にすればリテラルのまま残り、本当に値を差し込みたいときのために
スクリプト用の intrinsic が 2 つあります。

```html
<script type="module">
const config = {JsonForScript(settings)};
</script>
```

型のあるデータには `JsonForScript` を、固定文字列にだけ `RawJavaScript` を使って
ください。後者はサニタイザではなく、エスケープ規則は
[テンプレート](/ja/guides/frontend/templates/)にあります。スクリプトを `public/` の下へ
出してしまえばこの問題自体が消えますし、`script-src` ポリシーが望むのも結局これです。

```html
<script type="module" src="/public/copy-button.js"></script>
```

ここまでの話がまったく及ばない位置が 1 つあります。コンポーネントの `<head>`
**寄与ブロック**は内容がそのまま扱われるので、そこに書いた `{label}` は挿入でも
エラーでもありません。文字列 `{label}` そのものが、ドキュメントの head に届きます。

### ダイアログは行ごとではなく 1 つ

`command`/`commandfor` と `popovertarget` は `id` で解決します。行ごとに
ダイアログを吐く `for` ループは、行ごとに同じ `id` を吐きます。どのボタンも最初の
1 つを開くことになります。

id に行の識別子を与えてください。

```html
{for task in tasks}
  <li>
    <span>{task.title}</span>
    <button type="button" command="show-modal" commandfor="confirm-{task.id}">削除</button>
    <dialog id="confirm-{task.id}" class="confirm">…</dialog>
  </li>
{/for}
```

あるいは —— 長いリストではたいていこちらのほうがよく —— ループの外にダイアログを
1 つ描画し、それを開く操作に中身を用意させます。
[フラグメントのレシピ](/ja/guides/frontend/fragments/)が取っている形です。

## アクセシビリティの覚え書き

- `showModal()` はフォーカスと不活性化を引き受けます。`show()` とポップオーバーは
  引き受けません。ページの残りを使えなくしたいなら、`inert` 属性で組み直すのでは
  なくモーダル形式を使ってください。
- `<summary>` は支援技術にとってすでにボタンです。中にもう 1 つ入れないでください。
- 遷移なしに変化する領域は、変化したことを伝えるべきです。`aria-live="polite"` を
  付けてください。[フラグメントと島](/ja/guides/frontend/fragments/)の swap レシピは
  これに依存しています。
- アニメーションするものは `prefers-reduced-motion` を尊重してください。

作業といえるのは最後の 1 つだけです。残りは `<dialog>`、`<summary>`、popover 系の
属性がすでにやっていることの説明であり、同じウィジェットを `div` とリスナで組み
上げて階段を上がるのではなく、これらの要素を書くべき最大の理由でもあります。
