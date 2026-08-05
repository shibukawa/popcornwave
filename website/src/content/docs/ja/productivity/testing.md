---
title: テスト
description: testutil で、隔離された設定コピーから実際のアプリケーションを起動する。
sidebar:
  order: 1
---

高速なテストも、デプロイするアプリケーションを実際に検証してこそ役立ちます。
`github.com/shibukawa/popcornwave/testutil` は、本物のルート、ミドルウェアスタック、
設定バインディングを、登録済み設定の隔離されたコピーに対して起動します。テストは
手で組み立てた近似物を呼ぶのではなく、そのアプリケーションへ HTTP で到達します。

## 最初のテスト

```go
func TestHome(t *testing.T) {
	server := testutil.TestRun(t, Handlers(), func(config *testutil.Config) {
		testutil.Update[pw.MiddlewareConfig](config, func(middleware *pw.MiddlewareConfig) {
			middleware.RDB = pw.RDBConfig{
				Enabled:        true,
				DSN:            "sqlite://:memory:",
				ConnectTimeout: time.Second,
				MaxOpenConns:   1,
				MaxIdleConns:   1,
			}
		})
	}, testutil.WithMigrations("../migrations"))

	response, err := server.Client().Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
}
```

`TestRun` はまず登録済みのすべての設定をコピーし、コピー側のポートを `-1` にします。
次にカスタマイザを適用し、利用可能なループバックポートを確保し、コピーしたランタイム
リソースを初期化してサーバーを起動します。クリーンアップは `t` に登録されるため、
テスト側で defer するものはありません。

この順序から 1 つの制約が生まれます。カスタマイザがポートを読むと、その時点では
`-1` が見えます。実際のポートは後で決まるため、起動後に `server.URL` か
`server.Port` を使ってください。

## 生成パッケージの登録

本物のサーバーを使う以上、生成された登録処理も本物でなければなりません。ドキュメント
シェルや設定定義はパッケージの `init` で登録されるため、そのパッケージをテスト
バイナリにリンクする必要があります。

```go
import (
	_ "myapp"            // public.go
	_ "myapp/templates"  // ドキュメントシェル
)
```

これがないと、ドキュメントが登録されていない状態でサーバーが起動し、HTML の描画が起動時
に失敗します。

## 設定のカスタマイズ

| 呼び出し | 用途 |
| --- | --- |
| `testutil.Get[T](config)` | コピーされた設定構造体を読む |
| `testutil.Set(config, value)` | まるごと置き換える |
| `testutil.Update[T](config, fn)` | その場で編集する |

いずれも設定の型に対してジェネリックなので、フレームワークの設定もアプリケーションの
設定も同じ型付きの方法で扱えます。

```go
testutil.Update[AppConfig](config, func(app *AppConfig) {
	app.EnvLabel = "test"
})
```

## オプション

### `WithMigrations` / `WithMigrationsFS`

```go
testutil.WithMigrations("../migrations")
```

サーバーの起動前にマイグレーションを適用します。`WithMigrationsFS` は代わりに
`fs.FS` を取るので、埋め込みマイグレーションに使えます。

スキーマの届き方は DSN のエンジンによって変わります。

- **SQLite** はキャッシュしたスナップショットをコピー先のデータベースへ再生します。
  `sqlite://:memory:` が動くのはこのためです。プロセス内のデータベースには DSN で
  到達できないので、接続文字列ではなく SQL を渡しています。
- **PostgreSQL と MySQL** は設定されたデータベースへ直接適用します。同じデータベースに
  対する 2 回目の `TestRun` は何も適用せずスキーマを再利用するので、パッケージ内の
  テストが 1 つの準備済みサーバーを共有できます。

サーバーの DSN は、テストスイート専用のデータベースに向けてください。適用済み
バージョンは番号で記録されるため、他のプロジェクトのバージョン 1 をすでに持つ
データベースでは、最初のマイグレーションが適用済みに見えてスキーマが届きません。

### `WithSeed` / `WithSeedDir`

```go
testutil.WithSeed("initial")
```

