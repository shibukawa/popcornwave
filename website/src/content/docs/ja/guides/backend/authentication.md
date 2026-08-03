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

このページは設定キー、エンドポイント、儀式、保存先のリファレンスです。どのモードを
選ぶか、そしてログインが終わったあとセッションが何を許されるかは
[認証の設計](/ja/guides/backend/authentication-design/) で決めます。

## 有効にする

エントリポイントと設定ファイル、2つです。

```go
// cmd/myapp/main.go — アカウントリゾルバの登録が plugin/auth の import を
// 兼ねます。エンドポイントとセッション解決はその拡張が担当します。ストレージの
// import 2つは SQLite のもの——セッションと、ログインの儀式が使い切る単回限りの
// レコードです。
import (
	_ "github.com/shibukawa/popcornwave/authstate/sqlite"
	_ "github.com/shibukawa/popcornwave/sessionstore/sqlite"
)

func main() {
	handlers.RegisterAccounts()
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

ストレージの import が `plugin/auth` と別なのは意図的です。auth プラグインは
バックエンドを一切リンクしないので、アプリケーションは設定したものだけを持ちます。
SQL ストアはエンジンごとに別パッケージなので、PostgreSQL に移るときは
`sessionstore/postgres` と `authstate/postgres` を import します。
`pw init --auth=oidc --db=postgres` はその行を書き出します。

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

### ログインが始まる前に満たしておくもの

4つあります。どれもサインインの途中で判明するのではなく、起動時に検査されます。

- `session.enabled = true`。そうでないとログインの着地先がありません。どのバックエンドが
  それを持つかは別の判断です（[セッションストレージ](/ja/guides/storage/session-storage/)）。
- `middleware.rdb.enabled = true`。これはセッションが cookie でも redis でも同じです。
  単回限りのログイン記録と許可リストは、どの構成でもサーバー側の状態だからです。
- マイグレーション適用済み。フレームワークのテーブルが2つとも存在すること。
- `issuer`、`client_id`、`client_secret`、`redirect_url` がすべて非空。スキャフォールドされた
  ファイルではプレースホルダであって、省略可能な設定ではありません。

issuer は `https` である必要があります。例外はループバックの開発用プロバイダだけで、
`auth.oidc.allow_loopback_http = true` を要求します。それ以外の場所でこのフラグを立てては
いけません。

## 設定できるものすべて

`[auth]` のキーは、フレームワークが何をマウントし、何を保護するかを決めます。

| キー | 既定値 | 意味 |
| --- | --- | --- |
| `enabled` | `false` | true のときだけエンドポイントとガードが存在する |
| `mode` | `"oidc_only"` | 実装があるのはこれだけ（[モード](#モード)） |
| `login_path` | `"/auth/login"` | プロバイダへの入口。ルート相対 |
| `callback_path` | `"/auth/callback"` | プロバイダが戻ってくる先。ルート相対 |
| `logout_path` | `"/auth/logout"` | ルート相対。`POST` のみ |
| `post_login_path` | `"/"` | 行き先の指定がないログインの着地先 |
| `protection.include` | `[]` | セッションを要求するパスのパターン |
| `protection.exclude` | `[]` | `include` から除外するパターン |
| `protection.unauthenticated` | `"redirect"` | ログインへ `redirect` するか、`401` の `unauthorized` か |

各パスはルート相対であることと `//` を含まないことが起動前に検証されます。打ち間違いは
「誰も到達できないルート」ではなく起動エラーになります。

`[auth.oidc]` のキーは、リライングパーティの登録内容と、誰を通すかを決めます。

