---
title: フラグメントと島
description: サーバーで描画したフラグメントを、ダイアログ、ポップオーバー、カスタム要素と組み合わせる方法と、連携時の規則。
sidebar:
  order: 5
---

:::note[境目]
このページのフラグメント API はフレームワークのものです。swap ライブラリ、
ポップオーバー、カスタム要素、そしてそれらが呼ぶブラウザ API は違います。
アプリケーションが選び、失敗も含めて引き受けるものです。
:::

ブラウザだけを使う方法では、ページがすでに持つマークアップを並べ替えます。
フラグメントリクエストが向くのは、サーバーで新しいマークアップを作る場合です。
たとえば、データベースで絞り込むリスト、選んだ行によって内容が変わるパネル、
ダイアログを閉じずに検証エラーを返すフォームです。

API は 1 つです。`pw.WriteHTMLFragment` は、ドキュメントシェル、合成された head、
フレーミングを付けずにテンプレート 1 つを描画します。この契約は
[レスポンス](/ja/guides/frontend/responses/)で説明しており、`examples/htmx_fragment` には
これを使った完全なアプリケーションがあります。

返されたマークアップを現在のドキュメントへどう組み込むかは、アプリケーションが
決めます。以下では `<dialog>` を使ってその境界を示します。既存のマークアップを
表示・非表示にするだけなら、フラグメントを取得せずブラウザの標準機能を使うほうが軽量です。

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

ライブラリが追加・削除するクラスはコンパイル時に追跡できないため、スコープの外でも適用するには
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

## swap がタイマーになるとき

遅かれ早かれ、誰もクリックしていないのに変わらなければならない領域が出てきます。
swap ライブラリはそれをやってくれます。

```html
<div id="queue" hx-get="/queue" hx-trigger="every 2s" hx-swap="innerHTML">…</div>
```

2 秒は当て推量で、しかも両方向に同時に外れています。ほとんどのリクエストは動いて
いないキューの深さを描き直し、意味のあった変化は最大 2 秒待たされる。間隔を縮めれば
前者が増え、伸ばせば後者が深くなります。そのあいだ、開いているタブはそれぞれ自前の
タイマーを持つので、負荷はイベントの数ではなくタブの数に比例します。サーバーの側から
「何も起きていないから訊くのをやめてくれ」と言う手段はありません。

live 境界は、言うことができたときにサーバーが話す形にして、この当て推量を消します。
テンプレートは前節と同じ `await` 節のままで、違うのは一度で確定せず出し続ける
ソースを束縛することだけです。

```html
external live WatchQueue(): Depth

export component Queue(): html {
<section class="card" id="queue">
{await depth = WatchQueue()}
  <strong>{depth.waiting}</strong> 件待ち · <small>{depth.at}</small>
{fallback}
  <p class="pending">接続中…</p>
{/await}
</section>
}
```

```go
func WatchQueue(ctx context.Context) iter.Seq2[Depth, error] {
	return func(yield func(Depth, error) bool) {
		for event := range queue.Watch(ctx) {
			if !yield(Depth{Waiting: event.Waiting, At: event.At.Format("15:04:05")}, nil) {
				return
			}
		}
	}
}
```

マークアップに属性はなく、間隔の指定もなく、エンドポイントもありません。ブラウザは
ページ自身の URL に接続し直し、ドキュメントシェルがすでに読み込んでいるランタイムが
配信を適用します。届くのは領域を丸ごと描き直したものです。だからソースは差分ではなく
現在の状態を yield します。接続が切れたあとの再接続に必要なのは次の 1 回だけ、という
形になるからです。

手を出す前に知っておく価値のあることが 3 つあります。

live な領域には出力を置き、入力は置きません。live 節の中の `form`、`input`、
`textarea`、`select` は生成に失敗します。読み手が入力している最中に配信が届けば、
打ち込んだものが警告もなく消えるからです。フォームは境界の外に、変わるデータは
境界の中に置いてください。

読み上げの扱いは選ぶ側の仕事で、有用な答えは 2 つあって正反対です。毎秒描き直される
ゲージは何も読み上げるべきではなく、メッセージの一覧は polite を含む `role="log"` の
中に置くのが適切です。属性は境界の**外側**の要素に付けてください。置き換えられる
サブツリーの中にあるものは中身ごと壊れて作り直され、ライブリージョンがリセットされます。

そして、動いたのが読み手であるかぎり swap のほうが勝ちます。絞り込み、並べ替え、
インライン編集、ページ送り —— どれも swap のほうが書くのが速く、配信も安く済みます。
開いた接続もサブスクリプションも要らないからです。live 配信が担うのは swap では
表現できない場合、つまり値が変わったのに、変えたのはこのページの誰でもない場合です。

[ライブレンダリング](/ja/guides/cross-layer/live-rendering/)に上限・再接続の挙動・
接続 1 本のコストがあります。`examples/live_render` はそれで組んだ動くダッシュボードです。

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
  [非同期レンダリング](/ja/guides/cross-layer/async-rendering/)を参照してください。

### 島はどこへ POST するか

何かを変更する島には宛先が要ります。テンプレートに `/users/42/rename` と直書きする
のは、シンボルであるべき場所に文字列を置くことです。ページツリーの中なら
`server-action="Rename"` でエクスポートされた Go のハンドラを名指しでき、生成が
`data-tb-action="/_action/…/Rename"` へ落とします。関数名を変えれば、クリックが
実行時に失敗するのではなく、参照しているテンプレートで生成が失敗します。

その属性に対して動くのはフレームワークのランタイムです。クリックを横取りし、その
アドレスへ POST し、返ってきた領域を適用します。現在の `pw_csrf` クッキーを
`X-CSRF-Token` ヘッダへ移すのも、その途中で済みます。アクションを撃つ島が自分で
配線するものは何もありません。変更の前に島が判断を挟む場合は、要素から
`server-action` を外し、`window.popcornweb.updateHeaders()` と
`window.popcornweb.apply()` を使って自分でリクエストを出してください。同じトークンを
運び、同じように領域を適用します。[サーバーアクション](/ja/guides/interactivity/server-actions/)と
[React の統合](/ja/guides/interactivity/react/#サーバーへ書き込む場合)を参照してください。

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
- **オフラインや長期間保持するクライアント状態。** ノート PC を閉じた後も残す必要がある
  フォームには、自分で書いて自分で突き合わせるストレージが要ります。
- **状態を強く持つエディタ。** 表計算、作図キャンバス、リアルタイム共同編集。これらは
  本当にクライアントアプリケーションであり、正直な形は、swap を積み上げて作ることでは
  なく、このフレームワークが配信するページの上に 1 つ載せることです。

そこに至らないものは 4 つの段でよく賄えますし、多くのインターフェースは上の 2 段に
たどり着きません。
