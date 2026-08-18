---
title: リレーショナルデータベース
description: フレームワークが管理するデータベース接続。エンジン、DSN、接続プールの上限、ステートメントの実行先となる読み取り・書き込みグループを設定します。
sidebar:
  order: 0
---

アプリケーションはデータベースを開きません。接続を記述するのは `[middleware.rdb]` で、
最初のリクエストの前にフレームワークがそれを開き、生成されたステートメントは
リクエストの context から適切な接続を見つけます。正しい場所に置くべき `SetDatabase`
呼び出しはなく、ハンドラのシグネチャに通すべき `*sql.DB` もありません。

つまり、トポロジは設定ファイルがまるごと引き受けています。SQLite のファイル 1 つと
3 ノードのリーダー・ライター構成が同じアプリケーションでいられるのは、そのためです。
違うのはこのファイルの中だけです。

## 有効にする

プールは既定では**無効**です。`[[middleware.rdb.connections]]` の要素 1 つがプール 1 つで、
単一のデータベースなら要素も 1 つです。

```toml
[middleware.rdb]
enabled = true

[[middleware.rdb.connections]]
dsn = "sqlite://myapp.db"
connect_timeout = "5s"
max_open_conns = 1
max_idle_conns = 1
```

`group` を書かない要素は `default` という名前のグループに入るので、データベースが 1 つの
プロジェクトはグループ名をまったく書きません。`pw init --db=…` と `pw add database` は、
選んだエンジン向けにこのセクションと、そのエンジンに必要なブランクインポートを書きます。

| キー | 既定値 | 意味 |
| --- | --- | --- |
| `enabled` | `false` | フレームワークが管理する接続プールを開く |
| `default_group` | *(空)* | グループを指定しないステートメントの宛先。グループが 2 つ以上になると必須 |
| `write_group` | *(空)* | フレームワーク自身の書き込みの宛先。書き込み可能な接続を持つグループが 2 つ以上になると必須 |
| `migration_group` | *(空)* | マイグレーションとシードの宛先。省略すると `write_group` |

TOML では `[[…]]` ヘッダー以降のキーがその要素のものになるため、この 4 つは最初の接続より
前に書く必要があります。

## エンジン

DSN のスキームがエンジンを選びます。`database/sql` のドライバ名ではありません——3 つの
うち 1 つはドライバ名を登録しないからです。どのエンジンもブランクインポートで
バイナリに入り、アプリケーションは使わない SQL ドライバを持ちません。

| スキーム | エンジン | インポート |
| --- | --- | --- |
| `sqlite://` | SQLite | `_ "github.com/shibukawa/popcornweb/database/sqlite"` |
| `postgres://`, `postgresql://` | PostgreSQL | `_ "github.com/shibukawa/popcornweb/database/postgres"` |
| `mysql://` | MySQL、MariaDB | `_ "github.com/shibukawa/popcornweb/database/mysql"` |

このインポートは `pw init` が書きます。無い場合、プールは開くのを拒否し、登録されていない
ドライバを探して `database/sql` の奥で失敗する代わりに、追加すべきインポートを名指しします。
知らないスキームも同じように失敗し、このビルドが実際に持っているスキームを挙げます。

スキームの後ろの形はエンジンごとに違います。それぞれのドライバが受け取る形そのものだから
です。SQLite ならパスか `:memory:`、PostgreSQL なら libpq の URL、MySQL なら go-sql-driver の
DSN です。MySQL では、DSN 自身が指定していなければ `parseTime=true` が付きます。付けるのは
エンジンの側なので、呼ぶ側が覚えておく必要はありません。

解決されたエンジンは、フレームワークの残りが読むダイアレクトも決めます。セーブポイントの
対応可否、`EXPLAIN` の構文、マイグレーションランナーのダイアレクトはすべてそこから来ます。
スキームは `popcornweb.toml` の `project.database` と一致させてください。一方はどのドライバが
クエリを実行するかを、もう一方は `pw generate` がどの構文にコンパイルしたかを決めています。

PostgreSQL だけは、リクエストを `database/sql` ではなく pgx のネイティブプールで処理します。
`sql.DB` の層はプールのミューテックスと呼び出しごとのミューテックスを毎ステートメントに
課すためで、それを外したのがこの経路です。クエリの書き方も、トランザクションも、設定も
変わりません。どちらの経路を取ったかは起動ログが接続ごとに示します（`path=native` と
`path=database/sql`）。知っておくべき違いは 2 つ。PostgreSQL の接続では `pw.DB` が
`*sql.DB` を返さないこと、そして `max_idle_conns` が効かないことです。pgx のプールは
アイドル接続を数ではなく `conn_max_idle_time` の時間で回収します。マイグレーションと
シーディングは従来どおり `database/sql` で動きます。バイパスするのはリクエスト経路
だけです。

## 接続 1 つ

| キー | 既定値 | 意味 |
| --- | --- | --- |
| `group` | `"default"` | この接続を呼ぶときの名前 |
| `dsn` | *(空)* | データソース名。報告される場所ではマスクされるのは資格情報だけ |
| `readonly` | `false` | フレームワークの書き込みには決して選ばれない |
| `connect_timeout` | `"5s"` | 起動時の ping の上限 |
| `max_open_conns` | `0` | |
| `max_idle_conns` | `0` | |
| `conn_max_lifetime` | `"0s"` | |
| `conn_max_idle_time` | `"0s"` | |

