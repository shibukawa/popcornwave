---
title: 設定キー
description: フレームワークの全設定キーと既定値、そして TOML・環境変数・コマンドラインでの名前。
sidebar:
  order: 2
---

構造体のフィールド1つが、3つの入力になります。`ServerConfig.ReadHeaderTimeout` は
TOML では `server.read_header_timeout`、環境変数では `SERVER_READ_HEADER_TIMEOUT`、
コマンドラインでは `--server-read_header_timeout` です。しかも後ろ2つは、誰かが
保守している対応表ではなく規則で最初のひとつから導かれます。そこでこのページの
一覧では各キーを1回だけ挙げます。残りの2つは自分で導けます。

規則には例外があり、この規則の例外は表を見る前に知っておく価値がある程度の数です。
設定を*どう使うか* —— 自分の設定を宣言し、ハンドラで読み、ひな形を生成する —— は
[設定](/ja/guides/architecture/configuration/)にあります。

## 残り2つの名前の導き方

TOML のキーの `.` を `-` に置き換えて `--` を付けます。
`observability.query.slow_threshold` なら `--observability-query-slow_threshold`
です。キーの中のアンダースコアはそのまま残り、変わるのは階層を区切るドットだけ
です。

そのオプション名からダッシュを外して大文字にすれば
`OBSERVABILITY_QUERY_SLOW_THRESHOLD` になります。

5つのキーは意図的に規則を外れています。慣例的な名前がすでに存在していて、
アプリケーション側に読み替えを強いる理由がないからです。

| キー | 環境変数 |
| --- | --- |
| `server.port` | `PORT`（オプションは `--port`） |
| `observability.service_name` | `OTEL_SERVICE_NAME` |
| `observability.otel.endpoint` | `OTEL_EXPORTER_OTLP_ENDPOINT` |
| `observability.otel.headers` | `OTEL_EXPORTER_OTLP_HEADERS` |
| `auth.oidc.issuer`、`client_id`、`client_secret`、`redirect_url` | `AUTH_OIDC_*`。規則どおりの結果を、導出ではなく固定で指定している |

環境変数を持たないキーも3つあります。
`security.headers.content_security_policy`、その `_report_only` 版、
`security.headers.permissions_policy` です。TOML で設定してください。

`[[middleware.rdb.connections]]` の配列も TOML 専用です。テーブルの配列には
バインドできる平坦な名前がありません。

## 値の出どころ

`APP_ENV` が実行環境を選び、それがプロジェクトローカルのファイル名を決めます。
Popcorn Wave は次の順に読みます。

1. `./config.{APP_ENV}.toml`
2. `./config/config.{APP_ENV}.toml`
3. ユーザおよびシステムの設定ディレクトリ。こちらのファイル名は環境非依存の
   `config.toml`

プロジェクトツリーに置いた素の `config.toml` は読まれません。後のソースが前を
上書きし、環境変数とオプションはすべてのファイルを上書きします。

期間は Go の duration 文字列（`5s`、`200ms`、`2m`、`24h`）、サイズはバイト数の
整数です。リストは TOML の配列、環境変数ではカンマ区切りの値になります。

## `[server]`

| キー | 既定値 | 意味 |
| --- | --- | --- |
| `port` | `8080` | HTTP の待ち受けポート |
| `read_header_timeout` | `"5s"` | リクエストヘッダ読み取りのタイムアウト |
| `read_timeout` | `"30s"` | リクエスト読み取りのタイムアウト |
| `write_timeout` | `"0s"` | レスポンス書き込みのタイムアウト。ゼロは長時間のストリームを許容する |
| `idle_timeout` | `"2m"` | keep-alive のアイドルタイムアウト |
| `shutdown_timeout` | `"10s"` | グレースフルシャットダウンのタイムアウト |
| `max_request_body` | `10485760` | リクエストボディの上限（バイト） |
| `trusted_proxies` | `[]` | 信頼するプロキシの IP または CIDR |
| `health` | *(空)* | liveness エンドポイントのパス。例 `/healthz` |
| `readiness` | *(空)* | readiness エンドポイントのパス。例 `/readyz` |
| `openapi` | *(空)* | OpenAPI ドキュメントのパス。例 `/openapi.json` |
| `api_doc` | *(空)* | API ドキュメント UI。`scalar`、`swagger`、または空 |
| `api_doc_path` | `"/docs"` | その UI のマウント先 |
| `public.enabled` | `true` | 埋め込み静的アセットを配信する |
| `public.mount` | `"/public"` | そのマウント先 |
| `public.read_local` | `false` | 埋め込みツリーではなくディスクから読む |

