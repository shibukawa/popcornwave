---
title: 設定サマリ
description: どの設定値が実際に効いたのか、それぞれがどこから来たのかを、起動 1 回につき 1 回だけ確認する。
sidebar:
  order: 5
---

解決された設定は、キーごとに 1 行ではなく**まとめて 1 回**だけ報告されます。形式は
読み手によって変わります。対話的な端末では木構造で表示され、最後にリッスンを開始した
アドレスが続きます。

```
   .-.   .-.
 .(   ) (   ).    Popcorn Web v0.1.0
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

最後の行はリスナーが実際に受け付けたアドレスで、その上の `server.port` と一致するとは
限りません。開発時の実行は bind できなかったポートから移るため、サマリは設定された値と
応答するアドレスの両方を残します。[`pw dev`](/ja/pw/project/dev/#ポート) を参照して
ください。

それ以外の場所 —— パイプ、コンテナ、ログコレクタ —— では同じ情報が 1 レコードの構造化
ログになります。JSON ハンドラや OpenTelemetry ブリッジには 60 件ではなく 1 件のイベント
だけが流れます。

```json
{"time":"2026-07-27T23:31:04+09:00","level":"INFO","msg":"popcornweb started",
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

## `pw dev` での再起動

開発ループでは「プロセスごとに 1 回」は「リビルドごとに 1 回」です。テンプレートを 1 つ
保存しただけで 40 行が出力し直されれば、読んでいたものは画面から押し出されます。そこで
[`pw dev`](/ja/pw/project/dev/) はアプリケーションが出力したサマリを読み取り、次のサマリを
それとの差として報告します。設定が前回と変わらなかった再起動は、そう言って終わります。

```
reloaded
```

変わっていれば、動いた行だけを、それが属するセクションごと表示します。

```
└─ html
   └─ bot_async_timeout  5s → 10s  ← file
```

現れたキーと消えたキーには `← added`、`← removed` が付きます。`html.bot_detection` を
off にしたときがこれで、それが制御している設定はレポートから丸ごと消えます。値は同じで
勝った層だけが変わったキーは `← default → env` と読めます。値がどこから来たかを報告するのと
同じ理由で、そこが変わったことも報告されるからです。リッスンしているアドレス、環境、設定
ファイル、フレームワークのバージョンもキーと並べて比較されるので、bind できずにポートが
移った実行は無言ではなく 1 行になります。

セッション最初のサマリは全文が出ます。まだ何も読んでいない相手には、短くして真になる
答えがありません。`record` と `off` はそのままです。コレクタは重複を除きますし、サマリを
off にした人が代わりに reloaded の 1 行を求めたわけでもありません。

## ログ中の秘密情報

秘密の値はどちらの形式でもマスクされます。通常の値は `*****` になり、DSN は資格情報と
クエリ文字列を除いた公開部分だけを残します。名前による判定の正確な規則と、明示的な
`secret` の上書きは[起動サマリに出るもの](/ja/reference/configuration-declaration/#起動サマリに出るもの)
にあります。

値の取得元については[アプリケーション設定](/ja/guides/architecture/configuration/)を、同じ考え方を SQL に
適用したものについては[スロークエリー診断](/ja/productivity/query-diagnostics/)を
参照してください。
