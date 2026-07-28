---
title: はじめる
description: pw init でプロジェクトを作り、pw dev で動かし、最初の変更を加える。
sidebar:
  order: 2
---

空のディレクトリから動くアプリケーションまでは、数個のコマンドしか離れていません。
このチュートリアルではアプリケーションを作り、生成物を確認し、最後にページを変更して
開発ループを試します。

## 1. プロジェクトを作る

```sh
pw init myapp
```

プロジェクト名には英数字、`-`、`_` を使えます。それ以上に重要なのは、`pw init` が
すでにファイルを含むディレクトリへの書き込みを拒否することです。既存ツリーで
うっかり `pw init .` を実行しても、新しいファイルを散らかす前に失敗します。

`--tailwind` を付けると Tailwind CSS も一緒にスキャフォールドされます。`assets/app.css`
のエントリポイント、`popcornwave.toml` の `[assets.tailwind]` ブロック、Devbox への
`tailwindcss` のピン留め、ドキュメントシェルへのスタイルシートリンクが追加されます。
`package.json` も Node のロックファイルも作られません。

```sh
pw init myapp --tailwind
```

ファイルが揃うと、`pw init` は `go mod tidy` と `pw generate` を実行します。そのため、
コマンドが成功を報告する時点で、生成されたプロジェクトはコンパイル可能です。

```
Created myapp

  cd myapp
  devbox shell
  pw dev
```

## 2. 生成されるもの

```
myapp/
├── popcornwave.toml           プロジェクト名、main パッケージ、生成対象ディレクトリ
├── config.dev.toml            APP_ENV=dev のランタイム設定
├── go.mod
├── devbox.json                Go + Valkey（--tailwind なら tailwindcss も）
├── cmd/myapp/main.go          pw.Run を呼ぶ
├── handlers/
│   ├── index.go               パッケージレベルの mux と Handlers()
│   ├── home_handler.go        ルート登録と net/http ハンドラ
│   └── home.pw.html           型付きページテンプレート
├── templates/
│   ├── document.pw.html       共有ドキュメントシェル（doctype, html, head, body）
│   ├── templates.go           初回生成前から存在するパッケージマーカー
│   └── 400|404|500.pw.html    エラーページ
├── queries/users.pw.sql       型付き結果を持つ名前付き SQL
├── migrations/00001_init.sql  初期スキーマ（goose 形式）
├── public/.keep               空ツリーの番兵。配信されない
├── public.go                  public/ を埋め込んで登録する
├── .vscode/settings.json      **/*_pw_gen.go を隠す
└── .gitignore                 *_pw_gen.go などのビルド生成物を除外
```

すべての `.pw.html` と `.pw.sql` は、**ソースの隣**の `_pw_gen.go` になります。
これらはビルド生成物です。Git は無視し、VS Code は非表示にし、`pw generate` が
作り直します。編集するのは生成後の Go ではなく、元のソースです。

### エントリポイント

```go
package main

import (
	"context"
	"log"

	"myapp/handlers"

	"github.com/shibukawa/popcornwave/pw"
)

func main() {
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

`pw.Run` が設定のパース、起動時バリデーション、ミドルウェアスタック、配信、グレース
フルシャットダウン、逆順のリソースクリーンアップまでを引き受けます。

### ハンドラ

```go
package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
)

type homeInput struct {
	Name string `query:"name" default:"World"`
}

func init() { mux.HandleFunc("GET /", home) }