運用系の3エンドポイントに既定パスがないのは意図的です。`/healthz` で応答する
アプリケーションなら、設定を読む運用者の目に入る場所にそう書かれているべきです。
既定値を持たせれば、どのファイルにも書かれていないエンドポイントが3つ動くことに
なります。

アプリケーションのルートが有効な運用エンドポイントと衝突した場合、どちらかが
もう一方を覆い隠す前に起動が失敗します。`api_doc` はさらに `openapi` を必要とします。
誰も配信していないドキュメントの UI には、描画するものがありません。
[API ドキュメント](/ja/productivity/api-documentation/)を参照してください。

## `[middleware]`

| キー | 既定値 | 意味 |
| --- | --- | --- |
| `recovery` | `true` | panic したハンドラを 500 に回収する |
| `request_id` | `true` | リクエスト相関 ID を採番して伝播する |
| `access_log` | `true` | リクエストごとに1レコード |
| `compression` | `false` | 受け入れるクライアントに HTML を zstd で返す |
| `request_timeout` | `"0s"` | リクエスト単位の期限。ゼロなら設けない |
| `rdb.enabled` | `false` | フレームワーク所有のデータベースプールを開く |
| `rdb.dsn` | *(空)* | 単一データベースの DSN（起動サマリではマスクされる） |
| `rdb.connect_timeout` | `"5s"` | 接続を開く際の上限 |
| `rdb.max_open_conns` | `0` | `database/sql` のプール上限。ゼロはドライバの既定 |
| `rdb.max_idle_conns` | `0` | |
| `rdb.conn_max_lifetime` | `"0s"` | |
| `rdb.conn_max_idle_time` | `"0s"` | |
| `rdb.default_group` | *(空)* | どのグループも指定しないステートメント用の接続グループ |
| `rdb.write_group` | *(空)* | フレームワークの書き込み用の接続グループ |
| `rdb.migration_group` | *(空)* | マイグレーションとシード用の接続グループ |

`compression` が有効なとき、`Vary: Accept-Encoding` はどちらの経路でも付きます。
片方の表現をキャッシュしたものが、もう片方を求めるクライアントへ渡ってはいけない
からです。

単一のデータベースは `rdb.dsn` と上のプール系キーで設定します。リーダ・ライタ構成は
代わりに接続セットで、プール1つにつきテーブル1つを書きます。両方を書くのはマージ
ではなく設定エラーです。どちらが勝つのか、正直に答えられないからです。

### `[[middleware.rdb.connections]]`

| キー | 既定値 | 意味 |
| --- | --- | --- |
| `group` | *(空)* | この接続を指す名前 |
| `dsn` | *(空)* | DSN（起動サマリではマスクされる） |
| `readonly` | `false` | 読み取り専用トランザクションを開き、フレームワークの書き込みは引き受けない |
| `connect_timeout` | `"5s"` | 以下、接続ごとに上と同じ |
| `max_open_conns` | `0` | |
| `max_idle_conns` | `0` | |
| `conn_max_lifetime` | `"0s"` | |
| `conn_max_idle_time` | `"0s"` | |

`readonly` の接続が `pw.SelectWriteDB` に選ばれることはありません。だからこそ、
書き込みを行う側はデプロイ構成を知らずに済みます。
[クエリ](/ja/guides/backend/queries/)を参照してください。

## `[html]`

| キー | 既定値 | 意味 |
| --- | --- | --- |
| `streaming` | `true` | `false` はストリーミング可能なチェーンでもバッファ経路を強制する |
| `async_timeout` | `"3s"` | await 境界1つの上限。ゼロならリクエストコンテキストだけが期限になる |
| `async_concurrency` | `0` | 1回の描画で同時に走る境界処理の数。ゼロ以下は無制限 |
| `bot_detection` | `true` | クローラや CLI クライアントには確定済みのドキュメントを描画する |
| `bot_async_timeout` | `"5s"` | bot と判定したリクエストでの境界の上限 |
| `bot_user_agents` | `[]` | 追加の `User-Agent` 部分文字列。大文字小文字を無視して照合する |

