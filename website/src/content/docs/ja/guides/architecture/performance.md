---
title: パフォーマンス
description: リクエスト処理の目安と、本番リリース前に確認する設定。
sidebar:
  order: 5
---

Popcorn Wave はセッション、CSRF、セキュリティヘッダ、リクエスト ID などを
標準で処理します。それでも todo サービスの計測では、手書きの `net/http` 実装より
少ない CPU 時間でリクエストを完了しました。フレームワークの処理を外すことより、
データベースやレスポンス生成を見直すほうが効果の出る構成です。

ただし、ベンチマークの順位はアプリケーションと実行環境で変わります。以下の数値は
性能保証ではなく、どのレイヤーから調べるかを決めるための目安として読んでください。

## レイヤーごとの処理時間

[`examples/todo`](https://github.com/shibukawa/popcornwave/tree/main/examples/todo) に
20 並列で負荷をかけ、1 リクエストあたりの CPU 時間を測った結果です。計測環境は
Go 1.26.5、Apple M3、同一マシンの Docker 上で動く PostgreSQL 17 です。

| 1 リクエストあたり | CPU 時間 |
| --- | --- |
| ミドルウェア全体 | 3.0 µs |
| └ CSRF チェック | 2.1 µs |
| 50 行を返す `SELECT` 1 回 | 39 µs |
| JSON のエンコードと書き出し | 36 µs |
| HTML の描画と書き出し | 100 µs |
| リクエスト全体（Popcorn Wave） | 166 µs |
| リクエスト全体（`net/http` の比較実装） | 219 µs |

JSON と HTML は別のレスポンスなので、各行を合計して全体時間になる表ではありません。
ここで見るべき差は桁です。ミドルウェア全体は数 µs ですが、単純なクエリでも数十 µs、
HTML の生成にはさらに時間がかかっています。実際のデータベースがネットワーク越しに
あれば、クエリは数百 µs から数 ms になることもあります。

最後の 2 行の差は、上に並んだレイヤーの差では説明しきれません。残りはシステムコール
に消えています。`html/template` はテンプレートを歩きながら、値ひとつごとに
ResponseWriter へ書き出します。比較実装は CPU の 4 分の 3 を `write` の中で使って
いました。生成されたコンポーネントはバッファに描画してから、組み上がった文書を 1 回
で渡します。この行が表に無いのは、それがレイヤーではないからです。上のレイヤーが
ソケットに届くまでの経路の違いです。

まず、アプリケーションで時間を使っているレイヤーを測ってください。数 µs の
ミドルウェアを外すのは、そのレイヤーが実際にボトルネックだと分かってからで十分です。

## 本番リリース前に確認する設定

コードの細かな最適化より、開発用の設定を本番へ持ち込まないことのほうが重要です。

### 実行環境とログ

`config.dev.toml` はデバッグログと SQL の記録を有効にします。負荷試験と本番では
`APP_ENV=prod` などで本番用の設定を選び、クエリログが無効になっていることを確認して
ください。一時的に有効にした詳細ログも、調査が終わったら戻します。

[クエリ診断](/ja/productivity/query-diagnostics/)には、通常の本番ログを増やさずに
遅いクエリを調べる方法があります。

### セッションバックエンド

`cookie` バックエンドは外部ストレージを必要とせず、暗号処理も典型的なセッションで
1 リクエストあたり約 0.5 µs です。受信レコードの開封と送信レコードの封緘を合わせて、
256 バイトで 0.45 µs、1 KB で 0.73 µs でした。一方、セッションレコード全体を
レスポンスで送り、ブラウザが次のリクエストで送り返すため、レコードが大きくなるほど
通信量が増えます。

`rdb`、`redis`、`dynamo`、`firestore` は、ブラウザには小さな識別子だけを持たせます。
通信量は抑えられますが、代わりに毎リクエストでストレージへのアクセスが加わります。

```toml
[session]
enabled = true
backend = "rdb" # または redis, dynamo, firestore
```

小さなセッションを依存なしで扱うなら `cookie`、失効、永続化、サイズ、通信量を
サーバー側で管理するなら外部バックエンドが候補です。性能だけでなく運用上の要件で
絞り込み、実際のセッションサイズとストレージのレイテンシで測ってください。
`dev-volatile` は再起動ですべて消える開発専用のバックエンドで、本番では使えません。

詳しい違いは[セッション](/ja/guides/backend/sessions/)にあります。

### データベース接続

本番では、1 つの接続だけでなく、書き込み用と読み取り用の接続を複数設定できます。
同じ `group` に読み取り接続を複数置くと、そのグループ内でラウンドロビンされます。

```toml
[middleware.rdb]
enabled = true
default_group = "reader"
write_group = "writer"
migration_group = "writer"

[[middleware.rdb.connections]]
group = "writer"
dsn = "postgres://app:${DB_PASSWORD}@writer.example/app"
max_open_conns = 20

[[middleware.rdb.connections]]
group = "reader"
dsn = "postgres://app:${DB_PASSWORD}@reader-1.example/app"
readonly = true
max_open_conns = 20

[[middleware.rdb.connections]]
group = "reader"
dsn = "postgres://app:${DB_PASSWORD}@reader-2.example/app"
readonly = true
max_open_conns = 20
```

この設定では、グループを指定しないクエリは `reader` を使います。読み取りしかしない
ハンドラはそのまま既定のグループへ流せますが、Popcorn Wave は SQL の内容から接続先を
推測しません。書き込み側にはプログラムの変更が必要です。

```go
// 単独の書き込み
user, err := queries.CreateUser(pw.SelectDB(ctx, "writer"), name)

// 書き込みトランザクション
err := pw.Transaction(ctx, func(ctx context.Context) error {
	return queries.RecordAudit(ctx, "user.created")
}, pw.OnGroup("writer"))
```

書き込み直後の値を読むなど、レプリカの反映待ちを許容できない読み取りも `writer` へ
向けます。接続グループの詳しい動作は
[リレーショナルデータベース](/ja/guides/storage/rdb/)を参照してください。

プール上限は接続ごとに設定されます。`max_open_conns` を増やす前に、全接続の上限を
合計してアプリケーションのインスタンス数を掛け、データベース側の上限に収まることを
確認してください。

### 圧縮と CSRF

レスポンス圧縮は既定で無効です。CDN やリバースプロキシが圧縮していない場合だけ、
[レスポンス圧縮](/ja/guides/frontend/compression/)を参考に有効化します。

CSRF チェックはミドルウェアの中では目立つ処理ですが、上の計測でも 2.1 µs です。
ベアラートークンで認証し、ブラウザからクッキー付きで呼ばれない API だけを除外対象に
してください。性能のために保護範囲を広く外す価値はありません。

```toml
[security.csrf]
enabled = true
include = ["/**"]
exclude = ["/api/**"]
```

## アプリケーションを測る

比較には本番相当の設定を使い、同じ機能、同じレスポンス、同じデータで測ります。
ユーザーの待ち時間を調べるならレイテンシ、収容力ならスループット、コード上の処理量なら
CPU プロファイルというように、判断したいことに計測方法を合わせてください。

プロファイルで候補を絞ったら、そのレイヤーだけを差し替えて再計測します。HTTP スタック
自体が限界だと確認できた場合の選択肢は、
[なぜ Popcorn Wave なのか](/ja/start/why-popcorn-wave/#http-スタック自体がボトルネックになったら)に
まとめています。
