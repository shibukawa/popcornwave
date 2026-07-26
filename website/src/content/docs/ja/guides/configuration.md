---
title: 設定
description: APP_ENV、ファイルの解決順序、フレームワークの設定項目、アプリケーション固有の設定。
sidebar:
  order: 7
---

設定は TOML ファイル、環境変数、コマンドラインオプションから読み込まれ、型付きの構造体に
バインドされます。`pw.Run` は最初のリクエストを処理する前にパースするので、不正な値は
実行時ではなく起動時の失敗になります。

## 実行環境

`APP_ENV` が実行環境を選択します。`dev`、`stg`、`prod`、またはその他の小文字・数字・
`-`・`_` からなるトークンを受け付けます。不正なトークンは `ParseConfig` を失敗させます。
未設定または空の場合は **`dev`** が既定です。

```sh
APP_ENV=prod ./myapp
```

`pw.Env()` が解決済みのトークンを返し、`pw.EnvDevelopment`、`pw.EnvStaging`、
`pw.EnvProduction` がよく使う値を表します。

## ファイルの解決

プロジェクトローカルのファイルは環境ごとに分かれ、作業ディレクトリ、次にその `config/`
ディレクトリの順で探索されます。

1. `./config.{APP_ENV}.toml`
2. `./config/config.{APP_ENV}.toml`

ユーザおよびシステムの設定ディレクトリは環境非依存の `config.toml` を持ちます。素の
`config.toml` がプロジェクトツリーから読まれることはありません。

チェーンの後ろにあるもの —— 環境変数、次にオプション —— が前のものを上書きします。

## フレームワークの設定項目

フレームワーク自身が 5 つの prefix を登録します。

### `[server]`

| キー | 既定値 |
| --- | --- |
| `port` | `8080` |
| `read_header_timeout` | `5s` |
| `read_timeout` | `30s` |
| `write_timeout` | `0s`（無制限。長時間のストリームを許容する） |
| `idle_timeout` | `2m` |
| `shutdown_timeout` | `10s` |
| `max_request_body` | `10485760` |
| `trusted_proxies` | *(空)* |
| `health.enabled` / `health.path` | `true` / `/healthz` |
| `readiness.enabled` / `readiness.path` | `true` / `/readyz` |
| `openapi.enabled` / `openapi.path` | `true` / `/openapi.json` |
| `public.enabled` / `public.mount` | `true` / `/public` |
| `public.read_local` | `false` |

有効な運用エンドポイントと自分のルートが衝突すると、起動時に報告されます。

### `[middleware]`

| キー | 既定値 |
| --- | --- |
| `recovery` | `true` |
| `request_id` | `true` |
| `access_log` | `true` |
| `compression` | `false` |
| `request_timeout` | `0s` |
| `rdb.enabled` | `false` |
| `rdb.dsn` | *(空)* |
| `rdb.connect_timeout` | `5s` |
| `rdb.max_open_conns` / `rdb.max_idle_conns` | `0` |
| `rdb.conn_max_lifetime` / `rdb.conn_max_idle_time` | `0s` |

`compression` を有効にすると、受け入れるクライアントに対して HTML レスポンスが zstd で
エンコードされます。`Vary: Accept-Encoding` はいずれの場合も設定されます。

### `[security]`

`headers.enabled`（`true`）、`headers.content_type_options`（`true`）、
`headers.frame_options`（`deny`）、`headers.referrer_policy`
（`strict-origin-when-cross-origin`）、`headers.content_security_policy`、
`headers.content_security_policy_report_only`、`headers.permissions_policy`、そして
既定で無効の `headers.hsts` ブロック（HTTPS と確認できたリクエストにのみ適用）。

### `[observability]`

`minimum_level`（`info`）と `service_name`。後者は `OTEL_SERVICE_NAME` も読みます。

### `[session]`

`enabled`（`false`）、`ttl`（`24h`）、`secret`。後者は `SESSION_SECRET` も読みます。

### `[auth]`

`enabled`（`false`）と `mode`（`oidc`、`oidc_passkey`、`passkey_only`）、および
`login_path`、`callback_path`、`logout_path`、`post_login_redirect`、
`post_logout_redirect`、および `[auth.oidc]` のプロバイダ設定 `issuer`、`client_id`、
`client_secret`、`redirect_url`、`scopes`、`provider_logout`（既定 `true`。ログアウト
時にプロバイダ側のセッションも終了する）。プロバイダ設定はそれぞれ環境変数も読みます
（`AUTH_OIDC_ISSUER`、`AUTH_OIDC_CLIENT_ID`、`AUTH_OIDC_CLIENT_SECRET`、
`AUTH_OIDC_REDIRECT_URL`）。