await 境界を開くテンプレートは、`streaming` がどちらでも正しく描画されます。この
キーが決めるのは、その裏の処理が確定する前に fallback がブラウザへ届くかどうか
だけです。だから `streaming = false` は、どのみちレスポンスをバッファするプロキシ
のための逃げ道になります。

`bot_async_timeout` が `async_timeout` より大きいのは、bot 判定されたリクエストが
1バイトも返す前にすべての境界を待つうえ、インデクサはブラウザより長く待つから
です。ここのゼロは無制限ではなく `async_timeout` へのフォールバックです。打ち間違え
たキーが、リクエスト期限いっぱいクローラの接続を掴んだままにしてはいけません。
`bot_user_agents` の項目は組み込みのカタログに追加されるだけで、組み込みのトークンを
置き換えることはありません。

## `[security]`

| キー | 既定値 | 意味 |
| --- | --- | --- |
| `headers.enabled` | `true` | 以下すべてが従うスイッチ |
| `headers.content_type_options` | `true` | `X-Content-Type-Options: nosniff` |
| `headers.frame_options` | `"deny"` | `X-Frame-Options` |
| `headers.referrer_policy` | `"strict-origin-when-cross-origin"` | `Referrer-Policy` |
| `headers.content_security_policy` | *(空)* | `Content-Security-Policy`（環境変数なし） |
| `headers.content_security_policy_report_only` | *(空)* | その report-only 版（環境変数なし） |
| `headers.permissions_policy` | *(空)* | `Permissions-Policy`（環境変数なし） |
| `headers.hsts.enabled` | `false` | `Strict-Transport-Security`。検証済み HTTPS のリクエストにのみ付く |
| `headers.hsts.max_age` | `"0s"` | |
| `headers.hsts.include_subdomains` | `false` | |
| `headers.hsts.preload` | `false` | |

HSTS が付くのは検証済みの HTTPS リクエストだけです。平文で送るのは、その接続では
保証できないポリシーの記憶をブラウザに頼むことになります。

## `[observability]`

| キー | 既定値 | 意味 |
| --- | --- | --- |
| `minimum_level` | `"info"` | 重要度の下限。`trace`、`debug`、`info`、`warn`、`error`、`off` |
| `stdout_format` | `"json"` | 端末でのレコード表現。`json` または `plaintext` |
| `service_name` | *(空)* | `OTEL_SERVICE_NAME` からも読む |
| `resource_attributes` | `[]` | サービス名とともに報告する追加の `key=value` |
| `boot_log` | `"auto"` | 起動サマリ。`auto`、`tree`、`record`、`off` |

`auto` は対話的な端末ではツリーを、それ以外では構造化レコード1件を出します。
[起動サマリ](/ja/productivity/startup-summary/)を参照してください。

### `[observability.query]`

| キー | 既定値 | 意味 |
| --- | --- | --- |
| `enabled` | `"auto"` | 生成された全ステートメントを記録する。`auto`、`on`、`off`。`auto` は `dev` で on |
| `level` | `"info"` | 通常のステートメントレコードの重要度 |
| `slow_threshold` | `"200ms"` | これを超えると slow 扱い。ゼロで slow 検出を無効化 |
| `slow_level` | `"warn"` | slow なステートメントレコードの重要度 |
| `bind_values` | `"auto"` | 引数の値を記録する。`auto`、`on`、`off` |
| `explain` | `true` | slow なステートメントの EXPLAIN（プランのみ）を取得する |
| `reproduction` | `true` | slow なステートメントの再実行スニペットを出す |
| `max_sql_length` | `4096` | 記録するステートメント本文の上限 |
| `max_value_length` | `256` | 記録する引数値ごとの上限 |

`auto` は設定を実行環境に結びつけます。開発中の実行は無設定で計測され、それ以外の
環境は誰かが明示的に有効化するまで黙ったままです。`explain` と `reproduction` が
依存するのは `enabled` ではなく `slow_threshold` です。しきい値をゼロにすると、
この3つが同時に止まります。
[クエリ診断](/ja/productivity/query-diagnostics/)を参照してください。

