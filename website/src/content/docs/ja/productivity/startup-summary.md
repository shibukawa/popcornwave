---
title: 起動サマリ
description: どの設定値が実際に効いたのか、それぞれがどこから来たのかを、起動 1 回につき 1 レコードで。
sidebar:
  order: 4
---

解決された設定は、キーごとに 1 行ではなく**まとめて 1 回**だけ報告されます。形式は
読み手によって変わります。対話的な端末では木構造で表示され、最後にリッスンを開始した
アドレスが続きます。

```
   .-.   .-.
 .(   ) (   ).    Popcorn Wave v0.1.0
(   o     o   )   started at 2026-07-27 23:31:04 JST
(    \___/    )   env dev · config.dev.toml
 '-.__.___.__-'

configuration
├─ middleware
│  ├─ access_log       true
│  ├─ compression      true  ← file
│  └─ request_timeout  0s
├─ server
│  ├─ port          8080
│  └─ read_timeout  30s
└─ session
   └─ enabled  false

listening on http://localhost:8080
```

既定値以外から来た値だけが `← file`、`← env`、`← flag` と印されます。

それ以外の場所 —— パイプ、コンテナ、ログコレクタ —— では同じ情報が 1 レコードの構造化
ログになります。JSON ハンドラや OpenTelemetry ブリッジには 60 件ではなく 1 件のイベント
だけが流れます。

```json
{"time":"2026-07-27T23:31:04+09:00","level":"INFO","msg":"popcornwave started",
 "environment":"dev","config_file":"config.dev.toml",
 "listening":"http://localhost:8080",
 "config":{"server":{"port":"8080"},"session":{"enabled":"false"}},
 "config_source":{"middleware.compression":"file"}}
```

`observability.boot_log` で選択を上書きできます。

| 値 | 動作 |
| --- | --- |
| `auto`（既定） | 端末なら木構造、それ以外は 1 レコード |
| `tree` | 常に木構造を stderr に出力 |
| `record` | 常に既定の `slog` ロガーへ 1 レコード |
| `off` | 起動サマリを出力しない |

アプリケーションがリスナーを持つ場合 —— `pw.Run` ではなく `pw.Middlewares` を使う場合
—— サマリは初期化後に出力され、`listening` の行は付きません。

## ログ中の秘密情報

キー名に `secret`、`password`、`token`、`credential`、`dsn`、`private_key` を含むものは、
どちらの形式でも `[REDACTED]` として記録されます。

値の取得元については[設定](/ja/guides/architecture/configuration/)を、同じ考え方を SQL に
適用したものについては[クエリー診断](/ja/productivity/query-diagnostics/)を
参照してください。
