---
title: フラグメントと島
description: サーバー描画のフラグメントをダイアログ・ポップオーバー・カスタム要素と組み合わせる。そして両者の噛み合い方を決めるフレームワーク側の規則。
sidebar:
  order: 5
---

これより下の段は、ページがすでに持っているものを並べ替えるだけです。この段は
それ以外のためにあります。データベースで絞り込むほかないリスト、中身が行に依存する
パネル、ダイアログを開いたままエラーとともに戻ってくるフォーム。

API は 1 つの呼び出しです。`pw.WriteHTMLFragment` はテンプレート 1 つだけを描画し
—— ドキュメントシェルも、合成された head も、フレーミングもなく ——
契約は[レスポンス](/ja/guides/frontend/responses/)に、それだけで組んだ完全なアプリケーションは
`examples/htmx_fragment` にあります。

その契約が開いたままにしているところが、詰める価値のある部分です。フラグメントは
ドキュメントに囲まれていないマークアップで、`<dialog>` はドキュメントの一部です。
この 2 つをどう噛み合わせるかから、以下のレシピが出てきます。

## すべてのレシピを形づくる 3 つの規則

3 つとも「ドキュメントがない」ことから出てきますし、swap ライブラリより多くの設計を
決めます。

| 規則 | 帰結 |
| --- | --- |
| フラグメントは head に寄与できない | swap される領域のスタイルとスクリプトは、すでに読み込まれているページのもの |
| フラグメントはストリーミングしない | `await` 境界はサーバーで確定し、`fallback` はブラウザに届かない |
| フラグメントはエンベロープを持たない | 古い swap と新しい swap をフレームワークは区別できず、順序は swap ライブラリの問題 |

1 つ目は強制されます。`<head>` ブロックを持つテンプレートは、寄与を捨てるのではなく
500 で答えます。この失敗は意図的なもので、コンポーネントをページ経路から swap へ
移すときに最もよく出会う驚きです。

## 中身をサーバーが用意するダイアログ

ダイアログ要素はページに置きます。取りに行くのは中身だけです。

```html
<dialog id="drawer" class="drawer">
  <div id="drawer-body"></div>
  <form method="dialog"><button value="close">閉じる</button></form>
</dialog>
```

```html
{for task in tasks}
  <li>
    <span>{task.title}</span>
    <button type="button" hx-get="/tasks/{task.id}/edit"
            hx-target="#drawer-body" hx-swap="innerHTML">編集</button>
  </li>
{/for}
```

ルートはパネルだけを返します。

```go
func editForm(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[editInput](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	task, ok := tasks.find(input.ID)
	if !ok {
		pw.WriteProblem(w, r, pw.NotFound("no such task"))
		return
	}
	pw.WriteHTMLFragment(w, r, EditForm(EditFormParams{Task: task}))
}
```

開く 1 行だけは、どちらの側も提供しません。委譲したリスナにしておけば、ボタンごとに
書かずに済みます。

```js
// public/drawer.js
document.body.addEventListener('htmx:afterSwap', (event) => {
  if (event.detail.target.id === 'drawer-body') {
    document.getElementById('drawer').showModal();
  }
});
```

この配置が避けているものに注目してください。ダイアログ自体は swap されないので開いた
状態を失いません。スタイルはページ側にあるので head の規則を満たします。そして
すでにモーダルになっているダイアログへの `showModal()` は何もしないので、開いたままの
ドロワーへの 2 回目の swap は中身の更新になります。

ドロワーの中のフォームは、そのままフラグメントのステータス契約に従います。拒否された
送信は `#drawer-body` を対象に HTML と 200 で戻り、ダイアログはエラーを乗せたまま
開いています。成功した送信は一覧向けに更新された行を返し、同じリスナからドロワーを
閉じられます。

## クリックで消えないトースト

`popover="manual"` はライトディスミスしない変種で、通知が必要とするのはこちらです。
要素はページに置き、レスポンスが out-of-band で埋めます。

```html
<output id="toast" popover="manual" class="toast" aria-live="polite"></output>
```

フラグメントのレスポンスは単なるマークアップなので、1 つのテンプレートが差し替え
対象の領域と out-of-band の要素の両方を運べます。

```html
export component TaskList(tasks: Task[], note: string): html {
<ul id="task-list" class="task-list">
{for task in tasks}
  <li>{task.title}</li>
{/for}
</ul>
{if note != ''}
<output id="toast" popover="manual" class="toast" aria-live="polite" hx-swap-oob="true">{note}</output>
{/if}
}
```

```js
// public/toast.js
document.body.addEventListener('htmx:oobAfterSwap', () => {
  const toast = document.getElementById('toast');
  if (!toast || !toast.textContent.trim()) return;
  toast.showPopover();
  setTimeout(() => toast.hidePopover(), 4000);
});
```

リスナが渡された要素を保持せず id で引き直しているのは、out-of-band の swap が
一致する要素を**置き換える**からです。同じ事実がレスポンス側のマークアップも
説明します。`popover` 属性をそこで繰り返す必要があるのは、属性が着地先の穴ではなく、
差し込まれる要素のものだからです。

## 待機状態

ページ経路では、`await` 境界が自分の `fallback` を宣言し、値が確定するとランタイムが
差し替えます。フラグメント経路ではそれが起きません。レスポンスはバッファされ、境界は
サーバーで確定し、ボディは完成した状態で届きます。`fallback` は遅れているのではなく、
そもそも送られません。

したがって swap の待機表示はクライアントのものです。

```html
<button hx-get="/tasks/summary" hx-target="#summary" hx-indicator="#summary-spinner">
  更新
</button>
<span id="summary-spinner" class="spinner">集計中…</span>
```