### `[observability.otel]`

| キー | 既定値 | 意味 |
| --- | --- | --- |
| `enabled` | `false` | トレースとログをエクスポートする |
| `endpoint` | *(空)* | OTLP/HTTP のベース URL。`/v1/traces` と `/v1/logs` が付加される |
| `headers` | *(空)* | カンマ区切りの `key=value`。値がログに出ることはない |
| `request_timeout` | `"10s"` | エクスポート1回の上限 |
| `queue_size` | `2048` | メモリに保持するレコード数。満杯ならリクエストを止めずに捨てる |
| `max_export_size` | `512` | 1バッチの上限 |
| `flush_interval` | `"5s"` | 端数のバッチを送る間隔 |

これらの既定値は、ゼロ値に対してエクスポータが適用する上限をそのまま書き写した
ものです。生成されたファイルが「誰かに聞け」という意味のゼロではなく、プロセスの
実際の挙動を語るようにするためです。

`OTEL_EXPORTER_OTLP_TIMEOUT` は意図的に `request_timeout` へ**バインドしていません**。
標準の変数はミリ秒を数えますが、ここの期間はすべて Go の duration 文字列です。
ひとつのキーが両方を意味することはできません。

## `[session]`

| キー | 既定値 | 意味 |
| --- | --- | --- |
| `enabled` | `false` | |
| `backend` | `"rdb"` | 保存バックエンド: `rdb`、`cookie`、`redis` |
| `ttl` | `"24h"` | セッションの絶対寿命 |
| `idle_timeout` | `"0s"` | 無操作での失効。ゼロで無効 |
| `renewal_interval` | `"0s"` | 無操作失効の更新間隔の下限 |
| `cookie.name` | `"pw_session"` | |
| `cookie.path` | `"/"` | |
| `cookie.domain` | *(空)* | |
| `cookie.secure` | `true` | 無効にしてよいのはループバックの開発時だけ |
| `cookie.http_only` | `true` | |
| `cookie.same_site` | `"lax"` | |
| `rdb.source` | `"middleware"` | `middleware` は `middleware.rdb` のプールを再利用し、`dedicated` は `rdb.dsn` を開く |
| `rdb.group` | *(空)* | セッションテーブルを持つ接続グループ。空なら `middleware.rdb.write_group` |
| `rdb.dsn` | *(空)* | 専用セッションデータベース（起動サマリではマスクされる） |
| `rdb.table` | `"popcornwave_session"` | |
| `redis.dsn` | *(空)* | `redis://` または `rediss://` のサーバー（起動サマリではマスクされる） |
| `redis.key_prefix` | `"pw:session:"` | セッションストアが所有する鍵空間 |
| `redis.connect_timeout` | `"5s"` | 起動時の ping と各コマンドの期限 |
| `cookie_store.name` | `"pw_session_data"` | `backend = "cookie"` で封をしたレコードを運ぶクッキー |
| `cookie_store.secret` | *(空)* | クッキーのレコードを封印する base64 の秘密鍵（マスクされる） |
| `cookie_store.previous_secrets` | `[]` | ローテーション中も読める引退した秘密鍵（マスクされる） |

読まれるのは選んだバックエンドのキーだけです。`cookie` 以外のバックエンドは、それ自身の
blank import でバイナリに入ります。書き忘れたときは起動時のエラーが追加すべき行を引用
します。3つの比較は[クッキー](/ja/guides/backend/cookies/)にあります。

ブラウザにあるトークンはどのバックエンドでも不透明なので、ここに署名鍵はありません。
レコードそのものをブラウザに置くのは `backend = "cookie"` だけで、その場合は
`cookie_store.secret` で封をします。このセクション唯一の秘密鍵であり、ファイルではなく
環境に置くべきものです。

## `[auth]`

これらのキーが存在するのは、`plugin/auth` がバイナリにリンクされているときだけ
です。アカウントリゾルバを登録すると、そうなります。認証まわりを何もインポートして
いないアプリケーションには、設定すべき `[auth]` prefix そのものがありません。

