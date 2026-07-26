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
// cmd/myapp/main.go
import _ "github.com/shibukawa/popcornwave/auth"
```

```toml
# config.dev.toml
[auth]
enabled = true
mode = "oidc"

[auth.oidc]
issuer = "https://issuer.example"
client_id = "..."
client_secret = "..."
provider_logout = true   # プロバイダ側もサインアウトする

[session]
enabled = true
secret = "..."     # 本番では SESSION_SECRET
```

`pw init --auth=oidc` は両方を書き出します。実装を登録するのはブランク import です。
`auth.enabled = true` なのに登録がない場合、起動時に失敗し、不足している import を
名指しで知らせます。

## エンドポイント

| パス | メソッド | 動作 |
| --- | --- | --- |
| `auth.login_path`（`/auth/login`） | GET | プロバイダへリダイレクトする |
| `auth.callback_path`（`/auth/callback`） | GET | 結果を検証してセッションを開始する |
| `auth.logout_path`（`/auth/logout`） | POST | セッションを終了する |

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
`id_token_hint`、`client_id`、`auth.post_logout_redirect` を指す
`post_logout_redirect_uri` を付けて、プロバイダの RP-initiated logout を経由します。

```
POST /auth/logout
  → 303 https://issuer.example/end_session?client_id=…&id_token_hint=…&post_logout_redirect_uri=…
  → 302 ログアウト後のページへ
```

ローカルだけのログアウトに留めたい場合は `auth.oidc.provider_logout = false` に
します（他のアプリケーションと共有するプロバイダで、そちらはログインを維持したい
場合など）。`end_session_endpoint` を公開していないプロバイダでは自動的にローカル
ログアウトにフォールバックします。

ログイン後は `auth.post_login_redirect`、ログアウト後は `auth.post_logout_redirect`
に着地します。どちらも同一オリジンの絶対パスでなければならず、絶対 URL は
オープンリダイレクトになる前に起動時点で拒否されます。

## ユーザーを読む

セッションはハンドラが動く前に解決されています。

```go
func home(w http.ResponseWriter, r *http.Request) {
	identity, signedIn := pw.CurrentUser(r.Context())
	if signedIn {
		// identity.Subject, identity.Name, identity.Email
		role, _ := identity.Claim("role")
		_ = role
	}
	// ...
}
```

`Identity` はプロバイダが証明した内容であって、アカウントのレコードではありません。
`Subject` は `Issuer` の中で安定した識別子なので、そこから自分のアカウントを解決して
ください。期限切れ・改竄・壊れたセッションクッキーは黙って捨てられます。匿名の
リクエストは異常ではなく通常の状態なので、`CurrentUser` は単に `false` を返します。

`pw.CurrentUser` が答えるのは「誰か」だけで、「何をしてよいか」ではありません。認可は
アプリケーションの責任のままです。

## モード

| `auth.mode` | 状態 |
| --- | --- |
| `oidc` | 実装済み |
| `oidc_passkey` | 現状は OIDC ログインのみ。パスキー登録は未実装 |
| `passkey_only` | 未実装。`pw init` は `enabled = false` で雛形だけ書く |

## セッション

セッションは署名付きクッキーです（`HttpOnly`、`SameSite=Lax`、HTTPS では `Secure`）。
subject といくつかの claim、そしてログアウト時のヒントに使う ID Token を持ち、
`session.ttl` の間有効です。ID Token がハンドラに渡ることはありません。プロバイダの
claim が大きくクッキーの上限を超える場合は、セッションではなくヒントの方を落とします。

サーバー側のストアを動かす必要はありません。署名鍵は `session.secret` で、これが
ないと認証は起動を拒否します。鍵を変えれば全セッションが無効になります。

識別子は小さく保ってください。それ以上の情報は subject をキーにした自前のストレージ
の担当です。

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
