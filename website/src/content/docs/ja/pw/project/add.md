---
title: pw add
description: 既存プロジェクトにフレームワークの機能を追加する。
sidebar:
  order: 2
---

```sh
pw add [devbox|database|dynamo|redis-valkey|auth|tailwind]
```

`pw init` はどの機能から始めるかを尋ねますが、それはプロジェクトを理解しきる前の
判断です。`pw add` は後から 1 つ追加します。断った答えは、そのプロジェクトが
背負い続ける決定ではありません。

このコマンドは既存プロジェクトの中で動き、質問はウィザードで行います。フラグ形式は
ありません。新しいディレクトリを作る `pw init` と違い、こちらはプロジェクトが既に
依存している設定・マイグレーション・ソースを編集します。その編集を承認する場所が
レビュー画面です。

## カタログ

| 機能 | 追加されるもの |
| --- | --- |
| `devbox` | `devbox.json` と `devbox.lock`。このプロジェクトが既に使っているツールチェインを含む |
| `database` | `[middleware.rdb]` セクション、マイグレーションディレクトリ、型付き SQL の例 |
| `dynamo` | `[middleware.dynamo]` セクション、型付きレコード、ローカルの DynamoDB サーバー |
| `redis-valkey` | `devbox.json` の Valkey 開発サーバー |
| `auth` | ログインセッション、フレームワークのテーブル、アカウントリゾルバ |
| `tailwind` | ピン留めした Tailwind ツールチェイン、CSS エントリ、`[assets.tailwind]` ブロック |

引数は最初のステップの初期選択になります。省略すると、そのプロジェクトがまだ持って
いないものだけが並びます。他に依存するものが 2 つあります。`auth` はログインセッション
のために `database` を必要とし、`redis-valkey` は `devbox` を必要とします。Valkey の
選択は Devbox のパッケージ以外に何も書かないからです。依存を持たないプロジェクトで
選ぶと、先に依存を追加し、その旨をレビュー画面に表示します。

## 検出

機能の有無は、それを担っているファイルから検出します。`popcornwave.toml` の一覧では
ありません。マニフェストは手編集されたプロジェクトと食い違い得るからです。

| 機能 | 根拠 |
| --- | --- |
| `devbox` | `devbox.json` |
| `database` | 環境設定ファイルの `[middleware.rdb]` |
| `redis-valkey` | `devbox.json` の Valkey パッケージ |
| `auth` | `init_popcornwave_auth` マイグレーション（バージョンは問わない） |
| `tailwind` | `popcornwave.toml` の `assets.tailwind.enabled` |

既にある機能を追加しようとすると、根拠のファイルを名指しして失敗します。

```
pw: add: this project already has auth, per migrations/00003_init_popcornwave_auth.sql
```

## 書き込む内容

レビュー画面が、何も書く前に変更を全て列挙します。

```
  Review
    Capability     auth
    OIDC provider  External provider

    create  handlers/accounts.go
    create  migrations/00002_init_popcornwave_session.sql
    create  migrations/00003_init_popcornwave_auth.sql
    append  config.dev.toml
    edit    cmd/lean/main.go
    then    pw migrate up

  enter add  ·  esc back  ·  ctrl+c cancel
```

この一覧を支配する規則は 4 つです。

**マイグレーションは次に空いているバージョンを取ります。** `00001` から `00007` まで
適用済みのプロジェクトなら `00008` です。番号の振り直しは行いません。適用済みかも
しれないマイグレーションを動かすことは決してできないからです。

**設定は追記で、書き換えではありません。** コメントや調整した値はそのまま残ります。
同名のセクションが既にあればコマンドは停止します。

**アプリケーション所有のファイルは上書きしません。** 衝突を報告して何も書きません。
フレームワークが代わりにやらないこと（`main.go` の呼び出し、ドキュメントシェルへの
スタイルシートのリンク）は、手作業のステップとして表示します。

**中途半端な状態を作りません。** 全ファイルを先に計算してからまとめて書くので、
失敗するステップがあればプロジェクトは元のままです。

## 終了ステータス

| 状況 | 終了コード |
| --- | --- |
| 機能を追加した | 0 |
| ウィザードをキャンセルした | 0、何も書かない |
| 端末がない | 非ゼロ、usage を表示 |
| 既に存在する、または衝突 | 非ゼロ、パスと理由を表示 |
