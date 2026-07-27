---
title: pw generate
description: テンプレート、SQL、バインディングの呼び出し箇所を Go にコンパイルする。
sidebar:
  order: 2
---

```sh
pw generate [--check]
```

すべての `.pw.html` と `.pw.sql` ソースを**その隣**の `_pw_gen.go` に変換し、
ドキュメント登録パッケージを main パッケージにリンクします。出力するのは変更された
パスだけです。

## オプション

| オプション | 効果 |
| --- | --- |
| `--check` | 何も書かず、古いファイルがあれば列挙して非ゼロで終了する |

## 読み込む対象

生成はプロジェクトツリー全体を走査し、`.go`、`.pw.html`、`.pw.sql` の**いずれかを含む
すべてのディレクトリ**を処理します。`.git`、`vendor`、`node_modules`、`.devbox` は
除外されます。保守すべきパッケージのリストはありません。ディレクトリを作れば十分です。

テンプレートファイルに加えて、Go のソースから呼び出し箇所も読み取ります。

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
[テンプレート](/ja/guides/templates/)を参照。

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