`dsn` は秘密情報として扱われますが、隠されるのは資格情報だけです。起動サマリでも
`pw doctor` でもエラーメッセージでも `postgres://*****@db.internal:5432/app` の形で出ます。
スキーム・ホスト・ポート・データベース名は残り、ユーザー情報とクエリ文字列は落とします。
どのデータベースに繋がっているかは運用上の事実であり、何も答えない行は読まれなくなるから
です。SQLite のパスや `:memory:` は資格情報を持たないのでそのまま出ます。

接続の要素は固有の CLI オプションも環境変数も持ちません（要素の同一性はファイル内での位置
だからです）。そのため接続ごとのパスワードをコミットする TOML の外に出す手段が `${NAME}`
です。展開はファイル読み込み時に、文字列の値に対してのみ行われます。未定義の名前は空文字に
展開されるのではなく読み込みエラーになります。リテラルの `$` は `$$` と書きます。展開の
有無にかかわらず、`dsn` はマスクされたままです。
[アプリケーション設定](/ja/guides/architecture/configuration/)を参照してください。

## リーダーとライター

リードレプリカを持つ構成も同じ形で、要素が増えるだけです。要素ごとに所属グループを指定し、
同じグループに複数の要素を置けます。読み取りはその中でラウンドロビンに振り分けられますが、
1 つのリクエストが同じグループから 2 回読むときは、すでに持っている接続に留まります。

```toml
[middleware.rdb]
enabled = true
default_group = "replica"
write_group = "writer"

[[middleware.rdb.connections]]
group = "writer"
dsn = "postgres://app:${DB_PASSWORD}@writer.example/app"
max_open_conns = 20

[[middleware.rdb.connections]]
group = "replica"
dsn = "postgres://app:${DB_PASSWORD}@replica-1.example/app"
readonly = true

[[middleware.rdb.connections]]
group = "replica"
dsn = "postgres://app:${DB_PASSWORD}@replica-2.example/app"
readonly = true
```

グループを指定しないステートメントは `default_group` で実行されます。書き込みは明示的に
グループを選びます。

```go
// 単一のステートメント
user, err := queries.CreateUser(pw.SelectDB(r, "writer"), name)

// トランザクション全体。中でグループを指定しないステートメントも writer に残ります
err := pw.TransactionContext(pw.SelectDB(r, "writer"), func(ctx context.Context) error {
	return queries.RecordAudit(ctx, "user.created")
})
```

グループを固定するのは `pw.SelectDB` だけです。単一のステートメントでもトランザクション
全体でも同じ書き方で、トランザクション専用の書き方は存在しません。どちらのグループが
勝ったのかを考える場面がそもそも無いということです。

1 つのトランザクションが 2 つのグループにまたがることはありません。別のグループを指定した
ネストした `pw.Transaction` は `ErrCrossGroupTransaction` を返し、外側はそのまま使えます。
トランザクションの中から `readonly` グループを `SelectDB` することはできます（その読み取りは
トランザクションの外で実行されます）が、書き込み可能なグループは選べません。原子的に見えて
実際はそうではない書き込みになるためです。

[マイグレーション](/ja/productivity/migrations/)、[シードデータ](/ja/productivity/seed-data/)、
セッションテーブルは `write_group`（さらに絞る場合は `migration_group` と
`session.rdb.group`）に書き込まれます。`readonly` の接続がそれらに選ばれることはなく、
そう設定すると起動時にエラーになります。

接続が 1 つだけの構成 — `testutil` によるテスト実行を含みます — では、*どのグループ名*も
その 1 つのデータベースを指します。クラスタ向けに書いたコードが、テスト用の分岐なしで開発用の
SQLite ファイル 1 つに対してもそのまま動きます。

グループは、フェイルオーバーが置いてありそうな場所に見えます。置いてありません。ヘルス
チェックも、切り離しも、レプリカ間のフェイルオーバーも、レプリカ遅延の考慮も、
read-your-writes のルーティングもありません。遅れているレプリカもそのまま選ばれます。自分の
書き込みを読まなければならない読み取りは、`SelectDB` でライターへ送ります。それが問題だと
知っているコードだけが決められることだからです。

## 起動と readiness

すべての接続は、最初のリクエストを受ける前に、それぞれの `connect_timeout` の中で開かれて
ping されます。応答しない接続はデプロイを止め、`replica#2` のようにグループ名と連番の
ラベルで名指しされます。5 台のレプリカのどれが届かないのかが、メッセージだけでわかります。
途中まで開いた集合は、そのまま使われるのではなく閉じられます。そこを通り抜けたものを、
[設定サマリ](/ja/productivity/startup-summary/)がグループ、`readonly` の有無、プールの上限、
マスク済みの DSN とともに並べます。

[readiness エンドポイント](/ja/guides/deployment/operational-endpoints/)は、プロセスが動いて
いる間ずっとすべての接続を ping します。応答しなくなったレプリカはインスタンスを unready に
します。アプリケーションが読むのは default グループであり、読めないインスタンスはライターが
何と言おうと ready ではないからです。

シャットダウンでは、リスナーを止めて実行中のハンドラが終わったあとに、フレームワークが
開いたすべてのプールを閉じます。

## スキーマはどこから来るのか

ここではテーブルを 1 つも作りません。接続の集合はすでに存在するスキーマに対して開かれます。
だから初回のデプロイは、テーブルを勝手に作るのではなく、無いテーブルで失敗します。そこへ
スキーマを持っていくのが[データベースマイグレーション](/ja/productivity/migrations/)、初期の
行が[シードデータ](/ja/productivity/seed-data/)です。フレームワークが管理するテーブル、特に
セッションテーブル——は、それを持つパッケージがマイグレーションを同梱していて、同じ実行で
適用されます。

その接続の上を実際に走るステートメントが[クエリ](/ja/guides/storage/queries/)です。
