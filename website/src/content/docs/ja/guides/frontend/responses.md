---
title: レスポンス
description: HTML、JSON、ストリーム、RFC 9457 準拠のエラーを返す。
sidebar:
  order: 2
---

ハンドラは HTML、JSON、ストリーム、エラーのいずれも返せますが、4 つには共通する
制約があります。ボディを書き始める前に、ステータスとヘッダを確定しなければなりません。
レスポンスヘルパーは、それぞれに必要なワイヤ形式を保ちながら、この境界を守ります。

## HTML

```go
pw.WriteHTML(w, r, Home(HomeParams{Name: input.Name}))
```

`Home` と `HomeParams` は `handlers/home.pw.html` から生成されます。ハンドラが渡すのは
**リーフのフラグメント**で、`WriteHTML` がそれをアプリケーションに登録されたドキュメント
シェルの中に描画します。そのためハンドラはドキュメントを指名することも、インポート
することも、構築することもありません。

チェーン全体はバッファに描画され、コミットの**前に**検証されます。そのため
テンプレートが失敗しても、書きかけのページではなく、きれいな 500 にできます。圧縮が
有効でクライアントが受け入れる場合、同じバッファ済みボディが送出時に zstd で
エンコードされます。

ラッパーのチェーンを明示的に制御したい場合 —— あるページだけ別のシェルに入れたい場合
—— は `pw.WriteHTMLChain` を使います。

```go
pw.WriteHTMLChain(w, r,
	[]pw.HTMLWrapper{templates.BindPrintDocument(templates.PrintDocumentParams{})},
	Invoice(InvoiceParams{ID: input.ID}),
)
```

### フラグメント

htmx のような操作は、すでに存在するページの一部だけを差し替えます。必要なのは
ドキュメントではなくその領域です。`pw.WriteHTMLFragment` は指定したテンプレートだけを
描画します。

```go
pw.WriteHTMLFragment(w, r, Row(RowParams{Item: item}))
```

ドキュメントシェルも、ラッパーのチェーンも、head の合成も、フレーミングもありません。
ボディはテンプレートが書いたものそのままで、swap ライブラリがそのまま挿入できます。

マークアップを囲むドキュメントがないことから、2 つの帰結が生まれます。

まず、フラグメントはストリーミングしません。await 境界はその場で確定するため、
レスポンスにはクライアントランタイムが置き換えるべきプレースホルダも、挿入先の
ページで未確定な境界と衝突しうる境界 ID も残りません。

次に、ドキュメントの head に寄与するテンプレートは、寄与を黙って捨てるのではなく
500 で拒否します。スコープ付きスタイルが属するのは、すでに読み込まれているページの
head です。そのページが描画するコンポーネントか、共有のスタイルシートで宣言して
ください。

失敗時は HTML のエラーページではなく、`application/problem+json` と本来のステータスで
応答します。エラードキュメントを一領域に swap すると、その領域がページ丸ごとに
置き換わってしまうからです。htmx などのライブラリは 2xx 以外を swap しないため、
ステータスがそのまま判断材料になります。

`examples/htmx_fragment` はこの API だけで組み立てた完全なアプリケーションです。
ドキュメントを返すルートは 1 つだけで、絞り込み・追加フォーム・削除ボタンはそれぞれ
自分が描き直す領域だけを返します。検証に失敗したフォームの扱いも示しています。
`check` の失敗をそのまま problem レスポンスにすると swap ライブラリが無視するためです。

テンプレートの構文、スロット、エスケープ、スコープ付きスタイルは
[テンプレート](/ja/guides/frontend/templates/)で扱います。この API の上に何を組むか
—— サーバーが中身を用意するダイアログ、トースト、swap が最も安い答えでなくなる境目
—— は[フラグメントと島](/ja/guides/interactivity/fragments/)にあります。

## JSON

```go
pw.WriteAPI(w, r, user)
```

