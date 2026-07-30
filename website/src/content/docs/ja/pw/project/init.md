---
title: pw init
description: 動く Popcorn Wave プロジェクトを作成する。
sidebar:
  order: 1
---

```sh
pw init <project-name> [--tailwind] [--no-tinygo] [--no-devbox] [--no-database] [--db=<engine>] [--no-redis] [--router=<kind>] [--auth=<mode>] [--devidp] [-i]
```

新しいディレクトリに、動作する完全なプロジェクトを作ります。名前とオプションを
渡せば非対話で動き、名前を省略するか `-i` を付けると、同じ選択肢をウィザードで
答えられます。

## オプション

| オプション | 効果 |
| --- | --- |
| `--tailwind` | Tailwind CSS のツールチェインも一緒にスキャフォールドする |
| `--no-tinygo` | TinyGo ではなくホストの Go を対象にする |
| `--no-devbox` | `devbox.json` を作らない。mise、Docker Compose、Nix、Homebrew、Scoop など自分の環境を使う |
| `--no-database` | rdb 設定・マイグレーション・SQL の例を作らない |
| `--db=<engine>` | `sqlite`（既定）、`postgres`、`mysql` |
| `--no-redis` | `devbox.json` に Valkey 開発サーバーを入れない |
| `--router=<kind>` | `registered`（既定）、`discovered`、`both`。[探索型ルーティング](/ja/advanced/discovered-routing/#コマンド)を参照 |
| `--auth=<mode>` | `none`（既定）、`oidc`、`oidc-passkey`、`passkey` |
| `--devidp` | OIDC を選んだ場合に、ローカルの認証プロバイダを組み込む |
| `-i`, `--interactive` | 名前を与えた場合でも全項目を質問する |

`--tailwind`、`--no-devbox`、`--no-database`、`--no-redis`、`--auth` はいずれも、
あとから [`pw add`](/ja/pw/project/add/) で追加できる機能の選択です。断っても失うものは
ありません。ただし 2 つは他に依存します。認証はログインセッションをデータベースに
保存し、Valkey サーバーは Devbox のパッケージです。そのため `--no-database` と
`--auth` の併用は拒否され、`--no-devbox` は Valkey も一緒に落とし、答えても何も
適用されない質問はウィザードに現れません。

`--no-tinygo` だけは `pw add` で後から変えられません。
[ツールチェインを変更する](#ツールチェインを変更する)を参照してください。

## データベースを選ぶ

`--db` は 5 つのことを一度に決めます。`config.dev.toml` の DSN、最初の
マイグレーションを書く方言、`devbox.json` に入る開発サーバー、バイナリが
リンクするドライバ、そして `popcornwave.toml` の `project.database` です。
最後の 1 つは `pw generate` が読み、`.pw.sql` をどのプレースホルダ構文に
コンパイルするかを決めます。既定が SQLite なのは、アプリケーションのほかに
起動するものが何もないからです。

| エンジン | DSN | 開発サーバー |
| --- | --- | --- |
| `sqlite` | `sqlite://<name>.db` | なし |
| `postgres` | `postgres://<name>:<name>@127.0.0.1:5432/<name>?sslmode=disable` | `devbox.json` の `postgresql` |
| `mysql` | `mysql://<name>:<name>@tcp(127.0.0.1:3306)/<name>` | `devbox.json` の `mysql80` |

サーバーエンジンを選ぶと `main.go` にブランクインポートが 1 行入ります。これが
エンジンを登録します。

```go
import _ "github.com/shibukawa/popcornwave/database/postgres"
```

スキャフォールドが書く資格情報は `config.dev.toml` の開発用の値です。そこに書かれた
ロールとデータベースを一度だけ作ってから `pw migrate up` を実行してください。

あとからエンジンを変えることは `pw add` では行いません。DSN もマイグレーションも
`.pw.sql` もすべて書き直しになるからです。デプロイ先のエンジンを選んでください。

### 生成される SQL はエンジンに従う

`project.database` は、生成のためにプロジェクトがエンジンを表明する唯一の場所です。
ジェネレータ側に暗黙の既定はありません。黙って仮定した方言は、エンジンが最初の
クエリで拒否するプレースホルダを出力してしまうからです。`pw generate` は
`popcornwave.toml` に書かれたものだけを渡します。

```toml
[project]
database = "postgres"   # sqlite、postgres、mysql
```

| エンジン | プレースホルダ |
| --- | --- |
| `postgres` | `$1`、`$2`、… |
| `mysql` | `?` |
| `sqlite` | `?` |

このキーを変えると生成済みのクエリがすべて変わります。編集したら `pw generate` を
実行し、DSN の変更と一緒にコミットしてください。

このキーができる前に作られたプロジェクトには `[project] database` がありませんが、
`sqlite` として読まれます。当時存在したエンジンがそれだけだからです。

## ツールチェインを変更する

選んだコンパイラは `popcornwave.toml` の `project.toolchain` に記録され、ハンドラ
パッケージが使う mux の型を決めます。TinyGo プロジェクトは両方のツールチェインで同じ
import が通るよう `pw.ServeMux` を経由し、ホスト専用のプロジェクトは `http.ServeMux`
のままです。生成はどちらも検出するので、違いはスキャフォールドの中に閉じています。

あとから切り替えるコマンドはありません。変更があなたの所有するソースに及ぶからです。
手作業で行う場合は 4 か所です。

1. `popcornwave.toml` の `project.toolchain` を `tinygo` か `go` にする
2. 各ハンドラパッケージの `index.go` で mux の型とアクセサを差し替える
3. `devbox.json` の `tinygo@latest` を足すか外す
4. TinyGo 専用の netdev 登録 `tinygohelper.go` を足すか外す。これがないと TinyGo
   バイナリは起動時に `Netdev not set` で落ちます

そのあと [`pw generate`](/ja/pw/project/generate/) を実行します。`project.toolchain`
に `tinygo` と `go` 以外の値を書くと、プロジェクトの読み込み時に拒否されます。

## 認証

認証は複数の設定に影響するため、選んだモードが `config.dev.toml` に書かれる
`[auth]` セクションを決めます。

| 回答 | `auth.mode` | 意味 |
| --- | --- | --- |
| None | — | `[auth]` セクションを書かない |
| OIDC | `oidc` | ログインは常に OpenID Provider を経由する |
| OIDC + passkey | `oidc_passkey` | OIDC でアカウントを作り、以降はパスキーでログイン |
| Passkey only | `passkey_only` | 外部プロバイダなし。リカバリ方針は自分で決める |

OIDC 系を選ぶと、**ローカルエミュレータ**か**外部プロバイダ**かを1つだけ追加で質問
します。

ローカルエミュレータは [`pw dev`](/ja/pw/project/dev/) が起動する開発用認証プロバイダ
です。`pw init` は `popcornwave.toml` に `dev.idp.enabled` を設定し、初期ユーザー2人分の
`devidp.toml` を書き出します。issuer とクライアント資格情報は `pw dev` が注入するので、
コミットする設定ファイルにプロバイダの情報は一切書かれません。

外部プロバイダの場合、`config.dev.toml` には空の `issuer`、`client_id`、
`client_secret` が書かれます。これらはプレースホルダであり、**省略可能な設定では
ありません**。ファイルまたは `AUTH_OIDC_*` 環境変数から値を与えるまで、
アプリケーションは起動を拒否します。残る選択肢はエミュレータへの切り替えです。

## 検証

プロジェクト名に使えるのは英数字、`-`、`_` です。`.` と `..` は拒否されます。作成先は
空であるか存在しないかのどちらかでなければなりません。既存ツリーでうっかり
`pw init .` を実行してもファイルが散らばることはなく、エラーで止まります。

## 書き出されるもの

```
myapp/
├── popcornwave.toml           プロジェクト名、main パッケージ、生成対象ディレクトリ
├── config.dev.toml            APP_ENV=dev のランタイム設定
├── go.mod
├── devbox.json / devbox.lock  Go + Valkey（--tailwind なら tailwindcss も）
├── cmd/myapp/main.go          pw.Run を呼ぶ
├── handlers/
│   ├── index.go               パッケージレベルの mux と Handlers()
│   ├── home_handler.go        ルート登録と net/http ハンドラ
│   └── home.pw.html           型付きページテンプレート
├── templates/
│   ├── document.pw.html       共有ドキュメントシェル
│   ├── templates.go           初回生成前から存在するパッケージマーカー
│   └── 400|404|500.pw.html    エラーページ
├── queries/users.pw.sql       型付き結果を持つ名前付き SQL（データベース選択時）
├── migrations/00001_init.sql  初期スキーマ、goose 形式（データベース選択時）
├── public/.keep               空ツリーの番兵。配信されない
├── public.go                  public/ を埋め込んで登録する
├── .vscode/settings.json      **/*_pw_gen.go を隠す
└── .gitignore                 *_pw_gen.go などのビルド生成物を除外
```

`popcornwave.toml` には、いま作ったディレクトリが `[generate]` の各用途に振り分けて
書かれます。[`pw generate`](/ja/pw/project/generate/) はこれらのリストだけを読み、
既定値を持たないためです。スターターのページテンプレートはハンドラの隣にあるので
`handlers` は `handlers` と `templates` の両方に現れ、アプリケーションが設定を登録する
`cmd/myapp` は `config` に現れます。

OIDC 系のモードで `--devidp` を付けると、選択できる開発用ユーザーの一覧である
`devidp.toml` も書き出し、`popcornwave.toml` に `[dev.idp]` を追加します。

`--tailwind` を付けると、さらに `assets/app.css` と `public/generated/app.css` を書き、
`popcornwave.toml` に `[assets.tailwind]` ブロックを追加し、`devbox.json` に
`tailwindcss` をピン留めし、ドキュメントシェルからスタイルシートをリンクします。
`package.json` も Node のロックファイルも作られません。あとから有効にする方法は
[スタイリング](/ja/guides/frontend/styling/)を参照してください。

各ファイルは一時パスに書いてから所定の場所へリネームされます。コマンドが中断しても、
書きかけのソースファイルは残りません。

## 実行されるもの

ファイルを書くだけでは、スキャフォールドが使えることの証明になりません。そのため
`pw init` は成功を報告する前に `go mod tidy` と
[`pw generate`](/ja/pw/project/generate/) を実行し、すぐにコンパイルできる
プロジェクトを残します。

```
Created myapp

Not included: redis-valkey, auth, tailwind
  pw add <capability> enables one later

  cd myapp
  devbox shell
  pw dev
```

この案内には、その実行で断ったものだけが並びます。非対話で `pw init` を回した場合
でも、ウィザードが各選択肢の横に書いているのと同じことが分かります。

Devbox を断つと、入るシェルが無いので `devbox shell` の行も消えます。その場合
Tailwind のツールチェインもピン留めされないため、何を入れるべきかを表示します。

```
Tailwind CSS needs its own toolchain here:
  install the standalone tailwindcss CLI, version 4 or later
```

Devbox のパッケージ名ではなく要件を書きます。`tailwindcss_4@4.1.18` は nixpkgs の
識別子であり、mise や Homebrew、Scoop を使う人には何も伝えないからです。バイナリが
見つからないときの [`pw build`](/ja/pw/project/build/) も同じ内容を報告します。

生成される `go.mod` は、それを作った `pw` バイナリのバージョンのフレームワークを
require します。`pw` がリリース版ではなく作業コピーからビルドされていた場合は、代わりに
そのチェックアウトを指す `replace` ディレクティブが書かれます。

## 次のステップ

- [はじめる](/ja/start/getting-started/) — 生成物の詳しい解説。
- [pw add](/ja/pw/project/add/) — ここで断った機能をあとから追加する。
- [pw new](/ja/pw/project/new/) — 2 つめのハンドラを追加する。
- [プロジェクト構成](/ja/guides/architecture/project-structure/) — 1 パッケージを超えて成長させる。
