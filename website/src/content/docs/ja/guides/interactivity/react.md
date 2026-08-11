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
カスタム要素でも実現できますが、Popcorn Wave にはすでに必要なライフサイクルがあります。
[コンポーネントスクリプト](/ja/guides/interactivity/component-scripts/)は描画された
インスタンスごとに `setup` を走らせ、差し替え前にその teardown を呼びます。

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

書いた React のエントリを module script から参照します。最初のドキュメントがbundleを
読み込み、下にある島のコンポーネントスクリプトがインスタンスごとのmountとteardownを
所有します。

```html
export component TasksPage(initialCount: int): html {
<head>
  <script type="module" src="/public/islands/counter.tsx"></script>
</head>
<main>
  <h1>Tasks</h1>
  <CounterIsland initial={initialCount} />
</main>
}
```

ビルドはエントリを `react` と `react-dom` ごとバンドル・minifyし、ソースマップと
内容ハッシュ付きのファイルを作ってscript URLを書き換えます。JSX変換は
`tsconfig.json` から読みます。Node.js と `node_modules` はビルド時だけ必要で、
アプリケーションバイナリと一緒には配りません。

この変換は TypeScript の構文を JavaScript に落としますが、型検査はしません。
CI では `tsc --noEmit` を別に実行してください。

## サーバーマークアップの隣にライフサイクルを置く

島は React がなくても内容を読める HTML にします。ここでは、スクリプトが動くまで
現在値を表示し、操作できないことだけを `disabled` で正直に示します。

```html
export component CounterIsland(initial: int): html {
<script component>
export function setup(el) {
  return window.mountCounter(el, Number(el.dataset.initial ?? "0"));
}
</script>
<section class="counter" data-initial={initial}>
  <button type="button" disabled>Count: {initial}</button>
</section>
}
```

bundle側は、アプリケーション自身のmount関数を提供します。戻り値はコンポーネント
スクリプトを経由してランタイムへ渡すcleanupです。

```tsx
// public/islands/counter.tsx
import { useState } from "react";
import { createRoot } from "react-dom/client";

function Counter({ initial }: { initial: number }) {
  const [count, setCount] = useState(initial);
  return (
    <button type="button" onClick={() => setCount((value) => value + 1)}>
      Count: {count}
    </button>
  );
}

declare global {
  interface Window {
    mountCounter(el: HTMLElement, initial: number): () => void;
  }
}

window.mountCounter = (el: HTMLElement, initial: number) => {
  const root = createRoot(el);
  root.render(<Counter initial={initial} />);
  return () => root.unmount();
};
```

`mountCounter` はPopcorn WaveのAPIではなく、asset buildがURLを所有するbundleを
生成済みコンポーネントモジュールから呼ぶためだけのアプリケーション側の橋です。

`<section>` 自体は Popcorn Wave が所有します。その内側は、`setup` の起動後に React が
所有します。周囲の見出し、フォーム、一覧まで React のルートへ入れる必要はありません。

ランタイムは最初のページでも、部分更新や live 更新で挿入されたインスタンスでも
`setup` を呼びます。祖先を差し替える前には戻り値を呼び、React ツリーを unmount して
effect、購読、イベントを解放します。この
[コンポーネントスクリプトのライフサイクル](/ja/guides/interactivity/component-scripts/#解放は差し替えの前に走る)
が、サーバー側の DOM ライフサイクルと一致します。

複数の島で React コンポーネントを共有するなら、別の TypeScript ファイルへ分けて
`counter.tsx` から import します。mount 点と teardown を所有するのはテンプレート宣言
なので、`setup` はそこに残します。

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
| `.counter` の配置、`data-initial` | Popcorn Wave のテンプレート |
| `setup` 後の `.counter` の子ノード | React |
| 島の外にある一覧やフォームの差し替え | htmx またはアプリケーションの swap コード |
| 島を含む領域全体の再描画 | サーバー。古い島は unmount、新しい島は mount |

`hx-target` を `.counter` の中のボタンや React が作った子要素へ向けないでください。
サーバーから初期値を取り直したい場合は、島を丸ごと含むフラグメントを返します。
コンポーネントスクリプトの teardown が古いルートと新しいルートを入れ替えます。

React の島を `pw.WriteHTMLFragment` から返すこと自体は問題ありませんが、フラグメントは
React bundleをheadへ追加できません。最初のページがすでに `counter.tsx` を読み込んでいる
必要があり、後から来るインスタンスはそれを再利用して各自の `setup` を走らせます。

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

ハッシュ名やソースマップを含め、スクリプトビルドがファイルに対して何をするかは
[静的アセット](/ja/guides/frontend/static-assets/)にまとめてあります。`public` 配下の
ほかのファイルに起きる変換も同じページです。
