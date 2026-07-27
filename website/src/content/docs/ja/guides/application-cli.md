---
title: アプリケーション CLI
description: ビルドしたバイナリのコマンドライン。設定オプション、スキャフォールド出力、独自のサブコマンド。
sidebar:
  order: 8
---

`pw` は開発を制御しますが、利用者がデプロイするコマンドではありません。`pw build`
が生成するバイナリには独自の CLI があり、設定を駆動するのと同じ型付き宣言から
生成されます。

開発ツール自体については [pw コマンド](/ja/pw/overview/)を参照してください。

## すべての設定はオプションでもある

登録済みの各設定キーは 3 つの方法で指定でき、優先度は次の順に上がります。

```
既定値  <  TOML ファイル  <  環境変数  <  コマンドラインオプション
```

既定の名前は prefix とフィールド名から導出されます。

| prefix + フィールド | キー | オプション | 環境変数 |
| --- | --- | --- | --- |
| `app` + `EnvLabel` | `app.env_label` | `--app-env_label` | `APP_ENV_LABEL` |
| `middleware.rdb` + `DSN` | `middleware.rdb.dsn` | `--middleware-rdb-dsn` | `MIDDLEWARE_RDB_DSN` |

フィールド側でいずれも上書きできます。

```go
type ServerConfig struct {
	Port int `key:"listen_port" default:"8080" opt:"port,p" env:"PORT" help:"HTTP listen port"`
}
```

| タグ | 効果 |
| --- | --- |
| `default:"value"` | 他のどこからも値が来なかったときの値 |
| `key:"name"` | TOML や設定のキー |
| `opt:"long"` / `opt:"long,s"` | 長い形式のオプションと、任意の 1 文字の短縮形 |
| `env:"NAME"` | 環境変数名を明示する。`env:"-"` は環境変数入力を無効にする |
| `help:"text"` | usage やスキャフォールドに表示される説明 |

`opt` が長い形式を上書きした場合、環境変数名はそちらから導出されます。

```sh
./myapp --port=9090
PORT=9090 ./myapp
```

## 設定ファイルの選択

`APP_ENV` がどのプロジェクトローカルファイルを読むかを選び、`--config-path` は探索
そのものを上書きします。

```sh
APP_ENV=stg ./myapp
./myapp --config-path ./deploy/staging.toml
```

解決順序の全体は[設定](/ja/guides/configuration/)を参照してください。

## スキャフォールドの出力

```sh
./myapp --generate-config toml > config.dev.toml
./myapp --generate-config env > .env
```

どちらの形式も、フレームワークかアプリケーションかを問わず、**登録済みすべての
prefix** を出力し、`default` 値と `help` テキストをコメントとして残します。バイナリが
報告するのは、実際にリンクされたパッケージの登録内容です。設定を持つ依存を追加すれば、
次にスキャフォールドを生成したときにその設定も現れます。

いずれの形式も出力後に終了し、サーバーは起動しません。

## 独自のサブコマンド

サブコマンドに必要なのは、構造体 1 つと登録呼び出し 1 つだけです。`pw generate` が
呼び出し箇所を読んでパーサーを書き出すため、別にフラグの配線を保守する必要はありません。

```go
package main

type importCommand struct {
	Path   string   `arg:"required" help:"CSV file to import"`
	Label  string   `arg:"optional"`
	Tags   []string `arg:"*"`
	DryRun bool     `default:"false" help:"parse without writing"`
}
```

| タグ | 意味 |
| --- | --- |
| `arg:"required"` | 必須の位置引数 |
| `arg:"optional"` | 省略可能な位置引数 |
| `arg:"*"` | 可変長の位置引数（0 個以上） |
| *(`arg` タグなし)* | オプション。設定フィールドと同じタグが使える |

設定がパースされる前に登録し、そのあとディスパッチします。

```go
func main() {
	handlers.RegisterConfig()
	pw.RegisterSubCommand[importCommand]("import", "import a CSV file")

	if err := pw.ParseConfig(); err != nil {
		log.Fatal(err)
	}
	if command, ok := pw.Command[importCommand](); ok {
		if err := runImport(context.Background(), command); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

```sh
./myapp import ./data/users.csv --dry-run
```

`pw.Command[T]` はそのサブコマンドが選択されたときにだけ `true` を返すので、`if` 文が
そのままディスパッチになります。入力を受け取るのは選ばれたサブコマンドだけです。
`pw.ParseConfig` を自分で呼んでも安全です。`pw.Run` も呼びますが、パースは 1 回しか
行われません。

登録の順序が重要なのは設定と同じ理由です。生成された定義はパッケージの `init` で登録
されるため、`RegisterSubCommand` はすべての `init` の後、`ParseConfig` の前に実行する
必要があります。パース後の登録は panic します。

サブコマンドはサーバーのパース済み設定を共有します。そのため `pw.Config[T]` は、
DSN を含めてサーバーと同じ値を返し、同期すべき 2 つ目の設定経路を作りません。

データベースプールは別です。これは `ParseConfig` ではなく `pw.Run` や `pw.Middlewares`
が開きます。接続が必要なサブコマンドは、設定された DSN から自分で開いてください。
