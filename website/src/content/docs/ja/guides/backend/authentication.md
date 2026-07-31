---
title: 認証
description: 認証モードを選べば、ログイン・コールバック・ログアウトとパスキーのエンドポイントはフレームワークが提供する。
sidebar:
  order: 1
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

パスキーはアカウントを作れません。最初のクレデンシャルを結びつける先が無いからです。
モードの違いはログイン方法そのものではなく、**アカウントが存在する前に何がそれを
確立するか**にあります。

| `auth.mode` | アカウントの出どころ | 日常のログイン |
| --- | --- | --- |
| `oidc_only` | プロバイダ | プロバイダ |
| `oidc_passkey` | プロバイダ | パスキー。プロバイダは復旧手段 |
| `passkey_only` | 管理者が発行するログイン ID と使い捨てシークレット | パスキー |

各モードは自分が使う設定だけを読み、扱えない設定は拒否します。`passkey_only` で
`AUTH_OIDC_ISSUER` が残っていれば起動時にエラーになり、プロバイダが関与しているかの
ような見え方にはなりません。黙って無視される設定は、設定済みのセキュリティに見えて
しまうためです。

### パスキーのエンドポイント

儀式をマウントするモードは `auth.passkey.path`（既定 `/auth/passkey`）配下に 5 本を
提供します。`POST` と JSON のみで、bootstrap は `passkey_only` にしか存在しません。

```
POST /auth/passkey/login/begin      POST /auth/passkey/login/finish
POST /auth/passkey/register/begin   POST /auth/passkey/register/finish
POST /auth/passkey/bootstrap        (passkey_only のみ)
```

フレームワークはエンドポイントを提供できますが、ページの代わりに
`navigator.credentials` を呼ぶことはできません。そのため、エンドポイントが話す
Base64url と WebAuthn API が要求する ArrayBuffer を変換する小さなスクリプトが必要で、
`pw init` が `public/passkey.js` として生成します。

パスキーのモードではアカウントの継ぎ目が 1 つ増えます。`auth.SetAccountResolver` は
「この検証済み ID はどのアカウントか」に答え、`auth.SetAccountLookup` は「この識別子は
どのアカウントか」に答えます。パスキーのアサーションが必要とするのは後者の向きです。
クレデンシャル自体がアカウントを名指しするので、結びつける外部 ID が存在しません。

### パスキー構成にはアドレスではなく名前で到達する

WebAuthn の Relying Party は**ドメイン**にスコープされ、IP リテラルは RP ID になれません。
`http://127.0.0.1:8080` ではなく `http://localhost:8080` を使ってください。WebAuthn は
`localhost` を secure origin として扱うので、ローカル開発に証明書もトンネルも要りません。

### `passkey_only` の初回サインイン

管理者が `auth.IssueBootstrapCredential` でログイン ID と使い捨てシークレットを発行します。
生のシークレットが返るのは一度きりで、保存されるのはダイジェストだけです。引き換えると
**登録を 1 回だけ許可するチケット**が得られます。セッションではありません。パスキーが
永続化されるまでリクエストは未認証のままなので、引き換え済みのシークレットをログイン状態と
取り違えるハンドラは存在しえません。

`auth.bootstrap.issue_ttl` は受け渡しの猶予を、`auth.bootstrap.enrollment_ttl` は
その後の儀式を区切ります。既定値が時間単位で違うのはそのためです。

## セッション

クッキーが運ぶのは不透明なトークンだけで、セッション本体は `plugin/session/rdb` が
データベースに保存します。だからサーバー側で失効させられます。`session.ttl` が絶対
有効期限、`session.idle_timeout` が無操作期限です。ログイン時にトークンは新しくなり、
それ以前にブラウザが持っていたセッションは失効します。

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
