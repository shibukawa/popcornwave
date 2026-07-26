---
title: pw dev
description: 開発ループ。サービス起動、生成、マイグレーション、CSS、変更時の再起動。
sidebar:
  order: 3
---

```sh
pw dev
```

日常的に使うコマンドです。引数は取りません。

## 起動時にすること

1. `devbox.json` に宣言された Devbox サービスを起動する
2. [`pw generate`](/ja/pw/project/generate/) を実行する
3. `migration.auto` が `false` でなければ、未適用のマイグレーションを適用する
4. Tailwind が有効なら、スタイルシートをビルドしてウォッチャを起動する
5. `project.main` をビルドして実行する

そのあとは 0.5 秒ごとに監視対象を確認し、変更があれば該当するステップを繰り返します。

## 監視する対象

- プロジェクト自身の Go、`.pw.html`、`.pw.sql` のソース
- マイグレーションディレクトリ
- Tailwind が有効な場合はその入力ファイル
- `popcornwave.toml` の `dev.extra_watch` に一致するもの

`dev.extra_watch` は相対の glob パターンを取ります。絶対パスは拒否されます。

```toml
[dev]
extra_watch = ["config.dev.toml", "assets/**/*.svg"]
```

## Tailwind

開発中のウォッチャは `assets.tailwind.minify` の設定に関わらず**非 minify** で動きます。
minify がループの中で最も遅い部分だからです。ウォッチャが終了しても `pw dev` は動き
続け、入力ファイルを直接監視する方式にフォールバックします。CSS のプロセスが落ちても
サーバーまで巻き込まれることはありません。

`tailwindcss` は `PATH` 上にある必要があります。そのための `devbox shell` です。
[スタイリング](/ja/guides/styling/)を参照。

## マイグレーション

未適用のマイグレーションはアプリケーションの起動前に適用され、マイグレーション
ディレクトリのファイルが変わったときにも適用されます。自分で制御したい場合は無効に
できます。

```toml
[migration]
auto = false
```

## 停止

`Ctrl-C` で実行がキャンセルされ、アプリケーション、Tailwind のウォッチャ、Devbox の
サービスが停止します。アプリケーションが自分でエラー終了した場合、`pw dev` は
`application exited: …` と報告して停止します。起動できないプロセスをループし続ける
ことはありません。
