---
title: 認証
description: OIDC を設定すると、ログイン・コールバック・ログアウトはフレームワークが提供する。
sidebar:
  order: 10
---

Popcorn Wave は認証エンドポイントを自分で提供します。プロバイダを設定すれば、
ログイン、コールバック、ログアウトがマウントされ、リクエストごとにセッションが解決
され、ハンドラは認証済みの識別子を受け取ります。登録するルートも、書く OIDC の
コードもありません。

## 有効にする

必要なのは2つだけです。

```go
// cmd/myapp/main.go — アカウントリゾルバの登録が plugin/auth の import を
// 兼ねます。エンドポイントとセッション解決はその拡張が担当します。
func main() {
	handlers.RegisterAccountResolver()
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

```toml
# config.dev.toml
[session]
enabled = true
backend = "rdb"          # セッションは不透明でサーバー側に保存される

[auth]
enabled = true
mode = "oidc_only"

[auth.oidc]
issuer = "https://issuer.example"
client_id = "..."
client_secret = "..."
redirect_url = "https://app.example/auth/callback"
identity_claim = "sub"   # アカウントを識別する検証済み claim
provider_logout = true   # プロバイダ側もサインアウトする
```

`pw init --auth=oidc` は両方に加えて、フレームワークのテーブルを作るマイグレーションも
書き出します。起動時にテーブルの存在を検証し、足りなければ適用すべきマイグレーションを
名指しで知らせます。

## エンドポイント

| パス | メソッド | 動作 |
| --- | --- | --- |
| `auth.login_path`（`/auth/login`） | GET | プロバイダへリダイレクトする |
| `auth.callback_path`（`/auth/callback`） | GET | 結果を検証してセッションを開始する |
| `auth.logout_path`（`/auth/logout`） | POST | セッションを終了する |

`auth.protection.include` に列挙したパスだけがセッションを必要とし、それ以外は公開の
ままです。未認証のリクエストはログインを経由して元のパスに戻され、
`auth.protection.unauthenticated = "unauthorized"` なら `401` を返します。

**ログアウトは POST のみです。** リンクやブラウザのプリフェッチで発火するログアウトは
利便性ではなく DoS の入口なので、`GET` には `405` を返します。サインアウトの操作は
フォームになります。

```html
<form method="post" action={logoutPath}>
  <button type="submit">Sign out</button>
</form>
```

クロスオリジンのログアウトは拒否されます。セッションクッキーは `SameSite=Lax` で、
さらにエンドポイント側でも `Origin` の不一致を弾きます。

ログアウトは**プロバイダ側のセッションも終了**させます。ローカルの cookie を消すだけ
では、プロバイダにはログインしたままなので、次のログインで同じアカウントが無確認で
返ってきて、サインアウトが何もしなかったように見えます。そのためこのエンドポイントは
`client_id` と、このオリジンを指す `post_logout_redirect_uri` を付けて、プロバイダの
RP-initiated logout を経由します。

```
POST /auth/logout
  → 303 https://issuer.example/end_session?client_id=…&post_logout_redirect_uri=…
  → 302 ログアウト後のページへ
```

ローカルだけのログアウトに留めたい場合は `auth.oidc.provider_logout = false` に
します（他のアプリケーションと共有するプロバイダで、そちらはログインを維持したい
場合など）。`end_session_endpoint` を公開していないプロバイダでは自動的にローカル
ログアウトにフォールバックします。

ログイン後は `auth.post_login_path`、または元々アクセスしようとしていたパスに着地し
ます。受け付けるのは同一サイトのルート相対パスだけなので、ログインリンクを
オープンリダイレクトに仕立てることはできません。

## ユーザーを読む

セッションはハンドラが動く前に解決されています。

```go
func home(w http.ResponseWriter, r *http.Request) {
	user, signedIn := auth.User(r.Context())
	if signedIn {
		// user.AccountID, user.DisplayName, user.Email, user.Issuer, user.Key
	}
	// ...
}
```

検証済み identity に対応するアカウントを決めるのはアプリケーションです。フレーム
ワークは `auth.SetAccountResolver` で登録した関数を呼び、`auth.oidc.auto_provision` が
許す場合はそこで新規作成もできます。リンクのキーは issuer と
`auth.oidc.identity_claim` が指す claim であって、メールアドレスではありません。

期限切れや不明なセッションクッキーは黙って捨てられます。匿名のリクエストは異常では
なく通常の状態なので、`auth.User` は単に `false` を返します。答えるのは「誰か」だけで、
「何をしてよいか」ではありません。認可はアプリケーションの責任のままです。

## モード

| `auth.mode` | 状態 |
| --- | --- |
| `oidc_only` | 実装済み |
| `oidc_passkey`、`passkey_only` | 未実装。起動時に拒否される |

## セッション

クッキーが運ぶのは不透明なトークンだけで、セッション本体は `plugin/session/rdb` が
データベースに保存します。だからサーバー側で失効させられます。`session.ttl` が絶対
有効期限、`session.idle_timeout` が無操作期限です。ログイン時にトークンは新しくなり、
それ以前にブラウザが持っていたセッションは失効します。

保存されるのはアカウントの要約だけで、トークン本体は含みません。プロバイダの
アクセストークンや ID Token がセッションに残ることはありません。

## 開発中

ログインを作るのに本物のプロバイダは要りません。`pw dev` が用意できます。

```toml
# popcornwave.toml
[dev.idp]
enabled = true
```

[開発用の認証プロバイダ](/ja/pw/project/dev/#開発用の認証プロバイダ)を起動し、その実行
専用のクライアントを登録し、`AUTH_OIDC_ISSUER`、`AUTH_OIDC_CLIENT_ID`、
`AUTH_OIDC_CLIENT_SECRET` を注入します。この方式でスキャフォールドしたプロジェクトの
コミット対象ファイルには、プロバイダの値が一切現れません。ログインは一覧から
ユーザーを選ぶだけで、パスワードは検証しません。だからこそ開発以外では動きません。

テストでは `testutil.WithIdentityProvider` が同じプロバイダを起動し、`WithLoginUser`
でユーザーを事前指定できます。`auth.login_path` への1リクエストでフロー全体が完了
します。[テスト](/ja/guides/testing/#withidentityprovider)を参照してください。

## デプロイ

`issuer`、`client_id`、`client_secret` は空であってはならず、空のままだとアプリケー
ションは起動を拒否し、足りないキーと対応する `AUTH_OIDC_*` 環境変数を示します。
`SESSION_SECRET` と合わせて、コミットするファイルではなく環境から与えてください。

`auth.oidc.redirect_url` は空のままでもかまいません。その場合コールバック URL は
リクエストのオリジンに追従します。ブラウザから見えるオリジンとアプリケーションが
認識するオリジンが異なる場合は明示的に設定し、同じ値をプロバイダにも登録してください。

ログアウト後の URL もプロバイダに登録してください。未登録の `post_logout_redirect_uri`
は拒否されます。値は公開オリジン上の `auth.post_logout_redirect` で、既定の `/` なら
`https://app.example/` です。

開発用プロバイダだけは例外で、ローカル宛てのログアウト後 URL を登録なしで受け付け
ます。`pw dev` ではどこにも登録しないままログアウトが動きます。
