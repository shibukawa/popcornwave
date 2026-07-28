---
title: pw build
description: 生成コード、minify 済み CSS、準備済みアセットを含むリリースバイナリを作る。
sidebar:
  order: 6
---

```sh
pw build
```

現在のプロジェクト状態をリリース用バイナリにします。引数は取らず、入力は
`popcornwave.toml` と環境から得ます。

## 実行内容

1. [`pw generate`](/ja/pw/project/generate/) を実行する
2. Tailwind が有効なら、スタイルシートを **minify して**ビルドする。これは
   `assets.tailwind.minify` を上書きするので、リリースが誤って非 minify になることは
   ない
3. 公開アセットツリーを準備し、対応するクライアントに配信する圧縮済み `*.zstd`
   サイドカーを書き出す
4. `project.main` が開発専用パッケージに依存していればビルドを拒否する
5. `popcornwave.toml` の `project.main` に対して `go build` を実行する

バイナリは main パッケージ名でプロジェクトルートに置かれます。スキャフォールドされた
`.gitignore` は `public/**/*.zstd` とともにこれを除外済みです。

現時点で開発専用パッケージは `contrib/devidp` だけです。これは
[`pw dev`](/ja/pw/project/dev/) が使う認証プロバイダで、パスワードを検証せずに
ユーザーをログインさせます。この動作が配布用バイナリにリンクされるのは本番設定ではなく
ビルドの欠陥なので、`pw build` は import しているパッケージ名を示して停止します。

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

生成コードの経路は実行時リフレクションを使わないため、同じソースで TinyGo も
ターゲットにできます。生成後、そのコンパイラを直接呼びます。

```sh
pw generate
tinygo build -o myapp ./cmd/myapp
```

TinyGo の `net` パッケージは独自のネットワーク実装を持たず、すべてのソケットが
プログラムの登録した Netdever を経由します。TinyGo サポートを有効にして作成した
プロジェクトには、その登録を行う `tinygohelper.go` がルートに置かれます。

```go
//go:build tinygo

package publicassets

import _ "github.com/shibukawa/tinygodriver/netdev"
```

`//go:build tinygo` により、ホストの Go ビルドではこのファイルは無視されます。
これがないと TinyGo のビルド自体は成功しますが、起動直後に終了します。

```
2026/01/01 00:00:00 Netdev not set
```

`--no-tinygo` で作成したプロジェクトにはこのファイルはありません。TinyGo に
切り替えるときは手動で追加してください。

## CI での使い方

ビルド前に、生成コードが最新であることを検証します。

```sh
pw generate --check
pw build
```
