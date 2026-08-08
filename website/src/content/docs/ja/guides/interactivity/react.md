---
title: React の統合
description: サーバー描画のページ全体はそのままに、ローカル状態が必要な一部分だけを React のルートにする。
sidebar:
  order: 7
---

ページの一部だけに React を置くことはできます。境界は明快です。Popcorn Wave は
ドキュメントと周囲の HTML を描画し、React は 1 つの要素の**内側だけ**を管理します。

難しいのは mount そのものではありません。サーバーフラグメントがその要素をあとで
差し替えるとき、古い React ルートを誰が片づけ、新しいルートを誰が起動するかです。
カスタム要素を境界にすると、その 2 つをブラウザのライフサイクルへ載せられます。

## 依存とスクリプトビルド

React はビルド時の npm 依存です。ブラウザへ配るときは、Popcorn Wave のアセット
パイプラインが依存をエントリへバンドルします。

```bash
npm install react react-dom
npm install --save-dev typescript @types/react @types/react-dom
```

型検査と JSX 変換には、プロジェクトルートの `tsconfig.json` を使います。既存の設定が
ある場合は、対応するキーをそこへ合わせてください。

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "jsx": "react-jsx",
    "lib": ["DOM", "ES2020"],
    "strict": true,
    "noEmit": true
  },
  "include": ["public/**/*.ts", "public/**/*.tsx"]
}
```

`package-lock.json` もコミットしてください。次にスクリプト変換を有効にします。

```toml
# popcornwave.toml
[assets.scripts]
enabled = true
```

現時点で内蔵ビルドがエントリとして認識するのは `.ts` です。ページ側の `<script>` は
ソースの `.ts` を指し、`type="module"` を付けます。

```html
export component TasksPage(initialCount: int): html {
<head>
  <script type="module" src="/public/islands/counter.ts"></script>
</head>
<main>
  <h1>Tasks</h1>
  <CounterIsland initial={initialCount} />
</main>
}
```

`pw build` はこの参照を見つけると、`react` と `react-dom` を含む ES module を
バンドル・minify し、ソースマップと内容ハッシュ付きのファイルを作ります。生成された
コードが指す URL も、そのハッシュ付き URL へ書き換わります。Node.js と
`node_modules` はビルド時だけ必要で、アプリケーションバイナリと一緒には配りません。

この変換は TypeScript の構文を JavaScript に落としますが、型検査はしません。
CI では `tsc --noEmit` を別に実行してください。

## サーバー側に mount 点を置く

島は React がなくても内容を読める HTML にします。ここでは、スクリプトが動くまで
現在値を表示し、操作できないことだけを `disabled` で正直に示します。

```html
export component CounterIsland(initial: int): html {
<react-counter data-initial={initial}>
  <button type="button" disabled>Count: {initial}</button>
</react-counter>
}
```

`<react-counter>` 自体は Popcorn Wave が所有します。その内側は、起動後に React が
所有します。周囲の見出し、フォーム、一覧まで React のルートへ入れる必要はありません。

## React コンポーネントとエントリを分ける

直接参照されるエントリは `.ts` に限られますが、そのエントリが import する `.tsx` は
esbuild が同じバンドルへ取り込みます。したがって React コンポーネントは通常の JSX で
書けます。

```tsx
// public/islands/counter-view.tsx
import { useState } from 'react';
import { createRoot } from 'react-dom/client';

type CounterProps = { initial: number };

function Counter({ initial }: CounterProps) {
  const [count, setCount] = useState(initial);
  return (
    <button type="button" onClick={() => setCount((value) => value + 1)}>
      Count: {count}
    </button>
  );
}

export function mountCounter(element: Element, initial: number) {
  const root = createRoot(element);
  root.render(<Counter initial={initial} />);
  return root;
}
```

`.pw.html` が参照する薄い `.ts` エントリは、カスタム要素のライフサイクルだけを
受け持ちます。

```ts
// public/islands/counter.ts
import { mountCounter } from './counter-view';

class ReactCounterElement extends HTMLElement {
  root: ReturnType<typeof mountCounter> | null = null;

  connectedCallback() {
    if (this.root) return;
    const initial = Number(this.dataset.initial ?? '0');
    this.root = mountCounter(this, initial);
  }

  disconnectedCallback() {
    this.root?.unmount();
    this.root = null;
  }
}

