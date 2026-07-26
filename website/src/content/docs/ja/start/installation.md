---
title: インストール
description: pw コマンドを導入し、Popcorn Wave を Go モジュールに追加する。
sidebar:
  order: 1
---

Popcorn Wave は **Go 1.26 以降**が必要です。

## `pw` コマンド

スキャフォールド、コード生成、マイグレーション、開発サーバまで、ほとんどの操作は
`pw` コマンドを経由します。

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

`pw init` はフレームワークを require 済みの `go.mod` を書き出すので、スキャフォールド
したプロジェクトに手動の `go get` は不要です。既存モジュールに追加する場合は次のように
します。

```sh
go get github.com/shibukawa/popcornwave
```

アプリケーションコードは、安定したアプリケーション向け API である
[`pw`](/ja/guides/handlers/) パッケージをインポートします。

```go
import "github.com/shibukawa/popcornwave/pw"
```

## Devbox（任意）

生成されるプロジェクトには Go と Valkey サービスをピン留めした `devbox.json` が付属し、
`pw init --tailwind` を使うと標準の Tailwind CSS バイナリも追加されます。Devbox は便利
ですが必須ではありません。Go が `PATH` にあれば `devbox shell` を飛ばして `pw dev` を
直接実行できます。

Devbox は [jetify.com/devbox](https://www.jetify.com/devbox/) から導入できます。

## 次のステップ

- [はじめる](/ja/start/getting-started/) — 最初のプロジェクトを作って動かす。
