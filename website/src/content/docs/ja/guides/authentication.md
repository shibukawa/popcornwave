---
title: 認証
description: OIDC を設定すると、ログイン・コールバック・ログアウトはフレームワークが提供する。
sidebar:
  order: 9
---

OIDC には通常、3 つのルート、セッション解決、そして一連のプロトコルコードが伴います。
Popcorn Wave はその仕組みを引き受けます。プロバイダを設定すれば、ログイン、
コールバック、ログアウトをマウントし、リクエストごとにセッションを解決して、
ハンドラへ identity を渡します。アプリケーションはルートも OIDC コールバックも
登録しません。

## 有効にする

必要なのは2つだけです。

```go
// cmd/myapp/main.go — アカウントリゾルバの登録が plugin/auth の import を
// 兼ねます。エンドポイントとセッション解決はその拡張が担当します。
import _ "github.com/shibukawa/popcornwave/plugin/session/rdb" // session.backend = "rdb"

func main() {
	handlers.RegisterAccountResolver()
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

ストレージの import が `plugin/auth` と別なのは意図的です。auth プラグインは
バックエンドを一切リンクしないので、アプリケーションは `session.backend` で選んだ
ものだけを持ちます。`pw init --auth=oidc` は両方の行を書き出します。

```toml
# config.dev.toml
[session]
enabled = true
backend = "rdb"          # "cookie" と "redis" も選べる。トークンが不透明なのはどれも同じ

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

**ログアウトは POST のみです。** リンクやブラウザのプリフェッチでセッションを
終了できると、サインアウトが DoS の入口になります。そのため `GET` には `405` を返し、
操作にはフォームを使います。

```html
<form method="post" action={logoutPath}>
  <button type="submit">Sign out</button>
</form>
```

クロスオリジンのログアウトは拒否されます。セッションクッキーは `SameSite=Lax` で、
さらにエンドポイント側でも `Origin` の不一致を弾きます。

既定では、ログアウトは**プロバイダ側のセッションも終了**させます。ローカルの cookie
だけを消すと、上流ではログインしたままです。次のログインで同じアカウントが即座に
返され、サインアウトが効かなかったように見えます。そのためこのエンドポイントは
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

検証済み identity がアプリケーションのアカウントモデルまで決めるわけではありません。
フレームワークは `auth.SetAccountResolver` で登録した関数を呼び、アカウントを検索し、
`auth.oidc.auto_provision` が許す場合は新規作成も行います。安定したリンクは issuer と
`auth.oidc.identity_claim` が指す claim の組み合わせであり、メールアドレスでは
ありません。

期限切れや不明なセッションクッキーは黙って捨てられます。匿名リクエストは通常の状態
なので、`auth.User` は単に `false` を返します。ユーザーを返す場合も、答えるのは
「誰か」であって「その人が何をしてよいか」ではありません。認可はアプリケーションに
残ります。

## モード

| `auth.mode` | 状態 |
| --- | --- |
| `oidc_only` | 実装済み |
| `oidc_passkey`、`passkey_only` | 未実装。起動時に拒否される |

## セッション

クッキーが運ぶのは不透明なトークンだけで、セッション本体がどこに住むかは
`session.backend` が決めます。既定の `rdb` は `plugin/session/rdb` がデータベースに、
`redis` は Redis または Valkey にサーバー側 TTL 付きで保存し、`cookie` はセッション用
ストレージを持たないデプロイのために2つ目のクッキーに封をします（違いは
[クッキー](/ja/guides/cookies/)を参照）。`session.ttl` が絶対有効期限、
`session.idle_timeout` が無操作期限です。ログイン時にトークンは新しくなり、それ以前に
ブラウザが持っていたセッションは失効します——ただし cookie バックエンドだけは、
クライアントがすでに取ったコピーを失効させられません。

保存されるのはアカウントの要約だけで、トークン本体は含みません。プロバイダの
アクセストークンや ID Token がセッションに残ることはありません。

## 開発中

ローカル開発でログインフローを試すために、本物のプロバイダを必須にする必要はありません。
代わりに `pw dev` が開発用プロバイダを起動できます。

```toml
# popcornwave.toml
[dev.idp]
enabled = true
```

[開発用の認証プロバイダ](/ja/productivity/dev-identity-provider/)を起動し、その実行
専用のクライアントを登録し、`AUTH_OIDC_ISSUER`、`AUTH_OIDC_CLIENT_ID`、
`AUTH_OIDC_CLIENT_SECRET` を注入します。この方式でスキャフォールドしたプロジェクトの
コミット対象ファイルには、プロバイダの値が一切現れません。ログインは一覧から
ユーザーを選ぶだけで、パスワードは検証しません。だからこそ開発以外では動きません。

テストでは `testutil.WithIdentityProvider` が同じプロバイダを起動し、`WithLoginUser`
でユーザーを事前指定できます。`auth.login_path` への1リクエストでフロー全体が完了
します。[テスト](/ja/productivity/testing/#withidentityprovider)を参照してください。

## デプロイ

デプロイ時には、その利便性は外れます。`issuer`、`client_id`、`client_secret` は
空であってはならず、不足しているとアプリケーションは起動を拒否し、キーと対応する
`AUTH_OIDC_*` 環境変数を示します。`SESSION_SECRET` と合わせて、コミットするファイル
ではなく環境から与えてください。

`auth.oidc.redirect_url` は空のままでもかまいません。その場合コールバック URL は
リクエストのオリジンに追従します。ブラウザから見えるオリジンとアプリケーションが
認識するオリジンが異なる場合は明示的に設定し、同じ値をプロバイダにも登録してください。

ログアウト後の URL もプロバイダに登録してください。未登録の `post_logout_redirect_uri`
は拒否されます。値は公開オリジン上の `auth.post_logout_redirect` で、既定の `/` なら
`https://app.example/` です。

開発用プロバイダだけは例外で、ローカル宛てのログアウト後 URL を登録なしで受け付け
ます。`pw dev` ではどこにも登録しないままログアウトが動きます。