```html
<head>
<style>
.spinner { visibility: hidden }
:global(.htmx-request) .spinner, .spinner:global(.htmx-request) { visibility: visible }
</style>
</head>
```

ライブラリが付け外しするクラスは自分のものではないので、スコープを生き延びさせるには
`:global(...)` が要ります。純粋な CSS で済ませる手もあります。ターゲットにスケルトンの
スタイルを与え、届いたマークアップに置き換えさせれば、この結合自体がなくなります。

## 1 つのコンポーネント、2 つの呼び出し元

ページと swap が、行の見た目について食い違えてはいけません。定義が 1 つなら
食い違えません。

```html
<TaskList tasks={tasks} emptyLabel={emptyLabel} />
```

最初の描画では `Home` がこれを呼び、以降の swap では部分ルートが単体で呼びます。
両方の呼び出し元が同じパラメータリストで型チェックされるので、片方を壊す変更は
同じデータの 2 通りの描画ではなくビルドの停止になります。ここでフラグメントが安く
つくのはこの性質のおかげで、設計の指針にする価値もあります。**swap が差し替える
領域は、ルートである前にコンポーネントであるべきです。**

## 自分で書く島

サーバーとの往復がまったく関わらない操作もあります。コピーボタン、ドラッグでの
並べ替え、canvas。それが階段の最上段で、自然な境界はカスタム要素です。

```html
<copy-button data-value={task.id} class="copy">
  <button type="button">ID をコピー</button>
</copy-button>
```

```js
// public/copy-button.js
class CopyButton extends HTMLElement {
  connectedCallback() {
    this.addEventListener('click', async () => {
      await navigator.clipboard.writeText(this.dataset.value);
      this.querySelector('button').textContent = 'コピーしました';
    });
  }
}
customElements.define('copy-button', CopyButton);
```

サーバー描画のアプリケーションにとってこの形が正しい理由は 3 つあります。

- **サーバーの HTML が完成している。** スクリプトがなくても、読み手に見えるのは空の箱
  ではなく「何も起きないボタン」です。すでに意味のあるマークアップを強化してください。
- **swap されたフラグメントは自分で upgrade する。** swap で挿入された要素は upgrade
  され、`connectedCallback` が走ります。描き直された領域の中の島に再初期化フックは
  要りません —— ページ読み込み時にハンドラを結びつける方式が取りこぼすのは、まさに
  この帳尻合わせです。
- **フレームワーク自身のランタイムの読み込み方と同じ。** 非同期レンダリングの
  ランタイムは、リビジョン入りのパスから `src` で読み込まれるモジュールです。自作の島も
  `public/` 下のファイルに置けば、同じ `script-src 'self'` ポリシーの内側に収まります。
  [非同期レンダリング](/ja/advanced/async-rendering/)を参照してください。

### 島はどこへ POST するか

何かを変更する島には宛先が要ります。テンプレートに `/users/42/rename` と直書きする
のは、シンボルであるべき場所に文字列を置くことです。ページツリーの中なら
`server-action="Rename"` でエクスポートされた Go のハンドラを名指しでき、生成が
`data-pw-action="/_action/…/Rename"` へ落とします。関数名を変えれば、クリックが
実行時に失敗するのではなく、参照しているテンプレートで生成が失敗します。

その属性に対して何をするかは、今日のところ自分の仕事です。それを横取りする
フレームワークのモジュールはまだありませんし、ambient credential で届く `POST`
エンドポイントの前に立つべき CSRF ミドルウェアもまだありません。これを撃つ島は
両方を引き受けることになります。[探索型ルーティング](/ja/advanced/discovered-routing/)を
参照してください。

light DOM を勧めます。shadow root はここではめったに要らないカプセル化を買う代わりに、
ページのスタイルシートを失わせます。サーバーが描画した子要素に付いた Tailwind
ユーティリティや daisyUI のクラスが、その内側では効かなくなります。

状態はサーバーが作った DOM に置いたままにしてください。データの写しを自前で持つ島は
写しを 2 つ抱えることになり、次の swap がその片方を置き換えます。

## そもそもライブラリをどう読み込むか

ドキュメントシェルが読み込むものは、すべてのページが払うものです。選択肢とその代償は
`examples/htmx_fragment` で検討しています。バージョンと Subresource Integrity で
固定した CDN の URL か、同じファイルを `public/` にベンダリングするか。後者はサード
パーティのオリジンを、ネットワーク経路からも `script-src` ポリシーからも消します。

自前でアセットを配っているアプリケーションなら、ベンダリングのほうが既定として
優れています。`pw build` は `public/` を事前圧縮し、ディレクトリは埋め込まれるので、
ベンダリングの費用はコミットされたファイル 1 つとビルド設定ゼロです。

## ここで止まる

階段には上端があり、はっきり名指ししておく価値があります。ハイドレーションも、
クライアントサイドルーターも、クライアント状態ストアもないので、いくつかのものは
どの段でも安くなりません。

- **楽観的更新。** 読み手に見せる変更はすべて、サーバーへ行って戻ってきています。
- **オフラインや長寿命のクライアント状態。** 閉じたノート PC をまたいで生き延びる
  フォームには、自分で書いて自分で突き合わせるストレージが要ります。
- **状態を強く持つエディタ。** 表計算、作図キャンバス、リアルタイム共同編集。これらは
  本当にクライアントアプリケーションであり、正直な形は、swap を積み上げて作ることでは
  なく、このフレームワークが配信するページの上に 1 つ載せることです。

そこに至らないものは 4 つの段でよく賄えますし、多くのインターフェースは上の 2 段に
たどり着きません。
