---
title: pw generate
description: テンプレート、SQL、バインディングの呼び出し箇所を Go にコンパイルする。
sidebar:
  order: 4
---

```sh
pw generate [--check]
```

テンプレート、SQL、ページツリー、型付きストア宣言を、**ソースの隣**の
`_pw_gen.go` に変換し、アプリケーションに必要なパッケージをリンクします。出力するのは
変更されたパスだけです。

## オプション

| オプション | 効果 |
| --- | --- |
| `--check` | 何も書かず、古いファイルがあれば列挙して非ゼロで終了する |

## 読み込む対象

生成の範囲は**用途ごと**です。各用途は読んでよいディレクトリを列挙し、それ以外は
読みません。

```toml
[generate]
handlers = ["handlers"]
templates = ["handlers", "templates"]
queries = ["queries"]
config = ["cmd/myapp"]
pages = ["pages"]
dynamo = []
firestore = ["entities"]
```

| 用途 | 読むもの | 生成するもの |
| --- | --- | --- |
| `handlers` | Go 中のルート登録、`pw.Parse`、レスポンス呼び出し | リクエストバインディング、JSON コーデック、OpenAPI フラグメント |
| `templates` | `.pw.html` | 型付きレンダラ。ドキュメントシェルとエラーページを探す場所でもある |
| `queries` | `.pw.sql` | context ベースのクエリ関数 |
| `config` | Go 中の `pw.RegisterConfig`、`pw.RegisterSubCommand` | 設定とサブコマンドのバインディング |
| `pages` | ページツリーのルート | 探索型ルーティングのルート登録とページパラメータ |
| `dynamo` | `dynamo` タグ付き Go 型と `.pw.dynamo` | レコードのコーデック、キー、型付き DynamoDB クエリ |
| `firestore` | `firestore` タグ付き Go 型と `.pw.firestore` | エンティティのコーデック、キー、Datastore mode の型付きクエリ |

1 つのディレクトリが複数の用途に現れて構いません。ページテンプレートはそれを描画する
ハンドラの隣に置くため、`handlers` は通常 `handlers` と `templates` の両方に現れます。
列挙したディレクトリは再帰的に走査されるので、入れ子のパッケージを個別に書く必要は
ありません。

以前からある `handlers`、`templates`、`queries`、`config` の 4 キーは必須で、既定値は
ありません。`pages`、`dynamo`、`firestore` は古いプロジェクトとの互換性のため省略できます。
それでも、その用途では何も生成しないという判断は空リストで明示するのが分かりやすい書き方です。

```toml
queries = []   # このプロジェクトに .pw.sql はない
```

この区別には意味があります。キーの書き忘れはエラーになり、`[]` は次に読む人に見える
判断として残ります。

自分の用途の外にあるソースはビルドを失敗させず、報告してスキップします。意図して置いた
サンプルやフィクスチャがコードの隣にあっても構わないためです。

```
pw: samples/home.pw.html is outside generate.templates and is not generated from; list its directory to include it
```

Go のソースは報告しません。普通の Go コードはプロジェクト全体にあり、用途の外にある
呼び出し箇所は単にバインディングが生成されないだけだからです。以前のレイアウトが
どの用途にも属さない場所に残した `_pw_gen.go` は報告します。もう再生成も削除も
されないファイルだからです。

宣言ファイルに加えて、Go のソースから呼び出し箇所も読み取ります。

| 呼び出し | 生成されるもの |
| --- | --- |
| `pw.Parse[T]` | `T` のリクエストバインディング |
| `pw.WriteAPI[T]` | `T` の JSON エンコーディング |
| `pw.NewStream[T]` | `T` のストリームエンコーディング |
| `pw.RegisterConfig[T]` | `T` の設定バインディング |
| `pw.RegisterSubCommand[T]` | `T` のサブコマンドのパース |
| `pw.BadRequest` などのエラーコンストラクタ | ドキュメント化されるエラーレスポンス |

同じ根拠から、パッケージごとに 1 つの OpenAPI 3.1 フラグメントも生成されます。
ビルド時に決定的にマージされるため、API 記述は別のアノテーション群ではなくコードに
追従します。

## 書き出すもの

生成された Go はソースではなくビルド生成物です。

- ファイル名は `{ソース名}_pw_gen.go` で、常にソースの隣に置かれる
- スキャフォールドされた `.gitignore` によりバージョン管理から除外される
- `.vscode/settings.json` によりエディタから隠される
- アプリケーションのビルドのたびに作り直される

いつでも再生成できます。編集もコミットもしないでください。

`cmd/<name>/popcornwave_bootstrap_pw_gen.go` は種類として例外です。ドキュメントシェルと
埋め込み公開アセットをバイナリにリンクするためのブランクインポートだけを含む生成
ファイルで、これによりハンドラがそれらを参照する必要がなくなります。どちらも存在しない
場合は自動的に削除されます。

## ドキュメント 1 つの規則

プロジェクトに `document.pw.html` はちょうど 1 つです。ツリーのどこかに 2 つ以上あると
次のように失敗します。

```
pw: multiple default documents: templates/document.pw.html, admin/document.pw.html
```

別のシェルは名前なしスロットを持つ通常のエクスポート済みコンポーネントとして書き、
ハンドラごとに `pw.WriteHTMLChain` で選択してください。
[テンプレート](/ja/guides/frontend/templates/)を参照。

## CI での使い方

```sh
pw generate --check
```

生成された Go は Git 管理外なので、リポジトリの差分では CI で古さを検出できません。
`--check` はメモリ上で再生成し、変わるファイルがあれば失敗します。

```
pw: generated files are stale:
  handlers/home_pw_gen.go
```

[`pw dev`](/ja/pw/project/dev/) と [`pw build`](/ja/pw/project/build/) はどちらも
最初に生成します。直接呼び出すのは、主に CI や生成エラーの診断時です。
