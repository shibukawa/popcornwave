---
title: テレメトリ
description: アプリケーションログ、トレース、開発診断、ローカルJSONL、DuckDB分析をPopcorn Waveがどう接続するか。
sidebar:
  order: 7
---

テレメトリは、性質の異なる二つの問いに答えます。ログは個々の判断や失敗を説明し、
トレースは一つのリクエストが処理と時間をどう通過したかを示します。Popcorn Wave は両者を
関連付けつつ、開発環境と本番環境に同じ出力先を強制しません。

## テレメトリのモデル

アプリケーションコードは [`pw.Logger(ctx)`](/ja/reference/runtime/#ロギング) から構造化
レコードを書きます。コンテキストは
実行中のトレース識別子を持つため、スパン内で出したレコードには `trace_id`、`span_id`、
`trace_flags` が自動的に入ります。フレームワークの診断も同じロガーを使います。メッセージは
人が読み、型を保った属性はクエリに使います。

```go
package handlers

import (
    "net/http"

    "github.com/shibukawa/popcornwave/pw"
)

func showAccount(w http.ResponseWriter, r *http.Request) {
    accountID := r.PathValue("id")
    pw.Logger(r.Context()).Info(
        "account requested",
        pw.String("account_id", accountID),
        pw.Bool("cached", false),
    )
    w.WriteHeader(http.StatusNoContent)
}
```

レベルは `trace` から `error` まであり、設定上の下限には `off` も使えます。値を
メッセージへ埋め込まず、`pw.String`、`pw.Int`、`pw.Bool` などのスカラー用
コンストラクタを使います。`timestamp`、`severity`、`message`、`service_name`、
`trace_id`、`span_id`、`trace_flags` は出力パイプラインの予約名であり、
アプリケーション属性から置き換えられません。

生成されたデータベース呼び出しは、[クエリ診断](/ja/productivity/query-diagnostics/)を通して
ステートメントと実行時間を記録できます。bind値のログは独立した機密性の高い設定で、
トレースとの関連付けに必要なものではありません。

パスワード、トークン、セッション値、不要な個人情報をメッセージや属性に入れないでください。
構造化保存は、誤って入れた秘密を安全にするのではなく、検索しやすくします。

## 開発環境の出力先

`pw dev` では、アプリケーションのレコードを読みやすいテキストとしてターミナルへ出し続けます。
開発テレメトリビューアが有効なら、関連付いたログとトレースも受信します。それとは独立して、
`pw dev` はアプリケーションログを既定でプロジェクト内の `.log` へ JSONL 形式で保存します。

一回の `pw dev` 起動につき `pw-dev-*.jsonl` が一つ対応します。再ビルドやアプリケーションの
再起動は同じファイルへ追記し、次の `pw dev` 起動では新しいファイルを使います。ディレクトリと
ファイルは最初のレコードで初めて作られるため、何も出力しない実行はファイルを残しません。
既存ファイルを切り詰めたり自動削除したりせず、新しい `pw init` プロジェクトでは `.log/` を
Git の対象外にします。既存プロジェクトは自身の `.gitignore` に `.log/` を追加してください。
ディレクトリとファイルは、OSの規則が許す範囲で所有者だけが使える権限で作られます。

```toml
[dev.logs]
enabled = true
directory = ".log"
```

ビューアには、独立したプロジェクト設定があります。

```toml
[dev.otel]
enabled = true
```

`directory` はプロジェクト内の相対パスでなければなりません。`enabled = false` にすると、
ターミナルと設定済みの OTLP 出力を維持したままローカルファイルだけを止めます。ファイル
システムのエラーはローカル保存だけを無効化し、一度診断を出します。アプリケーションは停止しません。

各 JSON 行には安定した `timestamp`、`severity`、`message`、`service_name` があります。
関連付いたレコードには `trace_id`、`span_id`、数値の `trace_flags` も入ります。
アプリケーション属性は型を保ったトップレベルフィールドなので、数値や真偽値をクエリに使えます。

## DuckDBでJSONLをクエリする

DuckDB は任意で導入する外部ツールであり、`pw` は同梱、インストール、実行のいずれも行いません。
データベースへ取り込まずに複数回の実行を横断して検索できます。プロジェクトルートで
実行するか、glob を設定済みディレクトリへ合わせてください。Popcorn Wave のエージェントSkillも
このスキーマを知っているため、「直近1時間で繰り返したエラーを見せて」のような質問からクエリを
作れます。

```sql
FROM read_ndjson_auto('.log/*.jsonl', union_by_name = true)
ORDER BY timestamp DESC
LIMIT 100;
```

`union_by_name = true` により、ファイルごとに任意のアプリケーション属性が異なっていても
読み取れます。未知のフィールドを絞り込む前に、推論された型を確認します。

```sql
DESCRIBE SELECT *
FROM read_ndjson_auto('.log/*.jsonl', union_by_name = true);
```

最近の警告とエラーを調べるクエリです。

```sql
SELECT timestamp, severity, service_name, message, trace_id
FROM read_ndjson_auto('.log/*.jsonl', union_by_name = true)
WHERE lower(severity) IN ('warn', 'error')
  AND timestamp >= now() - INTERVAL '1 hour'
ORDER BY timestamp DESC
LIMIT 100;
```

繰り返し発生しているイベントを数えます。

```sql
SELECT service_name, severity, message, count(*) AS occurrences
FROM read_ndjson_auto('.log/*.jsonl', union_by_name = true)
GROUP BY ALL
ORDER BY occurrences DESC
LIMIT 50;
```

一つのトレースを発生順に並べます。

```sql
SELECT timestamp, severity, message, span_id
FROM read_ndjson_auto('.log/*.jsonl', union_by_name = true)
WHERE trace_id = 'REPLACE_WITH_TRACE_ID'
ORDER BY timestamp;
```

一回の起動だけに絞る場合、JSONリーダーの `filename` オプションに対応した DuckDB では、
入力元パスを仮想列として取り出せます。

```sql
SELECT filename, timestamp, severity, message
FROM read_ndjson_auto(
    '.log/*.jsonl',
    union_by_name = true,
    filename = true
)
WHERE filename = '.log/REPLACE_WITH_RUN_FILE.jsonl'
ORDER BY timestamp;
```

探索的な分析は読み取り専用にしてください。利用者が渡した値はSQL文字列へ連結せず、bindするか
慎重に引用します。実行中のアプリケーションが走査中に追記することがあります。最新レコードが
一時的に不完全なら、もう一度実行してください。

## 本番環境の出力先

`pw dev` 以外で Popcorn Wave がローカルログファイルを作ることはありません。本番ログは
プラットフォームのログ収集器に向けた標準出力上の構造化 JSON となり、OTLP は設定済みの
コレクタへ送られます。ファイルの所有、ローテーション、保持期間、アクセス制御、削除は、
アプリケーションコンテナではなくデプロイ先のプラットフォームが管理します。

レベル、`stdout_format`、サービス識別子、リソース属性、OTLPのendpoint/headersは、TOML
または対応する `OTEL_*` 環境変数から
[アプリケーション設定](/ja/reference/configuration/#observability)で設定します。ローカル保存の
スイッチは、デプロイ済みアプリケーションではなく開発プロセスを制御するため、
`popcornwave.toml` に属します。

OTLP は、リクエスト処理がコレクタを待たないよう上限付きキューを使い、満杯ならレコードを
破棄します。設定された間隔で未充足のバッチも送り、終了時には期限付きの最終flushを行います。
キュー、バッチ、リクエストのtimeout、終了時timeoutの各設定は同じ設定リファレンスにあります。

## ローカル保存を使わない場面

`.log` をデプロイ先のストレージ、監査証跡、コレクタの代替として使わないでください。
ローテーション、保持期間の強制、転送、アクセス制御のワークフローを持たないためです。また、
知りたいことがリクエスト内の所要時間だけなら、開始・終了ログを増やすのではなくトレースを使います。

## 運用チェックリスト

- 安定したイベント名と型付き属性を記録し、データをメッセージへ埋め込まない。
- リクエストのコンテキストを `pw.Logger` へ渡し、トレースとの関連を保つ。
- `.log/` をコミットせず、ローカル方針に沿って古いファイルを削除し、本番ストレージにしない。
- 機密フィールドを含み得る生ファイルではなく、クエリと要約を共有する。
- リクエストの形と時間はテレメトリビューア、複数レコードや実行を横断する問いは DuckDB で調べる。

開発ループ全体は [`pw dev`](/ja/pw/project/dev/#テレメトリビューア)、対話的なトレース画面は
[開発テレメトリビューア](/ja/productivity/dev-telemetry-viewer/)を参照してください。