呼び出し箇所からレスポンス型のエンコーダを生成するため、実行時リフレクションは
ありません。エンコーダは `json` タグの名前部分を使いますが、`omitempty` や除外指定は
解釈しません。したがって、宣言する型を送りたい形に合わせる必要があります。

ステータスは 200 です。`pw.WriteAPI` の呼び出し箇所は生成される OpenAPI ドキュメント
にも反映されるため、JSON エンドポイントは別途アノテーションを書かなくても記述されます。

## ストリーム

トークン、ログ行、キューのイベントのように時間をかけて届くレスポンスは、代わりに
`pw.NewStream[T]` で書きます。

```go
func events(w http.ResponseWriter, r *http.Request) {
	stream := pw.NewStream[ChatEvent](w, r)
	defer stream.Close()

	for event := range source {
		if err := stream.Send(event); err != nil {
			return
		}
	}
}
```

Server-Sent Events、NDJSON、JSON 配列のどれになるかを選ぶのはクライアントで、上の
ハンドラは 3 つすべてをそのまま処理します。ネゴシエーションの詳細、フレーミング、
長時間のレスポンスが設定に求めるものは[ストリーム](/ja/guides/frontend/streams/)に
あります。

## エラー

`pw.WriteProblem` は **RFC 9457 Problem Details** のレスポンスを
`application/problem+json` として書き出します。

```go
pw.WriteProblem(w, r, pw.NotFound("no such user"))
```

```json
{
  "type": "about:blank",
  "title": "Not Found",
  "status": 404,
  "detail": "no such user",
  "code": "not_found"
}
```

### コンストラクタ

| コンストラクタ | ステータス |
| --- | --- |
| `pw.BadRequest` | 400 |
| `pw.Forbidden` | 403 |
| `pw.NotFound` | 404 |
| `pw.InternalServerError` | 500 |

いずれも `error`、`string`、別の `pw.Problem`、あるいは引数なしを受け付けます。これらの
呼び出し箇所も生成器に読まれるため、エンドポイントが返しうるエラーレスポンスが
OpenAPI の記述に現れます。

コンストラクタのないステータスは値を直接組み立てます。

```go
pw.WriteProblem(w, r, pw.Problem{
	Status:  http.StatusConflict,
	Title:   "Conflict",
	Code:    "already_registered",
	Message: "that email is already registered",
})
```

### エラーをそのまま流す

`pw.WriteProblem` は任意の `error` を受け取ってマッピングします。

- `pw.Problem`（`%w` でラップされたものを含む）はそのまま使われる
- バインディングやバリデーションのエラーは自身のステータスとフィールド情報を保つ
- それ以外は 500 になる

このマッピングにより、ハンドラは別の変換レイヤーを増やさず、サービスのエラーを
そのまま転送できます。

```go
if err := service.Register(r.Context(), input); err != nil {
	pw.WriteProblem(w, r, err)
	return
}
```

### 2 つの安全動作

**5xx の詳細は漏れません。** ステータスが 500 以上の場合、リクエストスコープのロガーで
完全な内容を記録したうえで、クライアントには `internal error`（code は `internal`）
としてのみ報告します。

**コミット済みのレスポンスは壊しません。** すでにボディの書き込みが始まっている場合、
`WriteProblem` は矛盾する 2 つ目のペイロードを追記せず、エラーをログに記録します。

### HTML のエラーページ

スキャフォールドされたプロジェクトには `templates/400.pw.html`、`404.pw.html`、
`500.pw.html` が含まれます。これらは他のページと同じように生成される通常の
コンポーネントです。

ただし、ステータスと結びつける部分は今のところ手作業です。`pw.WriteProblem` は常に
`application/problem+json` を返し、`pw.WriteHTML` はステータスコードを取らないため
200 を返します。これらのテンプレートを 4xx や 5xx のステータスで使いたい場合は、
アプリケーション側でその経路を組み立てることになります。
