---
title: pw dev
description: 開発ループ。サービス起動、生成、マイグレーション、CSS、変更時の再起動。
sidebar:
  order: 5
---

```sh
pw dev
```

日常的に使うコマンドです。開発ループはプロジェクトファイルで定義されるため、引数は
取りません。

## 起動時にすること

1. `devbox.json` に宣言された Devbox サービスを起動する
2. [`pw generate`](/ja/pw/project/generate/) を実行する
3. `migration.auto` が `false` でなければ、未適用のマイグレーションを適用する
4. Tailwind が有効なら、スタイルシートをビルドしてウォッチャを起動する
5. `dev.idp.enabled` が `true` なら、開発用の認証プロバイダを起動する
6. `dev.otel.enabled` が `false` でなければ、テレメトリビューアを起動する
7. `project.main` をビルドして実行する

起動後は 0.5 秒ごとに監視対象を確認します。変更があれば、環境全体ではなく、その
ファイルに関係するステップだけを繰り返します。

## 監視する対象

- プロジェクト自身の Go、`.pw.html`、`.pw.sql` のソース
- マイグレーションディレクトリ
- Tailwind が有効な場合はその入力ファイル
- `popcornwave.toml` の `dev.watch.includes` に一致するもの

走査の範囲は `[generate]` の用途ではなくモジュール全体です。どの用途も生成に使わない
ファイルを含め、Go のソースはすべて再ビルドの入力だからです。`.git`、`vendor`、
`node_modules`、`.devbox`、`public` ツリーは常にスキップされます。

`dev.watch.includes` は、走査が届かない入力を加えるための相対 glob パターンです。
`dev.watch.excludes` はサブツリーをスキップします。大きな依存ツリーが走査をループ中で
最も遅いステップにしてしまうときに使います。どちらも絶対パスは拒否されます。

```toml
[dev.watch]
includes = ["config.dev.toml", "assets/**/*.svg"]
excludes = ["web/node_modules"]
```

## サービス

`devbox.json` に宣言されたサービス（既定では Valkey）は、Devbox のプロセスマネージャの
全画面 TUI を無効にした状態で動きます。ログは画面を覆い隠すのではなく、コード生成・
マイグレーション・アプリケーションの出力と同じストリームに、サービス名つきの 1 行ずつ
流れます。

```
[valkey	] 1:M 27 Jul 2026 23:02:32.103 * Ready to accept connections tcp
```

サービスが不要なプロジェクトは `devbox.json` からパッケージを外してください。`pw dev` が
起動するのは Devbox が宣言したものだけです。

## Tailwind

開発中のウォッチャは `assets.tailwind.minify` の設定に関わらず、常に**非 minify** の
CSS を作ります。minify がループの中で最も遅い部分だからです。CSS ウォッチャが失敗しても
サーバーは停止しません。`pw dev` は動き続け、入力ファイルを直接監視する方式へ
フォールバックします。

`tailwindcss` は `PATH` 上にある必要があります。そのための `devbox shell` です。
[スタイリング](/ja/guides/frontend/styling/)を参照。

## マイグレーション

未適用のマイグレーションはアプリケーションの起動前に適用され、マイグレーション
ディレクトリのファイルが変わったときにも適用されます。自分で制御したい場合は無効に
できます。

```toml
[migration]
auto = false
```

## 開発用の認証プロバイダ

`dev.idp.enabled` が true のとき、`pw dev` はローカルの OpenID Provider を起動し、
issuer と資格情報をアプリケーションのプロセスに注入し、ループと一緒に停止します。
ユーザー定義ファイルを編集すると、再起動なしでリロードされます。

```toml
[dev.idp]
enabled = true
```

ユーザー定義ファイルの形式、claim、このプロバイダが実装するもの・しないものは
[開発用の認証プロバイダ](/ja/productivity/dev-identity-provider/)を参照してください。

## テレメトリビューア

`pw dev` はループバックの OpenTelemetry レシーバとブラウザ UI も起動し、標準の OTLP
環境変数でアプリケーションをそこへ向けます。既定で有効なので、コレクタを用意しなくても
トレースと関連づいたログレコードが読めます。

```
pw dev: telemetry viewer http://127.0.0.1:54321
```

```toml
[dev.otel]
enabled = true
```

`OTEL_EXPORTER_OTLP_ENDPOINT` がすでに設定されていれば何も起動せず、自前のコレクタに
任せます。[開発用テレメトリビューア](/ja/productivity/dev-telemetry-viewer/)を参照して
ください。

## コンソール

`pw dev` はアプリケーションと並んで、固定のループバックポートでブラウザ向けの
コンソールを配信します。ループに必要なペインがここに集まります。プロジェクトの状態、
静的ファイル、データベースと宣言済みクエリ、テンプレートの storybook、`pw doctor`、
そして上記のテレメトリビューアです。

```
pw dev: console http://127.0.0.1:18081
```

```toml
[dev.console]
enabled = true
port = 18081
```

ループの他の部分はここを通して読みます。そしてそのどれもリリースビルドには存在
しません。[開発コンソール](/ja/productivity/dev-console/)を参照してください。

## 停止

`Ctrl-C` はループ全体をキャンセルし、アプリケーション、Tailwind のウォッチャ、
Devbox のサービスを停止します。一方、アプリケーション自身がエラー終了した場合は、
`pw dev` が `application exited: …` と報告して停止します。維持できないプロセスを
再起動し続けることはありません。
