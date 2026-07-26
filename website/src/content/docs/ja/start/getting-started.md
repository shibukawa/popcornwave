---
title: はじめる
description: pw init でプロジェクトを作り、pw dev で動かし、最初の変更を加える。
sidebar:
  order: 2
---

空のディレクトリから動くアプリケーションまで進み、ページを書き換えてリロードを
確認するところまでを一通りたどります。

## 1. プロジェクトを作る

```sh
pw init myapp
```

プロジェクト名に使えるのは英数字、`-`、`_` です。`pw init` は中身のあるディレクトリへの
書き込みを拒否するので、既存ツリーでうっかり `pw init .` を実行してもファイルが
散らばることはなく、エラーで止まります。

`--tailwind` を付けると Tailwind CSS も一緒にスキャフォールドされます。`assets/app.css`
のエントリポイント、`popcornwave.toml` の `[assets.tailwind]` ブロック、Devbox への
`tailwindcss` のピン留め、ドキュメントシェルへのスタイルシートリンクが追加されます。
`package.json` も Node のロックファイルも作られません。

```sh
pw init myapp --tailwind
```

ファイルを書き出したあと、`pw init` は `go mod tidy` と `pw generate` を実行するので、
プロジェクトはすぐにコンパイルできる状態になります。最後に次の内容を表示します。

```
Created myapp

  cd myapp
  devbox shell
  pw dev
```

## 2. 生成されるもの

```
myapp/
├── popcornwave.toml           プロジェクト名、main パッケージ、dev の監視対象
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

すべての `.pw.html` と `.pw.sql` は、**ソースの隣**にある `_pw_gen.go` へコンパイル
されます。これらはビルド生成物です。Git 管理外で、VS Code では非表示になり、
`pw generate` が作り直します。手で編集するものではありません。

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

`Home` と `HomeParams` はどのファイルにも書かれていません。`handlers/home.pw.html`
から生成されます。

```html
package handlers

export component Home(name: string): html {
<h1 class="text-3xl font-bold">Hello, {name}</h1>
}
```

ハンドラが**していない**ことに注目してください。ドキュメントシェルには一切触れて
いません。`pw.WriteHTML` が `templates/document.pw.html` から登録されたドキュメントの
中にページフラグメントを描画します。詳しくは
[テンプレート](/ja/guides/templates/)を参照してください。

## 3. 動かす

```sh
cd myapp
devbox shell
pw dev
```

`pw dev` は Devbox のサービスを起動し、`pw generate` を実行し、未適用のマイグレーション
を適用し、Tailwind が有効ならウォッチャを起動し、そのうえでアプリケーションをビルドして
実行します。監視対象のファイルが変わるたびに再起動します。

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

保存すると `pw dev` が `home_pw_gen.go` を再生成し、リビルドして再起動します。
ブラウザをリロードしてください。

一方でコンポーネントの**シグネチャ**を変えた場合 —— たとえば
`Home(name: string, count: int)` —— 生成される `HomeParams` 構造体も変わり、新しい
フィールドを渡すまでハンドラはコンパイルできなくなります。これが狙いです。
テンプレートとハンドラは Go コンパイラによって相互に検査されます。

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
