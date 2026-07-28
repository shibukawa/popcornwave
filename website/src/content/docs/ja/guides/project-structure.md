---
title: プロジェクト構成
description: 単一の handlers パッケージを超えて成長させる。ハンドラとクエリの階層化、popcornwave.toml が制御するもの。
sidebar:
  order: 5
---

`pw init` は `handlers` パッケージと `queries` パッケージを 1 つずつ作ります。この
レイアウトは理解しやすい一方、アプリケーションが成長すると、利用者も所有者も異なる
領域が生まれます。そこで問題になるのは、フレームワーク側に別のレジストリを増やさずに
どう分割するかです。

## 生成が読むもの

`pw generate` の範囲は用途ごとです。生成されるコードの種類ごとに、読んでよい
ディレクトリを明示し、それ以外は読みません。

```toml
[generate]
handlers = ["handlers"]
templates = ["handlers", "templates"]
queries = ["queries"]
config = ["cmd/myapp"]
```

`handlers` が 2 回現れるのは意図的です。ページテンプレートはそれを描画するハンドラの
隣に置くので、このディレクトリは両方の用途を担います。こう分けることで、config の
用途だけが `cmd/myapp` に届き、handler の用途はそこを走査しません。各用途が何を読み
何を生成するかは [`pw generate`](/ja/pw/project/generate/) を参照してください。

どの用途にも既定値はありません。キーの書き忘れはエラーで、その用途が何も生成しない
ことは `[]` で表します。生成が何を読むのかは、走査規則から推測するものではなく
1 行読めば分かるものです。

列挙したディレクトリの中で入れ子にするのは自由です。`webroot/admin/queries` は
`webroot` のエントリに含まれます。編集が必要になるのは、**トップレベル**のソース
ディレクトリを増やしたときです。

自分の用途の外にあるソースはビルドを失敗させず、報告してスキップします。意図して
置いたサンプルやフィクスチャがコードの隣にあっても構わないためです。

```
pw: samples/home.pw.html is outside generate.templates and is not generated from; list its directory to include it
```

## 大きめのレイアウト

対象読者が分かれてきたアプリケーションでは、次のようなレイアウトがよく使われます。

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

この合成を成立させるのは依存の向きです。親が子をインポートするので、Go は子の
`init`、つまりルート登録を親の `init` より先に実行します。子から親をインポート
しないため、パッケージ間の循環も起きません。

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

[generate]
handlers = ["handlers"]
templates = ["handlers", "templates"]
queries = ["queries"]
config = ["cmd/myapp"]

[dev.watch]
includes = []
excludes = []

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
| `generate.handlers` | — | 必須。ルートとバインディングのために読むディレクトリ |
| `generate.templates` | — | 必須。`.pw.html` のために読むディレクトリ |
| `generate.queries` | — | 必須。`.pw.sql` のために読むディレクトリ |
| `generate.config` | — | 必須。設定登録のために読むディレクトリ |
| `dev.watch.includes` | `[]` | `pw dev` が追加で監視する相対 glob パターン |
| `dev.watch.excludes` | `[]` | `pw dev` が走査時にスキップするサブツリー |
| `migration.dir` | `migrations` | プロジェクトからの相対パス |
| `migration.auto` | `true` | `pw dev` 起動時に未適用のマイグレーションを適用する |
| `assets.tailwind.*` | 無効 | [スタイリング](/ja/guides/styling/)を参照 |

上のレイアウトへ拡張するときに必要な編集は 1 か所です。`handlers` と `templates` の
用途で `handlers` を `webroot` に置き換えます。その下に入れ子になったエリアは自動的に
含まれます。それ以外で成長に伴って変更する可能性が高いのは、走査が見つけられない
編集対象を加える `dev.watch.includes`、大きな依存ツリーが走査をループ中で最も遅い
ステップにしてしまうときの `dev.watch.excludes`、そしてマイグレーションを自分で
管理する場合の `migration.auto` です。

`pw dev` は生成よりも意図的に広く監視します。どの用途も生成に使わないファイルを
含め、Go のソースはすべて再ビルドの入力だからです。だからこそ範囲は includes で
宣言するのではなく excludes で削ります。
