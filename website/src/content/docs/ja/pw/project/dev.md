---
title: pw dev
description: 開発ループ。サービス起動、生成、マイグレーション、CSS、変更時の再起動。
sidebar:
  order: 3
---

```sh
pw dev
```

日常的に使うコマンドです。引数は取りません。

## 起動時にすること

1. `devbox.json` に宣言された Devbox サービスを起動する
2. [`pw generate`](/ja/pw/project/generate/) を実行する
3. `migration.auto` が `false` でなければ、未適用のマイグレーションを適用する
4. Tailwind が有効なら、スタイルシートをビルドしてウォッチャを起動する
5. `dev.idp.enabled` が `true` なら、開発用の認証プロバイダを起動する
6. `project.main` をビルドして実行する

そのあとは 0.5 秒ごとに監視対象を確認し、変更があれば該当するステップを繰り返します。

## 監視する対象

- プロジェクト自身の Go、`.pw.html`、`.pw.sql` のソース
- マイグレーションディレクトリ
- Tailwind が有効な場合はその入力ファイル
- `popcornwave.toml` の `dev.extra_watch` に一致するもの

`dev.extra_watch` は相対の glob パターンを取ります。絶対パスは拒否されます。

```toml
[dev]
extra_watch = ["config.dev.toml", "assets/**/*.svg"]
```

## Tailwind

開発中のウォッチャは `assets.tailwind.minify` の設定に関わらず**非 minify** で動きます。
minify がループの中で最も遅い部分だからです。ウォッチャが終了しても `pw dev` は動き
続け、入力ファイルを直接監視する方式にフォールバックします。CSS のプロセスが落ちても
サーバーまで巻き込まれることはありません。

`tailwindcss` は `PATH` 上にある必要があります。そのための `devbox shell` です。
[スタイリング](/ja/guides/styling/)を参照。

## マイグレーション

未適用のマイグレーションはアプリケーションの起動前に適用され、マイグレーション
ディレクトリのファイルが変わったときにも適用されます。自分で制御したい場合は無効に
できます。

```toml
[migration]
auto = false
```

## 開発用の認証プロバイダ

`pw dev` はローカルの OpenID Provider を起動できます。本物の IdP を用意する前から
OIDC ログインを試せます。ログインは一覧からユーザーを選ぶだけで、パスワードは
検証しません。だからこそ開発以外では決して動きません。

```toml
[dev.idp]
enabled = true
# config = "devidp.toml"   # ユーザー定義ファイル。プロジェクトからの相対パス
# port = 0                 # 0 なら空いているループバックポートを確保する
```

ユーザー定義ファイルには、選択できるユーザーと付与する claim を書きます。

```toml
[users.admin]
display_name = "Administrator"
extra_scopes = ["admin"]
[users.admin.claims]
email = "admin@example.com"
role = "admin"

[users.guest]
display_name = "Guest User"
[users.guest.claims]
email = "guest@example.com"
```

クライアント登録も issuer URL のコピーも不要です。`pw dev` が実行ごとに使い捨ての
クライアントを登録し、アプリケーションには

- `AUTH_OIDC_ISSUER`
- `AUTH_OIDC_CLIENT_ID`
- `AUTH_OIDC_CLIENT_SECRET`

を環境変数として渡します。環境変数は TOML より優先されるため、プロバイダに関する値を
コミットする設定ファイルに書く必要はありません。自分で export した値はそのまま尊重
されます。クライアントシークレットは実行ごとに生成され、出力には現れません。

ユーザー定義ファイルを編集すると、その場でリロードされます。issuer と、動作中の
アプリケーションが既に持っている資格情報はそのまま有効なので、再起動は不要です。

このプロバイダが実装するのは、S256 PKCE 必須の認可コードフロー、discovery、JWKS、
RS256 の ID Token、UserInfo です。リフレッシュトークン、ログアウト、デバイス
フロー、client credentials、同意画面は意図的にありません。詳細は
[`contrib/devidp`](https://github.com/shibukawa/popcornwave/tree/main/contrib/devidp)
を参照してください。これを import したアプリケーションは `pw build` が拒否します。

テストでは `testutil.WithIdentityProvider` が同じプロバイダを起動し、
`WithLoginUser` でログインするユーザーを事前に指定できます。ブラウザ操作なしで
ログインが完結します。[テスト](/ja/guides/testing/)を参照してください。

## 停止

`Ctrl-C` で実行がキャンセルされ、アプリケーション、Tailwind のウォッチャ、Devbox の
サービスが停止します。アプリケーションが自分でエラー終了した場合、`pw dev` は
`application exited: …` と報告して停止します。起動できないプロセスをループし続ける
ことはありません。
