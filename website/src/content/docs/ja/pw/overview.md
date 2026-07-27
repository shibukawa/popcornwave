---
title: 概要
description: pw 開発ツールの役割と、各コマンドの位置づけ。
---

1 つのコマンドがプロジェクトのライフサイクルをつなぎます。`pw` は最初のファイルを
スキャフォールドし、テンプレートと SQL を Go へコンパイルし、開発ループを動かし、
データベースを管理して、リリースビルドを作ります。

```sh
pw help
```

```
Usage: pw <command>
Commands: init, generate, migrate, seed, build, dev
Migrate actions: status, version, up, up-by-one, up-to, down, down-to, create, validate, snapshot
Seed usage: pw seed [--dir=testdata/seed] [name...]
```

インストールは次のとおりです。

```sh
go install github.com/shibukawa/popcornwave/cmd/pw@latest
```

## コマンド一覧

### プロジェクト

| コマンド | 用途 |
| --- | --- |
| [`pw init`](/ja/pw/project/init/) | 動くプロジェクトを作る |
| [`pw generate`](/ja/pw/project/generate/) | `.pw.html` と `.pw.sql` を Go にコンパイルする |
| [`pw dev`](/ja/pw/project/dev/) | 監視、再生成、マイグレーション、再起動 |
| [`pw build`](/ja/pw/project/build/) | リリース用バイナリを作る |

### データベース

| コマンド | 用途 |
| --- | --- |
| [`pw migrate`](/ja/pw/database/migrate/) | マイグレーションの確認、適用、ロールバック |
| [`pw seed`](/ja/pw/database/seed/) | シードデータセットの読み込み |

## プロジェクトの探索

`pw init` 以外のコマンドにはプロジェクトが必要ですが、プロジェクトルートで実行する
必要はありません。`pw` は作業ディレクトリから上へたどって `popcornwave.toml` を
探すため、どのサブディレクトリからでも動きます。最上層まで見つからなければ
`popcornwave.toml not found` で失敗します。

## 終了ステータス

成功で `0`、コマンドの失敗で `1`、コマンドが与えられなかった場合は `2` です。エラーは
`pw:` を前置して標準エラー出力に書かれます。

## 混同しないように

デプロイするバイナリには別のコマンドラインがあります。設定オプション、設定
スキャフォールドの出力、アプリケーション定義のサブコマンドです。この境界は
[アプリケーション CLI](/ja/guides/application-cli/)で扱います。
