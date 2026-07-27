---
title: pw init
description: 動く Popcorn Wave プロジェクトを作成する。
sidebar:
  order: 1
---

```sh
pw init <project-name> [--tailwind] [--auth=<mode>] [--devidp]
```

新しいディレクトリに、動作する完全なプロジェクトを作ります。名前とオプションを
渡せば非対話で動き、名前を省略するか `-i` を付けると、同じ選択肢をウィザードで
答えられます。

## オプション

| オプション | 効果 |
| --- | --- |
| `--tailwind` | Tailwind CSS のツールチェインも一緒にスキャフォールドする |
| `--no-tinygo` | TinyGo ではなくホストの Go を対象にする |
| `--auth=<mode>` | `none`（既定）、`oidc`、`oidc-passkey`、`passkey` |
| `--devidp` | OIDC を選んだ場合に、ローカルの認証プロバイダを組み込む |
| `-i`, `--interactive` | 名前を与えた場合でも全項目を質問する |

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
├── popcornwave.toml           プロジェクト名、main パッケージ、dev の監視対象
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
├── queries/users.pw.sql       型付き結果を持つ名前付き SQL
├── migrations/00001_init.sql  初期スキーマ（goose 形式）
├── public/.keep               空ツリーの番兵。配信されない
├── public.go                  public/ を埋め込んで登録する
├── .vscode/settings.json      **/*_pw_gen.go を隠す
└── .gitignore                 *_pw_gen.go などのビルド生成物を除外
```

OIDC 系のモードで `--devidp` を付けると、選択できる開発用ユーザーの一覧である
`devidp.toml` も書き出し、`popcornwave.toml` に `[dev.idp]` を追加します。

`--tailwind` を付けると、さらに `assets/app.css` と `public/generated/app.css` を書き、
`popcornwave.toml` に `[assets.tailwind]` ブロックを追加し、`devbox.json` に
`tailwindcss` をピン留めし、ドキュメントシェルからスタイルシートをリンクします。
`package.json` も Node のロックファイルも作られません。あとから有効にする方法は
[スタイリング](/ja/guides/styling/)を参照してください。

各ファイルは一時パスに書いてから所定の場所へリネームされます。コマンドが中断しても、
書きかけのソースファイルは残りません。

## 実行されるもの

ファイルを書くだけでは、スキャフォールドが使えることの証明になりません。そのため
`pw init` は成功を報告する前に `go mod tidy` と
[`pw generate`](/ja/pw/project/generate/) を実行し、すぐにコンパイルできる
プロジェクトを残します。

```
Created myapp

  cd myapp
  devbox shell
  pw dev
```

生成される `go.mod` は、それを作った `pw` バイナリのバージョンのフレームワークを
require します。`pw` がリリース版ではなく作業コピーからビルドされていた場合は、代わりに
そのチェックアウトを指す `replace` ディレクティブが書かれます。

## 次のステップ

- [はじめる](/ja/start/getting-started/) — 生成物の詳しい解説。
- [プロジェクト構成](/ja/guides/project-structure/) — 1 パッケージを超えて成長させる。
