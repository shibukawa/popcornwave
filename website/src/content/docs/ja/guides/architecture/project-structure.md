---
title: プロジェクト構成
description: 単一の handlers パッケージを超えて成長させる。ハンドラとクエリの階層化、popcornwave.toml が制御するもの。
sidebar:
  order: 1
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
運用エンドポイントの衝突は起動時に報告されます。別の場所にマウントするか、アセットのマウント位置を移動してください。
[静的ファイル配信](/ja/guides/frontend/static-assets/)を参照してください。
:::

## 分割されないもの

パッケージをいくら増やしても、次の 3 つは分割されません。

**ドキュメントシェル。** プロジェクトに `document.pw.html` はちょうど 1 つで、ツリーの
どこかに 2 つ以上あると生成エラーになります。エリアごとに別のシェルを使いたい場合は、
名前なしスロットを持つ通常のエクスポート済みコンポーネントを書き、ハンドラごとに
`pw.WriteHTMLChain` で選択してください。[テンプレート](/ja/guides/frontend/templates/)を参照。

**マイグレーション。** アプリケーション全体で 1 つの順序付き集合が `migration.dir` に
あります。

**設定の prefix。** 各エリアが自分の設定構造体を登録できますが、prefix は 1 つの名前空間
を共有します。[設定](/ja/guides/architecture/configuration/)を参照。

## `popcornwave.toml`

プロジェクトファイルは小さく、そのキーは**閉じた集合**です。未知のキーは警告ではなく
エラーになります。