| キー | 既定値 | 意味 |
| --- | --- | --- |
| `issuer` | *(空)* | **必須**。`allow_loopback_http` でなければ `https` |
| `client_id` | *(空)* | **必須** |
| `client_secret` | *(空)* | **必須**。起動サマリではマスクされる |
| `redirect_url` | *(空)* | **必須**。プロバイダに登録した値と一致すること |
| `scopes` | `[]` | `openid` に加えるスコープ |
| `identity_claim` | `"sub"` | ローカルアカウントを識別する検証済み claim |
| `admission` | `"authenticated"` | `authenticated` / `claim` / `registered` / `existing` |
| `auto_provision` | `true` | 未知の検証済み identity にアカウントを作らせる |
| `claim.path` | *(空)* | 検証済み claim への JSON Pointer。`admission = "claim"` で必須 |
| `claim.values` | `[]` | その位置で受け入れる値 |
| `claim.match` | `"any"` | `any` または `all` |
| `registered_claims` | *(空)* | 許可リストと突き合わせる claim。既定は `identity_claim` |
| `provider_logout` | `true` | ログアウト時にプロバイダ側のセッションも終える |
| `allow_loopback_http` | `false` | 開発時に `http` のループバック issuer を許す |

3つの秘密は `AUTH_OIDC_ISSUER`、`AUTH_OIDC_CLIENT_ID`、`AUTH_OIDC_CLIENT_SECRET` から、
あるいはファイル内の `${NAME}` 参照から与えます。どちらでも同じ値に届きます。どちらも
コミットするものではありません。

## 誰を通すか

検証済みの identity は、まだ認可された identity ではありません。その2つを分けるのが
`admission` です。

| `admission` | 通る相手 |
| --- | --- |
| `authenticated` | issuer が検証した全員 |
| `claim` | `claim.path` の値が `claim.values` に一致する identity |
| `registered` | 事前に許可リストのテーブルへ登録された identity |
| `existing` | アカウントリゾルバがすでに知っている identity のみ。`auto_provision = false` が必要 |

`claim` は、答えをすでにディレクトリが持っている場合の規則です。グループ、ロール、部署。
`claim.match = "all"` は列挙した値すべてを要求するのでグループの積に、`any` は最初の一致で
通します。

`registered` は、初回ログインより前に利用者が分かっている閉じたデプロイの規則です。許可
リストのテーブルは issuer と claim 名と期待値を取ります。`registered_claims` があるのは
このためで、社員番号で事前登録する運用なら、まだ知りようのない subject ではなくその claim を
突き合わせます。

どの規則で通ったとしても、アカウントとの結び付きは issuer と `identity_claim` が指す claim の
組です。メールアドレスではありません。プロバイダはそれを再割り当てします。`identity_claim` を
変えるのは、アカウントの生涯にわたって安定かつ一意だとディレクトリが保証する値に対してだけに
してください。

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

クッキーが運ぶのは不透明なトークンだけで、セッション本体がどこに住むかは
`session.backend` が決めます。この選択はここまでの設定から独立しています。3つの
バックエンド、それぞれに必要なキー、そして何を諦めるかは
[セッションストレージ](/ja/guides/storage/session-storage/)にあります。`session.ttl` が絶対有効期限、
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

デプロイ時には、その利便性は外れます。`issuer`、`client_id`、`client_secret`、
`redirect_url` はいずれも空であってはならず、不足しているとアプリケーションは起動を
拒否して不足分を示します。プロバイダの値は `AUTH_OIDC_ISSUER`、`AUTH_OIDC_CLIENT_ID`、
`AUTH_OIDC_CLIENT_SECRET`、あるいは `${NAME}` 参照から与え、コミットはしません。
cookie バックエンドのセッションはもう1つ独自の秘密鍵を要求します
（[セッションストレージ](/ja/guides/storage/session-storage/#cookie--ストレージなし)）。

`redirect_url` はプロバイダに登録した URL と一字一句一致している必要があります。
フレームワークが `redirect_uri` として送るのがこの値なので、登録と違えばアプリケーションに
届く前にプロバイダが拒否します。

ログアウト後の URL もプロバイダに登録してください。未登録の `post_logout_redirect_uri`
は拒否されます。フレームワークが送るのはリクエストオリジンのルートなので、
`https://app.example/` で配信しているならその URL を登録します。

開発用プロバイダだけは例外で、ローカル宛てのログアウト後 URL を登録なしで受け付け
ます。`pw dev` ではどこにも登録しないままログアウトが動きます。
