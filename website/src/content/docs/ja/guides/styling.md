---
title: スタイリング
description: コンポーネントスコープのスタイルと、Tailwind CSS の有効化。あとから有効にする手順も。
sidebar:
  order: 6
---

2 つのアプローチが共存します。コンポーネントスタイルはテンプレートシステムに含まれる
ので設定は不要で、Tailwind CSS はオプトインです。

## コンポーネントスタイル

コンポーネントは自分の head の内容を提供でき、そのクラス名は自動的にスコープされます。

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

ブロック内で宣言されたクラスはリネームされ、対応する `class` 属性も書き換えられます。
そこで宣言されて**いない**クラスは手を加えられずに通過するので、同じ属性の中で Tailwind
のユーティリティとスコープ付きルールを並べられます。
[テンプレート](/ja/guides/templates/)を参照。

## Tailwind CSS

Popcorn Wave は**スタンドアロン**の Tailwind バイナリを実行します。`package.json` も
`node_modules` も Node のロックファイルもありません。

いちばん簡単なのは作成時に指定する方法です。

```sh
pw init myapp --tailwind
```

### あとから有効にする

`--tailwind` を付けずに作ったプロジェクトでは、次の 4 つを揃える必要があります。
いずれも生成されないので、手で追加してください。

**1. `tailwindcss` バイナリを使えるようにする**

`pw dev` と `pw build` は `PATH` 上の `tailwindcss` を探し、見つからなければ明確な
メッセージで失敗します。`devbox.json` に追加します。

```json
{
  "$schema": "https://raw.githubusercontent.com/jetify-com/devbox/0.14.2/.schema/devbox.schema.json",
  "packages": ["go@latest", "valkey@latest", "tailwindcss_4@4.1.18"],
  "shell": {"init_hook": ["echo 'Popcorn Wave development environment'"]}
}
```

新しいパッケージを `PATH` に載せるためシェルに入り直します。

```sh
devbox shell
```

バージョンを固定するのは意図的です。CSS ツールチェインをピン留めしないと、再現可能な
ビルドが動く標的に変わってしまいます。別の方法でツールを管理しているなら、`PATH` 上に
`tailwindcss` があればそれで構いません。

**2. スタイルシートのエントリポイントを作る**

```css
/* assets/app.css */
@import "tailwindcss";
@source "../handlers";
@source "../templates";
```

`@source` の行が、`.pw.html` の中のクラス名を Tailwind に見せるための指定です。これが
ないと出力はほぼ空になります。テンプレートを含むディレクトリごとに 1 行ずつ追加して
ください。[プロジェクト構成](/ja/guides/project-structure/)のレイアウトなら
`@source "../webroot";` になります。

`@import "tailwindcss"` の行はビルド前に検証されるので、壊れたエントリポイントは
黙って空の CSS を出力するのではなくエラーとして報告されます。

**3. `popcornwave.toml` で有効にする**

```toml
[assets.tailwind]
enabled = true
input = "assets/app.css"
output = "public/generated/app.css"
minify = true
```

`input` と `output` はプロジェクトルートからの相対パスで、互いに異なる必要があります。
`enabled` が true でパスを省略した場合、この 2 つが既定値になります。

**4. ドキュメントシェルから出力をリンクする**

```html
package templates

export component Document(children: html?): html {
<!doctype html>
<html lang="en"><head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>My App</title>
  <link rel="stylesheet" href="/public/generated/app.css">
</head>
<body><slot /></body></html>
}
```

URL は `server.public.mount`（既定 `/public`）に、`public/` 内での `output` のパスを
続けたものです。

あとは開発サーバを起動します。

```sh
pw dev
```

### 実行のされ方

| コマンド | 動作 |
| --- | --- |
| `pw dev` | 非 minify で 1 回ビルドしてからウォッチャを起動。ウォッチャが落ちたら入力を再監視 |
| `pw build` | ファイルの `minify` 設定に関わらず minify して 1 回ビルド |

出力はいったん一時ファイルに書かれてからリネームされるので、書きかけのスタイルシートが
配信されることはありません。`public/generated/app.css` はビルド生成物です。スキャフォールド
された `.gitignore` は圧縮サイドカー `public/**/*.zstd` をすでに除外していますが、生成
された CSS も無視対象にしておくとよいでしょう。

### プラグイン

エントリポイントから参照されるローカルプラグインは、エントリポイントからの相対で解決
され、ビルド前に存在が確認されます。

```css
@import "tailwindcss";
@plugin "./plugins/typography.mjs";
@source "../handlers";
```

モジュールはプロジェクト内に置いてください。`assets/plugins/*.mjs` がスキャフォールド
の慣習です。これによりパッケージマネージャなしでビルドの再現性が保たれます。
