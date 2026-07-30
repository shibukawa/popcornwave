---
title: インストール
description: pw コマンドを導入し、Popcorn Wave を Go モジュールに追加する。
sidebar:
  order: 1
---

Popcorn Wave は **Go 1.26 以降**が必要です。その先で必須となる準備は、`pw` コマンドと、
それが扱うライブラリ依存だけです。

## `pw` コマンド

スキャフォールド、コード生成、マイグレーション、開発サーバは、いずれも `pw`
コマンドを経由します。まずはこれを入れます。

```sh
go install github.com/shibukawa/popcornwave/cmd/pw@latest
```

```sh
pw help
```

```
Usage: pw <command>
Commands: init, generate, migrate, seed, build, dev
Migrate actions: status, version, up, up-by-one, up-to, down, down-to, create, validate, snapshot
Seed usage: pw seed [--dir=testdata/seed] [name...]
```

## ライブラリ

新しいプロジェクトでは、`pw init` がフレームワークを require 済みの `go.mod` を
書き出すため、手動の `go get` は不要です。既存モジュールには 1 つだけ手順を追加します。

```sh
go get github.com/shibukawa/popcornwave
```

アプリケーションコードは、安定したアプリケーション向け API である
[`pw`](/ja/guides/handlers/) パッケージをインポートします。

```go
import "github.com/shibukawa/popcornwave/pw"
```

## Devbox（任意）

生成されるプロジェクトには、Go と Valkey サービスをピン留めした `devbox.json` が
付属します。`pw init --tailwind` を使った場合は、スタンドアロンの Tailwind CSS
バイナリもピン留めされます。Devbox はツールの再現性を保ちますが必須ではありません。
Go が `PATH` にあれば、`devbox shell` を飛ばして `pw dev` を直接実行できます。

Devbox は [jetify.com/devbox](https://www.jetify.com/devbox/) から導入できます。

## 次のステップ

- [はじめる](/ja/start/getting-started/) — 最初のプロジェクトを作って動かす。