OIDC 系のモードで `issuer`、`client_id`、`client_secret` のいずれかが空の場合、最初の
ログイン時ではなく起動時に失敗します。エラーには足りないキーと対応する環境変数が
並びます。

```
auth.mode "oidc" needs auth.oidc.issuer (AUTH_OIDC_ISSUER), auth.oidc.client_id
(AUTH_OIDC_CLIENT_ID); run pw dev to use the development identity provider, or
supply the values in config.dev.toml or the environment
```

ローカルエミュレータ向けにスキャフォールドしたプロジェクトにプロバイダの値が一切
書かれていないのはこのためです。[`pw dev`](/ja/pw/project/dev/) が注入し、それなしで
起動すれば何が足りないかがそのまま表示されます。

## 独自の設定を追加する

アプリケーションの設定はフレームワークのものと同じ仕組みです。構造体を宣言し、prefix
を付けて登録し、リクエストの context から読みます。`pw generate` が登録呼び出しを読んで
バインディングを書き出すので、パース処理を保守する必要はありません。

### 1. 構造体を宣言する

```go
package handlers

import "github.com/shibukawa/popcornwave/pw"

type AppConfig struct {
	EnvLabel      string `default:"local" help:"environment name shown in the page badge"`
	EnvLabelColor string `default:"#64748b" help:"CSS color of the environment badge"`
}
```

フィールド名は snake_case のキーになるので、`EnvLabel` は `app.env_label` です。
5 つのタグで調整できます。

| タグ | 効果 |
| --- | --- |
| `default:"value"` | 他のどこからも値が来なかったときの値 |
| `key:"name"` | TOML や設定のキーを上書きする |
| `opt:"long"` / `opt:"long,s"` | CLI オプションを上書きする。短縮形も指定可 |
| `env:"NAME"` / `env:"-"` | 環境変数名を明示する、または環境変数入力を無効にする |
| `help:"text"` | usage やスキャフォールドに表示される説明 |

ネストした構造体はキーもネストします。prefix が `app` で

```go
type AppConfig struct {
	Mailer MailerConfig
}

type MailerConfig struct {
	FromAddress string `default:"noreply@example.com"`
}
```

の場合、キーは `app.mailer.from_address`、TOML は `[app.mailer]
from_address = …`、オプションは `--app-mailer-from_address`、環境変数は
`APP_MAILER_FROM_ADDRESS` になります。

:::caution
バインド可能なフィールド型は `string`、`bool`、`int`、`[]string`、およびそれらを含む
ネストした構造体です。float、map、ポインタ、それ以外のスライス、`time.Duration` は
**バインドできません**。`string` か `int` として宣言し、パース後に変換してください。
（フレームワーク自身の `[server]` の duration が動くのは、そのバインディングが生成では
なく手書きだからです。）
:::

### 2. 登録する

```go
func RegisterConfig() { pw.RegisterConfig[AppConfig]("app") }
```

```go
func main() {
	handlers.RegisterConfig()
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

このタイミングは偶然ではありません。生成された定義はパッケージの `init` で登録される
ため、バインディング自体はすべての `init` が走った**あと**、かつ設定がパースされる
**前**に作る必要があります。`ParseConfig` の後に登録すると panic します。また生成器が
読めるよう、prefix は文字列リテラルでなければなりません。

大きめのアプリケーションでは各エリアが自分の構造体を登録できます
（[プロジェクト構成](/ja/guides/project-structure/)を参照）が、prefix は 1 つの名前空間を
共有するので、`app`、`billing`、`search` のように区別できる名前を付けてください。

### 3. 読む

```go
app := pw.Config[AppConfig](r.Context())
```

`pw.Config` はリクエスト context がある場所ならどこでも使えます。リクエスト外では
`nil` を渡します。

### 4. 設定する

```toml
[app]
env_label = "development"
env_label_color = "#059669"
```

```sh
APP_ENV_LABEL=development ./myapp
./myapp --app-env_label=development
```

## スキャフォールドの出力

登録済みのすべての prefix —— フレームワークのものもアプリケーションのものも —— は、
`default` 値を埋め `help` をコメントにした形で自分自身を出力できます。

```sh
./myapp --generate-config toml > config.dev.toml
./myapp --generate-config env > .env
```

バイナリは自分がインポートしたものが登録した内容を報告するので、スキャフォールドは常に
実際にリンクされているパッケージと一致します。構造体を追加してこれを実行すれば、新しい
キーがそこにあります。[アプリケーション CLI](/ja/guides/application-cli/)を参照。

## ログ中の秘密情報

起動時に、解決された各キーがその取得元とともにログ出力されます。キー名に `secret`、
`password`、`token`、`credential`、`dsn`、`private_key` を含むものは `[REDACTED]` として
記録されます。
