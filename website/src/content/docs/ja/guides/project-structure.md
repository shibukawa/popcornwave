---
title: プロジェクト構成
description: 単一の handlers パッケージを超えて成長させる。ハンドラとクエリの階層化、popcornwave.toml が制御するもの。
sidebar:
  order: 5
---

`pw init` は `handlers` パッケージと `queries` パッケージを 1 つずつ作ります。最初の
プロジェクトにはちょうどよい規模です。このページでは、それが手狭になったときに何が
変わるかを説明します。

## 生成が見つけるもの

`pw generate` はプロジェクトツリー全体を走査し、`.go`、`.pw.html`、`.pw.sql` の
**いずれかを含むすべてのディレクトリ**に対して生成を行います。`.git`、`vendor`、
`node_modules`、`.devbox` は除外されます。

この 1 点がほとんどの説明を不要にします。パッケージを追加する作業とはディレクトリを
作ることであり、登録するパッケージのリストは存在せず、`popcornwave.toml` にも列挙され
ません。

## 大きめのレイアウト

対象読者が分かれてきたアプリケーションでよくある形です。

```
myapp/
├── popcornwave.toml
├── cmd/myapp/main.go
├── templates/
│   ├── document.pw.html          唯一のドキュメントシェル
│   └── 400|404|500.pw.html
├── migrations/
├── webroot/
│   ├── index.go                  ルートの mux。サブアプリをマウントする
│   ├── home_handler.go
│   ├── home.pw.html
│   ├── admin/
│   │   ├── index.go              admin の mux
│   │   ├── dashboard_handler.go
│   │   ├── dashboard.pw.html
│   │   └── queries/
│   │       └── reports.pw.sql
│   └── public/
│       ├── index.go              public の mux
│       ├── signup_handler.go
│       ├── signup.pw.html
│       └── queries/
│           └── accounts.pw.sql
└── queries/
    └── users.pw.sql              複数のエリアが共有するクエリ
```

ハンドラとそれが描画するテンプレートは同じディレクトリに置き、各エリアは自分だけが
使うクエリを持ちます。複数のエリアで使うクエリは上位の共有パッケージへ移します。
所有権がパスから見て取れる、というのが要点です。

### 各エリアが mux を持つ

末端のパッケージはスキャフォールドされた `handlers` パッケージとまったく同じ形です。

```go
// webroot/admin/index.go
package admin

import "github.com/shibukawa/popcornwave/pw"

var mux = pw.NewServeMux()

func Handlers() *pw.ServeMux { return mux }
```

```go
// webroot/admin/dashboard_handler.go
package admin

func init() { mux.HandleFunc("GET /dashboard", dashboard) }
```

ここで登録するパスは、**そのエリアがマウントされる位置からの相対**です。

### ルートがマウントする

```go
// webroot/index.go
package webroot

import (
	"net/http"

	"myapp/webroot/admin"
	"myapp/webroot/public"

	"github.com/shibukawa/popcornwave/pw"
)

var mux = pw.NewServeMux()

func init() {
	mux.Handle("/admin/", http.StripPrefix("/admin", admin.Handlers()))
	mux.Handle("/", public.Handlers())
}

func Handlers() *pw.ServeMux { return mux }
```

```go
// cmd/myapp/main.go
func main() {
	if err := pw.Run(context.Background(), webroot.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

これが成立する理由は依存の向きです。親が子をインポートするので、Go は子の `init`、
つまりルート登録を親の `init` より先に実行します。子は親をインポートしないため循環も
起きません。

サブツリーのパターン（`"/admin/"`）と `http.StripPrefix` は素の `net/http` であり、
フレームワーク固有の仕組みは関係しません。

:::caution
エリアを `/public/` にマウントするのは避けてください。フレームワークは埋め込み静的
アセットを `server.public.mount`（既定 `/public`）で配信しており、自分のルートと有効な
運用エンドポイントの衝突は起動時に報告されます。別の場所にマウントするか、
[設定](/ja/guides/configuration/)でアセットのマウント位置を移動してください。
:::

## 分割されないもの

パッケージをいくら増やしても、次の 3 つは分割されません。

**ドキュメントシェル。** プロジェクトに `document.pw.html` はちょうど 1 つで、ツリーの
どこかに 2 つ以上あると生成エラーになります。エリアごとに別のシェルを使いたい場合は、
名前なしスロットを持つ通常のエクスポート済みコンポーネントを書き、ハンドラごとに
`pw.WriteHTMLChain` で選択してください。[テンプレート](/ja/guides/templates/)を参照。

**マイグレーション。** アプリケーション全体で 1 つの順序付き集合が `migration.dir` に
あります。

**設定の prefix。** 各エリアが自分の設定構造体を登録できますが、prefix は 1 つの名前空間
を共有します。[設定](/ja/guides/configuration/)を参照。

## `popcornwave.toml`

プロジェクトファイルは小さく、そのキーは**閉じた集合**です。未知のキーは警告ではなく
エラーになります。

```toml
[project]
name = "myapp"
main = "./cmd/myapp"

[dev]
extra_watch = []

[migration]
dir = "migrations"
auto = true

[assets.tailwind]
enabled = true
input = "assets/app.css"
output = "public/generated/app.css"
minify = true
```

| キー | 既定値 | 意味 |
| --- | --- | --- |
| `project.name` | — | 必須 |
| `project.main` | — | 必須。`pw build` と `pw dev` がビルドする main パッケージ |
| `dev.extra_watch` | `[]` | `pw dev` が追加で監視する相対 glob パターン |
| `migration.dir` | `migrations` | プロジェクトからの相対パス |
| `migration.auto` | `true` | `pw dev` 起動時に未適用のマイグレーションを適用する |
| `assets.tailwind.*` | 無効 | [スタイリング](/ja/guides/styling/)を参照 |

上のレイアウトに拡張しても、**ここを変更する必要はありません**。アプリケーションの成長
に伴って実際に触ることになるのは、`pw dev` が既定では監視しないファイルを扱うときの
`dev.extra_watch` と、マイグレーションを自分で実行したいときの `migration.auto` です。