```toml
[project]
name = "myapp"
main = "./cmd/myapp"
toolchain = "tinygo"
database = "sqlite"

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
| `project.toolchain` | `tinygo` | スキャフォールド時に選んだコンパイラ。[pw init](/ja/pw/project/init/#ツールチェインを変更する) を参照 |
| `project.database` | `sqlite` | `.pw.sql` を生成する対象エンジン。`sqlite`、`postgres`、`mysql`。[データベースを選ぶ](/ja/pw/project/init/#データベースを選ぶ) を参照 |
| `generate.handlers` | — | 必須。ルートとバインディングのために読むディレクトリ |
| `generate.templates` | — | 必須。`.pw.html` のために読むディレクトリ |
| `generate.queries` | — | 必須。`.pw.sql` のために読むディレクトリ |
| `generate.config` | — | 必須。設定登録のために読むディレクトリ |
| `dev.watch.includes` | `[]` | `pw dev` が追加で監視する相対 glob パターン |
| `dev.watch.excludes` | `[]` | `pw dev` が走査時にスキップするサブツリー |
| `migration.dir` | `migrations` | プロジェクトからの相対パス |
| `migration.auto` | `true` | `pw dev` 起動時に未適用のマイグレーションを適用する |
| `assets.tailwind.*` | 無効 | [スタイリング](/ja/guides/frontend/styling/)を参照 |

上のレイアウトへ拡張するときに必要な編集は 1 か所です。`handlers` と `templates` の
用途で `handlers` を `webroot` に置き換えます。その下に入れ子になったエリアは自動的に
含まれます。

成長に伴って動かすことになるキーは、ほかに 3 つです。`dev.watch.includes` は、走査が
取りこぼす編集対象を拾います。`dev.watch.excludes` は、大きな依存ツリーが走査をループ中で
最も遅いステップにしてしまうときに範囲を削ります。`migration.auto` は、マイグレーションを
自分で管理したくなったときに切ります。

`pw dev` は生成よりも意図的に広く監視します。どの用途も生成に使わないファイルを
含め、Go のソースはすべて再ビルドの入力だからです。だからこそ範囲は includes で
宣言するのではなく excludes で削ります。

## Popcorn Wave が想定するアーキテクチャ

パッケージを増やせば、分離が強くなるとは限りません。`net/http` と `database/sql` の上に
作る Go アプリケーションには、コンパイラ、ツール、ライブラリ、ほかの Go 開発者が共通して
理解できる強い境界が、すでにあります。その一つひとつを controller、use case、repository、
ローカルな interface で包んでも、プログラムの動作を変えない境界が重複するだけかもしれません。

Clean Architecture は、実在する依存の向きを反転させるときや、所有者の異なるコードを変更から
守るときには有効です。Popcorn Wave が採らないのは、異なる知識を持つかどうかに関係なく、
すべてのアプリケーションへ同じ数の円を当てはめる使い方です。レイヤーには、置く理由が要ります。

### ドメイン知識を、データのある場所へ置く

20年以上前のエンタープライズ Java から受け継がれた設計技法の一部は、データベースを触れれば
純粋さが失われるインフラとして扱い、ドメインをメモリ上のロジックへ隔離します。当時の前提が
変わったあとも、その古い図は規範として再生産されています。

データベースの重要性は変わりません。スキーマ、キー、制約、リレーション、インデックス、
クエリ、トランザクション境界は、アプリケーションが何を許し、何を効率よく実行できるかを
表します。トランザクションの境界を動かせば、原子性、並行実行、失敗時の振る舞いが変わる。
それはストレージの詳細ではなく、ドメイン上の結果です。

こうした性質を汎用的な CRUD repository の後ろへ隠すと、大切な選択まで見えにくくなります。
行を1件ずつ取得する、メモリ上で join する、不要な列まで読む、実際の処理ではなくレイヤーの
対称性に合わせてトランザクションを開く。純粋に見えるコードが、データベースの仕事を増やし、
保証を弱めることがあります。

そのため Popcorn Wave は、ドメイン知識がスキーマと SQL まで届く設計を期待します。Go の
アプリケーションコードは、リクエスト、外部システム、SQL だけでは明瞭に表現しきれない規則を
組み立てます。データモデルをドメインの外へ追い出す層ではありません。生成されたクエリが SQL
を見えるままにし、トランザクションの所有権をアプリケーションコードへ明示的に残すのは、
そのためです。

### layer ではなく feature でパッケージを分ける

トップレベルを `handlers`、`controllers`、`services`、`repositories`、`models` に分けると、
ひとつの変更が一般名のパッケージへ散らばります。同じような名前がエディタのタブやコード補完に
繰り返し現れるという問題もあります。境界を越えるたびに request 用、domain 用、永続化用の
構造体と、それらを詰め替える mapper が増えていく。変換コードが新しい知識をほとんど持たない
一方で、ソースの意味密度は下がり、コード量と場合によってはバイナリも膨らみます。レビュー時間、
人間の注意、AI が消費するコンテキストにも同じコストがかかります。

現在の Java アプリケーションが、一様に package by layer で構成されているわけでもありません。
それでも20年前の本にある図を、当時の制約に対する設計ではなく必須のプロジェクト構成として
読むことで、同じレイヤー分割が繰り返し持ち込まれています。

Russ Cox も、「標準的な Go のレイアウト」を名乗るリポジトリへの異議で、そこで示された構成は
非常に複雑であり、Go のリポジトリは一般にもっと単純だと指摘しています
（[this is not a standard Go project layout](https://github.com/golang-standards/project-layout/issues/117)）。
Popcorn Wave は、この単純さを Go の重要な性質として扱います。

したがって、小さなアプリケーションの最小構成は、layer を重ねず浅く保ちます。`pw init` が
作るのは1つの handler 領域と1つの query パッケージです。機能領域がまだ1つしかない段階で、
controller、service、repository の段を先に作ることはしません。ひとつの機能を理解するために
リポジトリ全体を巡らなくて済むよう、ハンドラ、そのテンプレート、クエリコードを短いパスの
中に置きます。規模が大きくなったら、上の `webroot` の例のように `admin`、`accounts`、
`billing` といった feature 単位でパッケージを分けます。feature の内部は浅いままにし、親の
mux が各パッケージを合成します。共有パッケージへ持ち上げるのは、複数の feature が実際に
共有するようになってからです。

すべてのレイヤーが人工的なわけではありません。設定の入力、外部からの HTTP リクエスト、SQL、
HTML は、表現が実際に切り替わる境界です。Popcorn Wave がそこでコード生成を提供するのは、
変換によって型とプロトコルを検査できるからです。空の円を維持するためだけの
controller-to-service-to-repository の接着コードや、構造体 mapper を生成する機能は提供しません。

目標は、パッケージ数を最小にすることではありません。異なる知識を持たないレイヤーを増やさない
ことです。Go の標準 interface を見えるままにし、データベースの振る舞いを明示し、feature ごとの
まとまりを保つ。所有権や feature の境界が実在するようになったときにパッケージを足します。
図を先取りするためには足しません。