if (!customElements.get('react-counter')) {
  customElements.define('react-counter', ReactCounterElement);
}
```

ページの解析中に要素が見つかれば `connectedCallback` が mount します。htmx などが
フラグメントをあとから挿入した場合も同じです。逆に祖先ごと差し替えられると
`disconnectedCallback` が走り、購読やイベントを含む React ツリーを unmount します。
ページ読み込み時と swap 後で、別々の初期化コードを持つ必要がありません。

light DOM を使っていることにも意味があります。React が作るボタンはページの
スタイルシート、Tailwind のユーティリティ、テーマをそのまま受け取ります。shadow DOM
に入れると、その共有を自分でつなぎ直す必要があります。

## `createRoot` でよく、`hydrateRoot` ではない

Popcorn Wave が出した fallback のボタンは React のサーバー描画結果ではありません。
そのため、ここでは `createRoot` が最初の `render` で内側を置き換えるのが正しい動作です。

見た目が同じだからと `hydrateRoot` を使うのは安全ではありません。hydration は
`react-dom/server` が生成したものと同じ React ツリーを前提にし、不一致はバグです。
Go のテンプレートと React コンポーネントで同じ DOM を二重に定義すると、最初の表示は
速く見えても、差分が生まれた時点で警告、入力値の消失、イベントのずれにつながります。

サーバーから必要なのが初期値だけなら、`data-*` 属性か JSON を渡して client render
するほうが小さな境界になります。本当の React SSR と hydration が必要なら、Node.js の
レンダラ、React のストリーミング形式、Go 側とのデプロイ境界を含む別の構成です。
Popcorn Wave はそれを提供していません。

## フラグメントと DOM の所有権

同じノードをサーバーの swap と React の両方に更新させると、どちらの状態も信用できなく
なります。次の切り分けを保ってください。

| 操作 | 所有者 |
| --- | --- |
| `<react-counter>` の配置、`data-initial` | Popcorn Wave のテンプレート |
| `<react-counter>` の子ノード | React |
| 島の外にある一覧やフォームの差し替え | htmx またはアプリケーションの swap コード |
| 島を含む領域全体の再描画 | サーバー。古い島は unmount、新しい島は mount |

`hx-target` を `<react-counter>` の中のボタンや React が作った子要素へ向けないでください。
サーバーから初期値を取り直したい場合は、島を丸ごと含むフラグメントを返します。カスタム
要素のライフサイクルが古いルートと新しいルートを入れ替えます。

React の島を `pw.WriteHTMLFragment` から返すこと自体は問題ありません。ただし、
フラグメントは `head` へ寄与できないため、`counter.ts` の `<script>` は最初のページが
すでに読み込んでいなければなりません。島のコンポーネントから `<head>` を分けたのは
そのためです。

## サーバーへ書き込む場合

カウンタのようにブラウザ内で閉じる状態には fetch は要りません。React の操作が
サーバーへ書き込むなら、通常のハンドラへ `fetch` し、成功後に React の状態を更新するか、
サーバーが返したフラグメントで島全体を差し替えます。

`security.csrf.enabled = true` の場合、unsafe なリクエストには既定名の `pw_csrf` クッキーを
リクエスト時に読み、`X-CSRF-Token` ヘッダとして送る必要があります。設定で名前を変えた
場合は、クライアントも同じ値に合わせます。描画時の値を props へ固定すると、別タブで
ログインしてセッションがローテートしたあとに古くなります。
[htmx の統合](/ja/guides/interactivity/htmx/#csrf-を有効にした書き込み)にあるクッキー読み取りは、
`fetch` の `headers` にもそのまま使えます。

## 追加したいビルドサポート

いちばん小さく、効果の大きい追加は **`.tsx` をスクリプトのエントリとして認識すること**
です。import された `.tsx` はすでに処理できるため、React を使う機能そのものが欠けて
いるわけではありません。内部では esbuild を使っており、npm 依存の解決、バンドル、minify、
ソースマップ、内容ハッシュまで揃っています。現在 `.ts` に限定されている入口を `.tsx`
にも広げ、`type="module"` の検査と参照書き換えを同じように適用すれば、薄い `.ts`
shim が不要になります。

その次に有用なのは、任意の `pw add react` が次をまとめて用意することです。

- `[assets.scripts]` と `.tsx` エントリ
- `react` / `react-dom`、型パッケージ、`tsc --noEmit` のスクリプト
- mount/unmount を担う小さなカスタム要素の雛形

一方、Node.js のインストールや `npm install` を `pw build` が暗黙に行うべきでは
ありません。ロックファイルとパッケージマネージャはアプリケーション側の供給網だからです。
React SSR も「部分的に React を使う」ための前提ではなく、独立した大きな機能です。

したがって優先順位は、まず `.tsx` エントリ、次に明示的な scaffold、SSR は実際に
hydration を必要とするユースケースが出てから、となります。いまのビルドでも JSX を
使った部分 mount は成立しますが、直接の `.tsx` 対応が入るとエントリのためだけの
`.ts` shim を置かずに済みます。
