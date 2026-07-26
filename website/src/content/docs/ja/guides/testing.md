---
title: テスト
description: testutil で、隔離された設定コピーから実際のアプリケーションを起動する。
sidebar:
  order: 9
---

`github.com/shibukawa/popcornwave/testutil` は、登録済みのすべての設定をコピーした状態で
実際のアプリケーションを起動します。ミドルウェアスタックも設定バインディングもルートも
本物です。テストは手で組み立てた部分集合ではなく、実物を HTTP 越しに検証します。

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

`TestRun` は登録済みのすべての設定をコピーし、コピー側のポートを `-1` にし、カスタマイザ
を適用し、利用可能なループバックポートを確保し、コピーしたランタイムリソースを初期化して
サーバーを起動します。クリーンアップは `t` に登録されるので、defer は不要です。

ポートの扱いは、ポートを読むカスタマイザを書くときに効いてきます。カスタマイザの時点では
`-1` が見えます。実際のポートはその後に決まるためです。`server.URL` か `server.Port` を
読んでください。

## 生成パッケージの登録

生成された登録処理 —— ドキュメントシェルや設定定義 —— はパッケージの `init` で走るので、
テストバイナリにリンクする必要があります。

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

サーバーの起動前に、コピーされたデータベースへマイグレーションを適用します。
`WithMigrationsFS` は代わりに `fs.FS` を取るので、埋め込みマイグレーションに使えます。

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

これは `pw seed` が適用するのと同じファイルなので、フィクスチャが CLI とテストスイートの
間でずれることはありません。

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

## データベースに対するアサーション

`server.Context()` は、サーバーがリクエストに設定するのと同じランタイムリソースを持つ
context を返します。`WithTransaction` のトランザクションも含まれるので、生成された
クエリでハンドラと同じトランザクションの中のデータを準備したり検証したりできます。

```go
counter, err := queries.CurrentAccess(server.Context())
```

生の SQL が必要な場合は `server.DB` がプールを直接公開しています。
