---
title: pw prepare
description: ビルドに必要なファイルを生成し、コンパイル直前の状態まで準備します。pw 以外のツールでコンパイルするときに使います。
sidebar:
  order: 6.5
---

```sh
pw prepare [--debug] [--backend nethttp|fasthttp]
```

`pw prepare` は [`pw build`](/ja/pw/project/build/) から最後の手順を引いたもの
です。バイナリは作りません。コンパイラが読める状態のツリーを残して止まります。
`--backend` は dependency safety check に使う build tag を選びます。provider の
`--target` packaging は `pw build` の役割で、このコマンドでは受け付けません。

## 何をするか

1. [`pw generate`](/ja/pw/project/generate/) を実行する
2. Tailwind が有効なら、スタイルシートを**最小化して**ビルドする
3. [アセットツリー](/ja/guides/frontend/static-assets/)を `dist/public` に構築
   する。圧縮サイドカーとマニフェストも含む
4. `project.main` が開発専用パッケージに依存していれば拒否する

`pw build` がリンクの前に行うのと同じ内容、同じ順序です。`pw build` が「この
コマンド＋コンパイラ」として定義されているためで、両者がずれることはありません。

`--debug` はビルド済みツリーにソースマップを残します。
[`pw build --debug`](/ja/pw/project/build/#デバッグ用の成果物) と同じです。ただし
そのフラグのもう半分はこのコマンドの持ち物ではありません。`-ldflags` は自分で書く
コンパイラ行に属するので、ここでデバッグ用の成果物を作るなら `pw prepare --debug` と、
strip を指示しない `go build` の組み合わせになります。

4 番目がコンパイラの隣ではなくここにあるのには理由があります。このコマンドは
自分が実行しないコンパイラにツリーを渡します。`contrib/devidp` — パスワードを
確認せずにユーザーをサインインさせる開発用の IdP — を配布バイナリから締め出す
検査は、渡したあとではなく渡す前に済んでいなければなりません。

## どんなときに使うか

コンパイルを他のものが担当するのでなければ `pw build` を使ってください。担当が
移るのは次の 3 つの場合です。

**TinyGo のビルド。** `pw build` は必ずホストの `go` でリンクするので、TinyGo の
プロジェクトは準備してから自分のコンパイラを呼びます。

```sh
pw prepare
tinygo build -scheduler=threads -o myapp ./cmd/myapp
```

**制御したい `go build`。** 変わったターゲットへのクロスコンパイル、`-ldflags` の
指定、一つのツリーから複数のバイナリ。どれもコンパイラの行を自分で書く理由です。

```sh
pw prepare
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/myapp ./cmd/myapp
```

**`go build` を握るイメージビルダー。** ko や Cloud Native Buildpacks は自分で
コンパイルし、生成は代行してくれません。作業ツリーでこれを先に実行してから、
ビルダーを呼びます。

## `pw generate` では足りない

コンパイラに足りないのは生成された Go なのだから、`pw generate` で済むはずだと
考えたくなります。足りません。しかも失敗の読み違えが起きやすい形で足りません。

`pw generate` が書くのは `_pw_gen.go` だけです。`dist/public` は作りません。
`public.go` はそのディレクトリを `go:embed` で名指ししているので、`pw generate`
だけで準備したプロジェクトは、一度も作られなかったディレクトリでコンパイルに
失敗します。Tailwind を使っていればスタイルシートも欠けますが、こちらはもっと
静かに、もっと遅れて表面化します。ページがスタイル無しで描画されるだけです。

`pw generate` は狭い役割のまま、エディタと CI の `--check` ゲートのために残り
ます。コンパイラに食わせられるツリーが欲しいときは、こちらのコマンドです。

## CI では

生成コードが最新かを検証してから、準備してコンパイルします。

```sh
pw generate --check
pw prepare
go build ./cmd/myapp
```

[コンテナイメージ](/ja/guides/deployment/container-images/)ではこのコマンドを
`Dockerfile.tinygo` の中で使っています。そもそも Popcorn Wave のビルドに
ホストフェーズがある理由も、そちらにあります。
