---
title: pw init
description: 動く Popcorn Wave プロジェクトを作成する。
sidebar:
  order: 1
---

```sh
pw init <project-name> [--tailwind] [--auth=<mode>] [--devidp]
```

プロジェクト名のディレクトリを新規に作り、その中に動作する完全なプロジェクトを作成
します。名前を省略するか `-i` を付けると、同じ項目をウィザードで対話的に答えられます。

## オプション

| オプション | 効果 |
| --- | --- |
| `--tailwind` | Tailwind CSS のツールチェインも一緒にスキャフォールドする |
| `--no-tinygo` | TinyGo ではなくホストの Go を対象にする |
| `--auth=<mode>` | `none`（既定）、`oidc`、`oidc-passkey`、`passkey` |
| `--devidp` | OIDC を選んだ場合に、ローカルの認証プロバイダを組み込む |
| `-i`, `--interactive` | 名前を与えた場合でも全項目を質問する |

## 認証

認証の質問は `config.dev.toml` の `[auth]` セクションの内容を決めます。

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

外部プロバイダの場合、`config.dev.toml` の `issuer`、`client_id`、`client_secret` は
空文字列で書き出されます。**これらは省略可能ではありません**。いずれかが空のままだと
アプリケーションは起動を拒否し、足りないキーと対応する `AUTH_OIDC_*` 環境変数を示し
ます。値を書くか、環境変数を設定するか、エミュレータに切り替えてください。

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

ファイルはアトミックに書かれます。それぞれ一時ファイルに書いてからリネームするので、
中断してもソースファイルが半端な状態で残ることはありません。

## 実行されるもの

ファイルを書き出したあと、`pw init` は `go mod tidy` と
[`pw generate`](/ja/pw/project/generate/) を実行するので、プロジェクトはすぐに
コンパイルできる状態になります。最後に次の内容を表示します。

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