func home(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[homeInput](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	pw.WriteHTML(w, r, Home(HomeParams{Name: input.Name}))
}
```

ハンドラは `Home` と `HomeParams` を呼びますが、どちらも手書きの Go には現れません。
両方とも `handlers/home.pw.html` から生成されます。

```html
package handlers

export component Home(name: string): html {
<h1 class="text-3xl font-bold">Hello, {name}</h1>
}
```

ハンドラはドキュメントシェルに一切触れません。`pw.WriteHTML` がページフラグメントを
受け取り、`templates/document.pw.html` から登録されたドキュメントの中に描画します。
この合成については[テンプレート](/ja/guides/templates/)で詳しく説明します。

## 3. 動かす

```sh
cd myapp
devbox shell
pw dev
```

`pw dev` は Devbox のサービスを起動し、`pw generate` を実行し、未適用のマイグレーション
を適用し、Tailwind が有効ならウォッチャを起動し、そのうえでアプリケーションをビルドして
実行します。監視対象のファイルが変わるたびに再起動します。

アプリケーションは起動内容を木構造で1回だけ報告し、最後に受け付けたアドレスを出します。

```
handlers/home_pw_gen.go
queries/users_pw_gen.go
version 0 -> 1

   .-.   .-.
 .(   ) (   ).    Popcorn Wave v0.1.0
(   o     o   )   started at 2026-07-28 09:12:31 JST
(    \___/    )   env dev · config.dev.toml
 '-.__.___.__-'

configuration
├─ middleware
│  ├─ rdb
│  │  ├─ dsn             [REDACTED]  ← file
│  │  ├─ enabled         true        ← file
│  │  └─ max_open_conns  1           ← file
│  └─ request_timeout  0s
├─ observability
│  ├─ minimum_level  debug  ← file
│  └─ service_name   myapp  ← file
└─ server
   ├─ api_doc  scalar  ← file
   └─ port     8080    ← file

listening on http://localhost:8080
```

ここでは一部だけを載せています。実際の木にはフレームワークとアプリケーション双方の
解決済みキーがすべて並びます。既定値以外から来た値だけが印されます（上の `← file` の
ほか、`← env` と `← flag` があります）。`rdb.dsn` のようなキーは伏せられます。端末以外
では同じ内容が構造化ログ1レコードになります。詳しくは
[起動サマリ](/ja/guides/configuration/#起動サマリ)を参照してください。

<http://localhost:8080/> を開いてください。生成されたページはクエリパラメータにも
反応するので、<http://localhost:8080/?name=Popcorn> で名前入りの挨拶になります。

フレームワークは既定で次のパスもマウントします。

| パス | 用途 |
| --- | --- |
| `/healthz` | 死活監視 |
| `/readyz` | 準備状態 |
| `/openapi.json` | 生成された OpenAPI ドキュメント |
| `/docs` | Scalar による API ドキュメント（`config.dev.toml` のみ有効） |
| `/public/` | 埋め込み静的アセット |

## 4. 変更してみる

`handlers/home.pw.html` を編集します。

```html
package handlers

export component Home(name: string): html {
<h1 class="text-3xl font-bold">Hello, {name}</h1>
<p>Served by Popcorn Wave.</p>
}
```

保存すると `pw dev` が `home_pw_gen.go` を再生成し、アプリケーションをリビルドして
再起動します。ブラウザをリロードすると、新しい段落を確認できます。

コンポーネントの**シグネチャ**を変えると、別の結果になります。たとえば
`Home(name: string, count: int)` にすると、生成される `HomeParams` も変わり、
`Count` を渡すまでハンドラはコンパイルできません。開発ループが、実行時ではなく
ビルド時に契約の不一致を表面化させます。

## 5. 本番用にビルドする

```sh
pw build
```

コードを再生成し、Tailwind が有効なら CSS を minify してビルドし、公開アセットの圧縮
サイドカーを準備し、`popcornwave.toml` の main パッケージに対して `go build` を実行
します。

実行環境は `APP_ENV` で選びます。

```sh
APP_ENV=prod ./myapp
```

## 次のステップ

- [アーキテクチャ](/ja/start/architecture/) — フレームワークが前提とするモデル。
- [ハンドラ](/ja/guides/handlers/)と[レスポンス](/ja/guides/responses/) — `pw` API の全体。
- [pw コマンド](/ja/pw/overview/) — すべてのサブコマンド。
