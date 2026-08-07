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
3. [アセットツリー](/ja/guides/frontend/static-assets/)を `dist/public` に構築する。
   プロジェクトが指定した変換を行い、圧縮済み `*.zstd` サイドカーを書き、キャッシュ
   ヘッダを決めるマニフェストを出力する
4. `project.main` が開発専用パッケージに依存していればビルドを拒否する
5. `popcornwave.toml` の `project.main` に対して `go build` を実行する

バイナリは main パッケージ名でプロジェクトルートに置かれます。スキャフォールドされた
`.gitignore` はこれと、`dist/` 配下すべてを除外済みです。ビルド済みツリーも変換キャッシュも
マニフェストも、すべてビルド成果物です。

現時点で開発専用パッケージは `contrib/devidp` だけです。これは
[`pw dev`](/ja/pw/project/dev/) が使う認証プロバイダで、パスワードを検証せずに
ユーザーをログインさせます。この動作が配布用バイナリにリンクされるのは本番設定ではなく
ビルドの欠陥なので、`pw build` は import しているパッケージ名を示して停止します。

## 実行する

```sh
APP_ENV=prod ./myapp
```

`APP_ENV` がどのプロジェクトローカル設定ファイルを読むかを選びます。
[設定](/ja/guides/architecture/configuration/)を参照。

## クロスコンパイルと TinyGo

`pw build` は `go build` を呼び出すだけなので、通常の環境変数がそのまま使えます。

```sh
GOOS=linux GOARCH=amd64 pw build
```

生成コードの経路は実行時リフレクションを使わないため、同じソースで TinyGo も
ターゲットにできます。`pw build` は必ずホストの `go` でリンクするので、TinyGo の
ビルドは準備手順を走らせてからそのコンパイラを自分で呼びます。

```sh
pw prepare
tinygo build -scheduler=threads -o myapp ./cmd/myapp
```

[`pw prepare`](/ja/pw/project/prepare/) はこのコマンドから最後の手順を引いた
ものです。`pw generate` ではなくこちらを使ってください。生成 Go は書かれますが
`dist/public` は作られず、`public.go` が `go:embed` で名指ししているため、
一度も作られなかったツリーでコンパイラが失敗します。

`-scheduler=threads` はネットワークプロトコルを話すエンジンには必須です。協調型
スケジューラの下ではブロッキングなソケット呼び出しがランタイム全体を掴み、ドライバの
キャンセル監視が動かないため、クエリはコンテキストのデッドラインを越えても何も
報告しません。`database/postgres` と `database/mysql` は、実行時にそうなるかわりに、
このフラグが無ければコンパイルを拒否します。

`pw init` が書く 2 つの Dockerfile は、この両方のコマンドを使います。
[コンテナイメージ](/ja/guides/deployment/container-images/)を参照してください。

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