スキーマの適用後、サーバーの起動前にデータセットを読み込みます。名前はシード
ディレクトリ —— 既定は `testdata/seed`、`WithSeedDir` で変更可能 —— からの相対で、
`.yaml` 拡張子は省略できます。データセットは指定した順に適用されます。

データセットのファイルは、テーブル名に行を対応させた形です。

```yaml
member:
- { id: 1, name: Frank }
- { id: 2, name: Grace }
```

同じファイルを `pw seed` も使います。CLI とテストスイートが別々のフィクスチャを持って
ずれる代わりに、1 つの形式を共有します。同じファイルを期待状態としても使う方法は
[フィクスチャ](#フィクスチャ)、形式そのものは
[シードデータ](/ja/productivity/seed-data/)を参照してください。

### `WithTransaction`

```go
testutil.WithTransaction(true)
```

テストサーバーのすべてのリクエストを 1 つのトランザクションの中で実行し、テスト終了時に
ロールバックします。1 つのデータベースを共有するテストどうしが独立を保ち、並列実行も
できます。アプリケーション自身が開始するトランザクションはセーブポイントとしてその中に
ネストするため、セーブポイント対応のドライバが必要です。

### `WithIdentityProvider`

```go
server := testutil.TestRun(t, handlers.Handlers(), nil, testutil.WithIdentityProvider(
	testutil.WithIdPConfig("../devidp.toml"),
	testutil.WithLoginUser("admin"),
	testutil.WithIdPBinding(func(config *testutil.Config, idp testutil.IdPInfo) {
		testutil.Update[handlers.AuthConfig](config, func(auth *handlers.AuthConfig) {
			auth.Issuer, auth.ClientID, auth.ClientSecret = idp.Issuer, idp.ClientID, idp.ClientSecret
		})
	}),
))
```

[`pw dev`](/ja/pw/project/dev/) と同じ開発用認証プロバイダを、専用のループバック
ポートでアプリケーションサーバーより先に起動します。`WithLoginUser` でログインする
ユーザーを事前に指定すると、認可エンドポイントは即座に認可コードを付けてリダイレクト
するので、ブラウザを操作せずにログインが完結します。

```go
response, err := server.Client().Get(server.URL + "/login")
```

ユーザー定義は `WithIdPConfig`（`devidp.toml` ファイル）、`WithIdPRoster`（同じ
TOML をテスト内に持つ）、`WithIdPUsers`（Go の値）のいずれか 1 つだけを指定します。
`WithIdPBinding` は issuer と生成されたクライアント資格情報をコピー済みの設定へ
書き込みます。`customize` の後に走るので、置いておいたプレースホルダより優先されます。
テストの途中でユーザーを切り替えるには `server.LoginAs(t, "guest")`、クライアントを
自前で組み立てる場合は `server.IdPInfo()` から同じ値を取得できます。

## フィクスチャ

データセットは、テストがそれを既知の状態として扱った瞬間から**フィクスチャ**に
なります。同じファイルが両端で働きます。リクエストの前には開始状態として読み込まれ、
その後はデータベースと突き合わせる期待状態になる。その後ろ半分が `server.AssertDB`
です。

```go
func TestArchiveRemovesTheMember(t *testing.T) {
	server := testutil.TestRun(t, Handlers(), nil,
		testutil.WithMigrations("../migrations"),
		testutil.WithSeed("initial"),
	)

	response, err := server.Client().Post(server.URL+"/members/2/archive", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	server.AssertDB(t, "after_archive")
}
```

期待値を `SELECT` の羅列ではなくファイルとして書くと、テーブル全体がテストの対象に
入ります。目的のメンバーを正しくアーカイブし、ついでに別人の行まで消してしまう
ハンドラは、ここで落ちます。`after_archive.yaml` はその行が変わってよいとは
言っていないからです。そしてこれは、メンバー 2 だけを `SELECT` するテストが素通り
させるバグでもあります。

不一致は `Fatalf` ではなく `Errorf` で報告されるので、テストはそのまま進みます。
1 回の実行で、最初の 1 テーブルだけでなく、ずれたテーブルがすべて出ます。

```
Popcorn Wave database does not match:
after_archive.yaml:
Table: member
- Expected
+ Actual
id: 1
  name: Frank
id: 2
- name: Grace
```

### テーブルの一部だけを比べる

既定では、テーブルはデータセットと完全に一致していなければなりません。ファイルが
書いていない行がデータベースにあれば失敗です。列はその限りではなく、ファイルが省いた
列は無視されます。連番の `id` や `created_at` を期待値から外しておけるのはこのため
です。

余分な「行」のほうが正当な場合は、テーブルごとに戦略を指定します。

```yaml
_match:
  member: exact          # 既定
  access_log_2026_*: sub # 余分な行は許す。並べた行は存在しなければならない

member:
- { id: 1, name: Frank }
```

既定の `exact` を使ってください。`sub` は、テストが制御していないところから行が来る
テーブル —— 監査証跡、追記専用のログ、同じパッケージの別のテストも書き込むテーブル
—— に限ります。普通のテーブルに `sub` を付けると、予期しない行を静かに捕まえなく
なります。テーブルごと比べる理由は、まさにそこにあったはずです。

そのままでは書き下せない値には、専用の形式があります。

| 値 | 一致するもの |
| --- | --- |
| `[null]` | NULL。`null` と書くのと同じ |
| `[notnull]` | NULL 以外であること |
| `[any]` | 任意の値 |
| `[currentdate, 2m]` | 現在からその期間内のタイムスタンプ。期間の既定は `1m` |
| `[regexp, ^User .+ logged in$]` | 文字列化した値がパターンに一致するもの |

```yaml
audit_log:
- { id: 1, created_at: [currentdate, 2m], message: [regexp, "^User .+ logged in$"] }
```

これがなければアサーションを Go 側へ追い出していた列 —— 生成されたタイムスタンプ、
識別子が埋め込まれたメッセージ —— が、ファイルの中に留まります。

### テストの途中で入れ直す

`server.Seed` は動いているサーバーにデータセットを適用します。1 つのテストのフェーズ
とフェーズの間で状態を戻すのに使えます。

```go
server.Seed(t, "initial")
```

ファイルに並んだテーブルは truncate されてから挿入し直されるので、前のフェーズが何を
していようと、データセットはそのテーブルを記述どおりの状態へ戻します。ファイルに出て
こないテーブルには触れません。

`WithTransaction` の下では、どちらのヘルパーもテストトランザクションの中で動きます。
2 つを組み合わせられるのはそのためです。`AssertDB` はリクエストがまだコミットして
いない書き込みを見ますし、`Seed` が入れた行はロールバックとともに消えます。付けない
場合、`AssertDB` が見るのはコミット済みの状態だけで、トランザクションを開いたままの
リクエストはまだ比較されていません。

### データセット形式のうち、ここでは効かない 2 つ

データセットには `_operation` ブロックを書けますし、dbtestify 自身のドキュメントも
それを説明しています。Popcorn Wave はこのブロックを解析したうえで無視します。シードは
常に clear-insert です。`insert` を要求したファイルも先に truncate するので、積み
上がるはずだった行は残りません。1 つのファイルで追記しようとせず、フェーズごとに
データセットを分けてください。

行の `_tag` による絞り込みも同じ形の制限を受けます。タグは解析されますが、それに作用
する include / exclude のフィルタを `WithSeed` も `Seed` も公開していないため、ファイル
内のすべての行が適用されます。部分集合が必要なら、ファイルを分けてください。

## データベースに対するアサーション

フィクスチャが比べるのはテーブル全体です。確認したいものがそれより狭いとき —— 1 つの
カウンタ、1 つの列、Go で計算したい値 —— は直接読んでください。HTTP アサーションは
クライアントが見た結果を示しますが、データベースのアサーションにはその結果を生んだ
ランタイム状態が必要になることがあります。`server.Context()` は、
`WithTransaction` のトランザクションを含む、リクエストと同じリソースを保持します。
そのため生成済みクエリから、ハンドラと同じトランザクション内のデータを準備・検証できます。

```go
counter, err := queries.CurrentAccess(server.Context())
```

生の SQL が必要な場合は `server.DB` がプールを直接公開しています。
