---
title: レスポンス
description: HTML、JSON、ストリーム、RFC 9457 準拠のエラーを返す。
sidebar:
  order: 2
---

ハンドラは必ず 1 つのレスポンスを書いて終わります。方法は 4 つあり、いずれもボディを
書く前にステータスとヘッダを決めます。

## HTML

```go
pw.WriteHTML(w, r, Home(HomeParams{Name: input.Name}))
```

`Home` と `HomeParams` は `handlers/home.pw.html` から生成されます。ハンドラが渡すのは
**リーフのフラグメント**で、`WriteHTML` がそれをアプリケーションに登録されたドキュメント
シェルの中に描画します。そのためハンドラはドキュメントを指名することも、インポート
することも、構築することもありません。

チェーン全体はバッファに描画され、コミットの**前に**検証されます。テンプレートの失敗は
中途半端なページではなく、きれいな 500 になります。圧縮が有効でクライアントが受け入れる
場合、バッファされたボディは送出時に zstd でエンコードされます。

ラッパーのチェーンを明示的に制御したい場合 —— あるページだけ別のシェルに入れたい場合
—— は `pw.WriteHTMLChain` を使います。

```go
pw.WriteHTMLChain(w, r,
	[]pw.HTMLWrapper{templates.BindPrintDocument(templates.PrintDocumentParams{})},
	Invoice(InvoiceParams{ID: input.ID}),
)
```

テンプレートの構文、スロット、エスケープ、スコープ付きスタイルは
[テンプレート](/ja/guides/templates/)で扱います。

## JSON

```go
pw.WriteAPI(w, r, user)
```

レスポンス型のエンコーダは呼び出し箇所から生成されるので、実行時のリフレクションは
ありません。`json` タグの名前部分は使いますが、`omitempty` や除外指定は解釈しません。
送りたい形をそのまま宣言してください。

ステータスは 200 です。`pw.WriteAPI` の呼び出し箇所は生成される OpenAPI ドキュメント
にも反映されるため、JSON エンドポイントは別途アノテーションを書かなくても記述されます。

## ストリーム

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

`pw.NewStream[T]` はリクエストからワイヤ形式をネゴシエートしてレスポンスを開始します。
`Send` が 1 つの値を書き、`Close` がレスポンスを確定します。JSON 配列形式では閉じ括弧が
ここで書かれるため、`Close` は重要です。

| 形式 | メディアタイプ |
| --- | --- |
| Server-Sent Events | `text/event-stream` |
| NDJSON | `application/x-ndjson`、`application/ndjson`、`application/jsonl` |
| JSON 配列 | `application/json` |

選択順は `?stream=` クエリパラメータ、`Accept`、User-Agent のヒューリスティック、
最後に NDJSON のフォールバックです。サポート対象をどれも要求しない `Accept` ヘッダには
`406 Not Acceptable` の problem レスポンスが返り、以降の `Send` は書き込まずにその
エラーを返します。

`server.write_timeout` の既定が `0s` なのは、まさに長時間のストリームを切断しないため
です。[設定](/ja/guides/configuration/)を参照。

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

したがってハンドラは、サービスが返したものを変換せずにそのまま転送できます。

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

現状の境界に注意してください。フレームワークのエラー経路は `pw.WriteProblem` であり、
常に `application/problem+json` を返します。また `pw.WriteHTML` はステータスコードを
取らないため、常に 200 を返します。これらのテンプレートを 4xx や 5xx のステータスで
描画する部分は、現時点ではアプリケーション側で組み立てる必要があります。