| キー | 既定値 | 意味 |
| --- | --- | --- |
| `enabled` | `false` | |
| `mode` | `"oidc_only"` | |
| `login_path` | `"/auth/login"` | プロバイダのフローを開始する |
| `callback_path` | `"/auth/callback"` | 結果を検証してセッションを開始する |
| `logout_path` | `"/auth/logout"` | セッションを終了する。`POST` のみ |
| `post_login_path` | `"/"` | ログイン完了後に着地するローカルパス |
| `protection.include` | `[]` | セッションを必要とするパスパターン |
| `protection.exclude` | `[]` | 公開のままにするパスパターン |
| `protection.unauthenticated` | `"redirect"` | `redirect` または `unauthorized` |

### `[auth.oidc]`

| キー | 既定値 | 意味 |
| --- | --- | --- |
| `issuer` | *(空)* | `AUTH_OIDC_ISSUER` |
| `client_id` | *(空)* | `AUTH_OIDC_CLIENT_ID` |
| `client_secret` | *(空)* | `AUTH_OIDC_CLIENT_SECRET`（起動サマリではマスクされる） |
| `redirect_url` | *(空)* | `AUTH_OIDC_REDIRECT_URL` |
| `scopes` | `[]` | |
| `identity_claim` | `"sub"` | ローカルアカウントを識別する検証済みクレーム |
| `admission` | `"authenticated"` | `authenticated`、`claim`、`registered`、`existing` |
| `auto_provision` | `true` | 未知の検証済み identity にリゾルバ経由でのアカウント作成を許す |
| `claim.path` | *(空)* | 検証済みクレームへの JSON Pointer。`admission = "claim"` 用 |
| `claim.values` | `[]` | 受け入れる値 |
| `claim.match` | `"any"` | `any` または `all` |
| `registered_claims` | `[]` | 許可リストと突き合わせるクレーム。既定は `identity_claim` |
| `provider_logout` | `true` | ログアウト時にプロバイダ側のセッションも終了する |
| `allow_loopback_http` | `false` | 開発時に `http` のループバック issuer を許可する |

`identity_claim` はアカウントとの結びつきそのものになるため、そこに指定する値は
アカウントの生涯にわたって安定し、かつ issuer の中で一意でなければなりません。
再発行や使い回しがあれば、ある人に別人のアカウントを渡すことになります。利用者を
事前に用意しておくデプロイでは subject をまだ知りようがないことが多く、社員番号の
ような自前のディレクトリ識別子をここに指すのが普通です。

有効な OIDC モードで `issuer`、`client_id`、`client_secret` のいずれかが空なら、
最初のログイン時ではなく起動時に失敗します。エラーは不足しているキーと、その環境
変数の両方を名指しします。ローカルのエミュレータ向けに生成されたプロジェクトが
プロバイダの値を一切持たないのはそのためで、[`pw dev`](/ja/pw/project/dev/) が
注入します。[認証](/ja/guides/backend/authentication/)を参照してください。

## 設定したのに起動サマリに出ないキー

上のキーの多くは、親のスイッチに従います。`server.api_doc_path` は
`server.api_doc` に、`[auth.oidc]` のすべては `auth.enabled` に、
`observability.otel.endpoint` は `otel.enabled` に依存します。

親が空か false のとき、サマリは従属するキーを省き、親のほうは残します。従属キーが
消えた理由が親だからであり、何も決めていないキーを7つ並べれば、実際に効いている
1つが埋もれるからです。バインド自体には影響しません。値は読まれていて、ただ作用
する先がないだけです。つまり、設定したのにサマリに見つからないキーは、綴りではなく
親についての問いだというわけです。

## そのバイナリが実際に持つ一覧

ここの表が扱うのはフレームワークと認証プラグインです。あなたのビルドには自分の
パッケージが登録したものも入っており、その和集合を知っているのはバイナリだけです。

```sh
./myapp --generate-config toml > config.dev.toml
./myapp --generate-config env > .env
```

ひな形はそのビルドに存在する登録から組み立てられるので、そのバイナリにとっては
これが正典です。自分の `[app]` prefix は含まれ、インポートしていないフレームワーク
機能は含まれません。構造体を足してコマンドを再実行すれば新しいキーが現れます。
このページに手を入れる必要はありません。
[カスタムコマンド](/ja/guides/architecture/custom-commands/)を参照してください。
