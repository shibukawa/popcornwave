---
title: 開発用の認証プロバイダ
description: 本物の IdP を用意する前に、一覧から選ぶだけのユーザーで本物の OIDC フローを通す。
sidebar:
  order: 3
---

[`pw dev`](/ja/pw/project/dev/) はローカルの OpenID Provider を起動できます。本物の IdP を用意する前から
OIDC ログインを試せます。ログインは一覧からユーザーを選ぶだけで、パスワードは
検証しません。だからこそ開発以外では決して動きません。

![Administrator と Member のアカウントが並び、パスワードを検証しないことを示す開発用 IdP のログイン画面](../../../../assets/screenshots/tutorial-login.png)

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

クライアント登録も issuer URL のコピーも不要です。`pw dev` が実行ごとに一時的な
クライアントを作り、アプリケーションには

- `AUTH_OIDC_ISSUER`
- `AUTH_OIDC_CLIENT_ID`
- `AUTH_OIDC_CLIENT_SECRET`

を環境変数として渡します。環境変数は TOML より優先されるため、プロバイダの資格情報を
コミットする設定ファイルに入れる必要はありません。自分で export した値は維持され、
生成されるクライアントシークレットは実行ごとに変わり、出力には現れません。

ユーザー定義ファイルを編集すると、その場でリロードされます。issuer と、動作中の
アプリケーションが既に持っている資格情報はそのまま有効なので、再起動は不要です。

このプロバイダが実装するのは、S256 PKCE 必須の認可コードフロー、discovery、JWKS、
RS256 の ID Token、UserInfo です。リフレッシュトークン、ログアウト、デバイス
フロー、client credentials、同意画面は意図的にありません。これを import した
アプリケーションは `pw build` が拒否します。詳細は
[`contrib/devidp`](https://github.com/shibukawa/popcornwave/tree/main/contrib/devidp)
を参照してください。

テストでは `testutil.WithIdentityProvider` が同じプロバイダを起動し、
`WithLoginUser` でログインするユーザーを事前に指定できます。ブラウザ操作なしで
ログインが完結します。[テスト](/ja/productivity/testing/)を参照してください。
