---
title: ハンドラ
description: ルーティングとリクエストバインディング。構造体タグ、バリデーション、JSON・フォーム・multipart のボディ。
sidebar:
  order: 1
---

`github.com/shibukawa/popcornwave/pw` が安定したアプリケーション向け API です。
このページの内容はすべてそこに含まれます。

## ルーティング

```go
package handlers

import "github.com/shibukawa/popcornwave/pw"

var mux = pw.NewServeMux()

func Handlers() *pw.ServeMux { return mux }
```

通常の Go ビルドでは `pw.ServeMux` は `net/http` の `ServeMux` **そのもの**です。
ラッパーではなく型エイリアスなので、パターン、ワイルドカード、優先順位は完全に標準
ライブラリのものです。TinyGo には標準の mux がないため、同じセマンティクスを持つ別
実装が使われます。

ハンドラのファイルはそれぞれ `init` で自分を登録します。ルートを追加する作業は、
中央のテーブルを編集することではなくファイルを追加することです。

```go
func init() {
	mux.HandleFunc("GET /", home)
	mux.HandleFunc("GET /users/{id}", showUser)
	mux.HandleFunc("POST /users", createUser)
}
```

アプリケーションの成長に合わせてルートを複数パッケージに分割する方法は
[プロジェクト構成](/ja/guides/project-structure/)を参照してください。

## リクエストのバインディング

`pw.Parse[T]` がリクエストから構造体を埋めます。生成器が呼び出し箇所を読むので、
`T` のバインディングコードは実行時のリフレクションではなく事前に書き出されます。

```go
type showUserInput struct {
	ID   int    `path:"id"`
	Sort string `query:"sort" default:"name"`
}

func showUser(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[showUserInput](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	// ...
}
```

### 取得元のタグ

| タグ | 取得元 |
| --- | --- |
| `input:"name"`（またはタグなし） | クエリ文字列。なければボディ |
| `query:"name"` | クエリ文字列のみ |
| `payload:"name"` | リクエストボディのみ |
| `path:"id"` | パスのワイルドカード（`/users/{id}` など） |
| `header:"Authorization"` | リクエストヘッダ |
| `cookie:"session"` | リクエストクッキー |
| `method:"method"` | HTTP メソッド |

ワイヤ名を明示しない場合、フィールド名は lowerCamelCase になります。`DisplayName` は
`displayName` です。

`input` は寛容な既定です。クエリパラメータが優先され、クエリに値がないときだけボディが
読まれます。取得元を 1 つに限定したいときは `query` か `payload` を使ってください。

### リクエストボディ

1 つのリクエスト構造体が 3 つの形式すべてを受け付けるので、形式はクライアントが
選べます。

- `application/json`
- `application/x-www-form-urlencoded`
- `multipart/form-data`

したがって、通常の HTML フォーム送信と JSON API 呼び出しで同じハンドラを共有できます。

```go
type createUserInput struct {
	Name  string `payload:"name" check:"required,maxlen=40"`
	Email string `payload:"email" check:"required,email"`
}
```

### ファイルアップロード

multipart のファイルフィールドには `httpbind.File` を使います。

```go
import "github.com/shibukawa/tinybind-go/httpbind"

type uploadInput struct {
	Title string        `payload:"title" check:"required"`
	Image httpbind.File `payload:"image" check:"required"`
}
```

`File` は `Filename`、`ContentType`、`Size`、`Content` を持ちます。multipart ボディの
上限は既定で 1 MiB で、`httpbind.SetMaxMultipartBodyBytes` で変更します。なお
フレームワーク側の `server.max_request_body`（既定 10 MiB）が先に適用される点に
注意してください。[設定](/ja/guides/configuration/)を参照。

### 宣言していないフィールド

`payload:"*"` は宣言していないフィールドをまとめて受け取ります。

```go
type eventInput struct {
	Type   string         `payload:"type"`
	Extras map[string]any `payload:"*"`
}
```

デコード済みの値ではなく生の JSON を保持したい場合は `map[string]json.RawMessage`
を使います。

## バリデーション

`check` タグが制約を宣言します。制約は生成時にコンパイルされ、リクエストごとに解釈
されるわけではありません。

| ルール | 対象 | 例 |
| --- | --- | --- |
| `required` | すべて | 空文字列、欠損値、空ファイルを拒否 |
| `default=value` | スカラー | 値がないときに適用 |
| `min` / `max` | 数値 | `check:"min=1,max=100"` |
| `minlen` / `maxlen` / `len` | 文字列 | `check:"maxlen=40"` |
| `enum=a\|b\|c` | スカラー | `check:"enum=asc\|desc"` |
| `pattern=...` | 文字列 | 正規表現 |
| `email` | 文字列 | RFC 形式 |
| `uuid` | 文字列 | UUID 形式 |
| `date` | 文字列 | `YYYY-MM-DD` |
| `time` | 文字列 | `HH:MM:SS` |
| `datetime` | 文字列 | RFC 3339 |

複数のルールはカンマで区切ります。`pattern` にカンマが含まれる場合は最後に置いて
ください。

```go
type listInput struct {
	Page int    `query:"page" check:"min=1" default:"1"`
	Sort string `query:"sort" check:"enum=asc|desc" default:"asc"`
}
```

チェックに失敗すると `pw.Parse` は該当フィールドの情報を持つエラーを返します。それを
`pw.WriteProblem` に渡すと、フィールド単位の詳細を含む 400 になります。
[レスポンス](/ja/guides/responses/)を参照。

## リクエストスコープのアクセサ

| 呼び出し | 戻り値 |
| --- | --- |
| `pw.Logger(ctx)` | リクエストスコープの `*slog.Logger` |
| `pw.Config[T](ctx)` | 登録済みの設定構造体 |
| `pw.DB(ctx)` | `(*sql.DB, bool)` |
| `pw.Transaction(ctx, fn)` | `fn` をトランザクション内で実行 |

```go
func createUser(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[createUserInput](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	err = pw.Transaction(r.Context(), func(ctx context.Context) error {
		if _, err := queries.InsertUser(ctx, input.Name, input.Email); err != nil {
			return err
		}
		return queries.RecordAudit(ctx, "user.created")
	})
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}
```

コールバック内で呼ばれる生成済みクエリ関数は、context からトランザクションを受け取り
ます。[クエリ](/ja/guides/queries/)を参照。

## ライフサイクル

```go
func main() {
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

`pw.Run` は設定をパースし、`--generate-config` などのアプリケーションフラグを処理し、
設定されたランタイムを検証し、データベースプールを初期化し、自分のルートと運用
エンドポイントの衝突を確認し、ミドルウェアスタックを構築し、配信し、`SIGINT` または
`SIGTERM` でグレースフルにシャットダウンし、登録済みリソースを逆順にクローズします。

サーバー自体は自分で持ちたい場合 —— 別のリスナーの背後や、テストの中など ——
`pw.Middlewares(handler, options...)` が同じ初期化を行い、同じスタックを素の
`http.Handler` として返します。

`pw.WithPublicFS(fsys)` は埋め込みの public ツリーを明示的に渡します。スキャフォールド
されたプロジェクトでは代わりに `public.go` が登録します。
