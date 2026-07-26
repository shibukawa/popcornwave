---
title: pw build
description: 生成コード、minify 済み CSS、準備済みアセットを含むリリースバイナリを作る。
sidebar:
  order: 4
---

```sh
pw build
```

リリース用のバイナリをビルドします。引数は取りません。

## 実行内容

1. [`pw generate`](/ja/pw/project/generate/) を実行する
2. Tailwind が有効なら、スタイルシートを **minify して**ビルドする。これは
   `assets.tailwind.minify` を上書きするので、リリースが誤って非 minify になることは
   ない
3. 公開アセットツリーを準備し、対応するクライアントに配信する圧縮済み `*.zstd`
   サイドカーを書き出す
4. `popcornwave.toml` の `project.main` に対して `go build` を実行する

バイナリは main パッケージ名でプロジェクトルートに置かれます。スキャフォールドされた
`.gitignore` は `public/**/*.zstd` とともにこれを除外済みです。

## 実行する

```sh
APP_ENV=prod ./myapp
```

`APP_ENV` がどのプロジェクトローカル設定ファイルを読むかを選びます。
[設定](/ja/guides/configuration/)を参照。

## クロスコンパイルと TinyGo

`pw build` は `go build` を呼び出すだけなので、通常の環境変数がそのまま使えます。

```sh
GOOS=linux GOARCH=amd64 pw build
```

生成コードの経路で実行時リフレクションを使っていないため、同じソースは TinyGo も
ターゲットにできます。生成後にそのコンパイラを直接呼んでください。

```sh
pw generate
tinygo build -o myapp ./cmd/myapp
```

## CI での使い方

ビルド前に、生成コードが最新であることを検証します。

```sh
pw generate --check
pw build
```
