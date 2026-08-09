---
title: API ドキュメント
description: すでに書いたコードから組み立てられる OpenAPI 3.1 ドキュメントと、それを読む閲覧 UI。
sidebar:
  order: 4
---

OpenAPI ドキュメントはたいてい 2 回書かれます。1 回はハンドラとして、もう 1 回は
仕様書として。そして仕様書は 1 つか 2 つのリリースで実装から離れていきます。

Popcorn Wave はドキュメントをコードから組み立てます。`pw generate` はバインディング
コードを書くために、ルート登録・`pw.Parse[T]` の呼び出し・`check` タグ・`pw.WriteAPI`
の呼び出しをすでに読んでいます。エンドポイントを説明するのに必要な材料は、それと
同じものです。パッケージごとに OpenAPI 3.1 のフラグメントが 1 つ生成され、
フレームワークが起動時にマージします。

アノテーションは 1 つも書きません。同期を保つべき別の仕様書ファイルもありません。
そもそも別の仕様書が存在しないからです。

## 配信する

```toml
[server]
openapi = "/openapi.json"    # 未設定なら何も配信しない
api_doc = "scalar"           # "scalar"、"swagger"、または空文字で無効
api_doc_path = "/docs"
```

`openapi` は、マージ済みのドキュメントが応答するパスです。既定値はありません。どこにも
書かれていないエンドポイントは誰も監査しないので、未設定ならルートを登録しません。
`api_doc` はその上に閲覧 UI（Scalar または Swagger UI）を追加し、`openapi` を必要とします。
空でない `api_doc` を `openapi` なしで指定すると、spec を読めないページを配信する代わりに
`server.api_doc requires server.openapi` で起動に失敗します。

`pw init` は `config.dev.toml` にのみ `api_doc = "scalar"` を書き出します。既定値は
空なので、ステージングや本番の設定に書かなければ API リファレンスは公開されません。

## 誰が読めるか

API の記述はサーフェス全体の地図です。だからどちらのパスも認証チェインの内側に
マウントされています。`auth.protection.include` は、アプリケーションのルートに対するのと
まったく同じようにこれらを覆います。

```toml
[auth.protection]
include = ["/openapi.json", "/docs"]
```

保護はオプトインなので、一致するパターンが無ければ、列挙していない他のルートと同じく
公開のままです。決して保護できないのは
[ヘルスチェックと readiness](/ja/guides/deployment/operational-endpoints/) の 2 つです。
その上には認証するものが何も無く、liveness チェックが必要とするのはまさにそれです。

UI のページは数百バイトの HTML で、インターフェース本体は公開 CDN から読み込みます。
そのためバイナリは大きくなりません。かわりにブラウザは CDN へ到達する必要があり、
UI を起動するインラインの script も実行する必要があります。自分のページ向けに書いた
Content-Security-Policy は、その両方を止めます。ページは真っ白になります。

これはエンドポイント自身が引き受けます。レスポンスにポリシーが載っていれば、この
ページが実際に必要とするものへ差し替えます。

```
script-src 'self' https://cdn.jsdelivr.net 'unsafe-inline'; style-src 'self' https://cdn.jsdelivr.net 'unsafe-inline'; img-src 'self' data:; font-src 'self' https://cdn.jsdelivr.net data:; connect-src 'self'
```

差し替えはレスポンス単位です。CDN ホストとインライン許可がこのページの外へ出ることは
なく、ほかのルートは `security.headers.content_security_policy` に書いたままの値を
返します。このキー自体を緩めていたら、両方がアプリケーションの全レスポンスに付いて
回っていました。

ポリシーを設定していないアプリケーションには、ここでもヘッダは付きません。
`content_security_policy_report_only` を使っている場合は、そちらが差し替えられます。
対処のしようがない違反でレポートが埋まることもありません。

## 何が最初から入っているか

次のようなハンドラがあるとします。

```go
type listItemsInput struct {
	Page  int    `query:"page" check:"min=1" default:"1"`
	Sort  string `query:"sort" enum:"asc,desc" default:"asc"`
	Owner string `query:"owner" check:"email"`
}

func listItems(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[listItemsInput](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	// ...
	pw.WriteAPI(w, r, items)
}
```

生成されるフラグメントには、ルート登録から得たパスとメソッド、`operationId: listItems`、
`minimum`・`enum`・`default`・`format: email` が付いた 3 つのクエリパラメータ、item
スキーマを参照する `200`、そして `ProblemDetails` を参照する `400` が入ります。最後の
1 つは、ハンドラがパース失敗を `pw.WriteProblem` に渡しているからです。

レスポンスはハンドラが実際に呼んでいるものから決まります。

| ハンドラの呼び出し | ドキュメント |
| --- | --- |
| `pw.WriteAPI` | レスポンススキーマ付きの `200` |
| `pw.WriteStatus` | 呼ばれた静的ステータスごとに 1 つ |
| `pw.WriteStream[T]` | `text/event-stream`, `application/x-ndjson`, `application/json` |
| `pw.BadRequest`, `NotFound`, `Conflict` など | そのステータス、`application/problem+json` として |
| リクエストの `check` 規則 | エラー呼び出しがなくても `400` |

`pw.WriteStatus(w, r, http.StatusCreated, value)` は、成功ステータスを明示する
`pw.WriteAPI` です — `201` や `202`、そしてボディを書かない `204`。ステータスは
リテラルか名前付き定数にしてください。実行時に計算されたステータスはスキャナに
見えませんし、ハンドラが `WriteHeader` で手動設定したステータスがドキュメントに
届くことはありません。

パスパラメータは自動的に `required` になります。ボディのフィールドは、JSON・
フォームエンコード・multipart を受け付けるリクエストボディになります。バインディングが
受け付ける 3 形式と同じです。

## より良いドキュメントにする

生成されはしますが、読みやすさは書き手次第です。3 つの習慣でほとんど決まります。

### どのみち書くべき godoc を書く

ハンドラのドキュメントコメントが operation の説明になります。**最初の 1 文**が
`summary`、**残り**が `description` です。

```go
// List the catalogue. Results are paginated and ordered by name unless the
// caller asks otherwise.
//
// The owner filter is applied before pagination.
func listItems(w http.ResponseWriter, r *http.Request) {
```

これが次になります。

```json
"summary": "List the catalogue.",
"description": "Results are paginated and ordered by name unless the caller asks otherwise.\n\nThe owner filter is applied before pagination."
```

テキストはそのまま運ばれます。生成器は書き換えも、Go の慣習である `FuncName ...` の
接頭辞の除去もしません。`// listItems lists the catalogue.` ではなく
`// List the catalogue.` と書けば、UI で読みやすい summary になり、`go doc` でも
問題なく読めます。

godoc の `Deprecated:` 段落は operation に `deprecated: true` を付けるので、UI では
取り消し線が引かれます。

```go
// Legacy is the previous listing endpoint.
//
// Deprecated: use listItems instead.
```

### エンドポイントだけでなくフィールドも説明する

構造体フィールドのドキュメントコメントはパラメータとプロパティの description に、
型のコメントはスキーマの description になります。

```go
// item is one catalogue entry.
type item struct {
	// ID is the stable identifier.
	ID int `json:"id"`
	// Name is shown to the reader.
	Name string `json:"name"`
}
```

リクエスト型もレスポンス型も同じように読まれます。次に読む Go の開発者のために書いた
コメントが、そのまま API の利用者向けの説明になります。

### 制約を宣言すれば、それが自分で自分を説明する

制約はすべて JSON Schema のキーワードに対応します。どのみち書く必要のあった
バリデーションが、そのまま契約の機械可読な部分になります。

| 宣言 | スキーマ |
| --- | --- |
| `check:"required"` | `required` に列挙 |
| `check:"min"` / `check:"max"` | `minimum` / `maximum` |
| `check:"minlen"` / `check:"maxlen"` | `minLength` / `maxLength` |
| `check:"len"` | `minLength` と `maxLength` の両方 |
| `check:"pattern=…"` | `pattern` |
| `check:"email"`, `uuid`, `date`, `time`, `datetime` | `format` |
| `enum:"a,b"` | `enum` |
| `default:"…"` | `default` |

最後の 2 つは `check` の規則ではなく独立したタグです。`check` の中に書くとエラーに
なります。[ハンドラ](/ja/guides/frontend/handlers/#デフォルト値と列挙)を参照してください。

## API に名前を付ける

各フラグメントの既定は `"<パッケージ名> API"` でバージョンは `0.0.0`、マージ後の
ドキュメントは `Application API` にフォールバックします。配信前に 1 度だけ設定します。

```go
func main() {
	if err := pw.SetOpenAPIInfo(pw.OpenAPIInfo{
		Title:   "Catalogue API",
		Version: "1.4.0",
	}); err != nil {
		log.Fatal(err)
	}
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

両方のフィールドが必須です。同じ値で 2 回呼んでも無害ですが、異なる値で 2 回呼ぶのは
エラーになります。2 つのパッケージが API の名前について黙って食い違うことはありません。

## 自分でドキュメントに触る

| 呼び出し | 用途 |
| --- | --- |
| `pw.AssembleOpenAPI()` | マージ済みドキュメントの JSON バイト列 |
| `pw.OpenAPIJSON(w, r)` | `openapi.path` の裏にあるハンドラ |
| `pw.ScalarUI(specURL)` | 任意の spec URL に対する Scalar ページ |
| `pw.SwaggerUI(specURL)` | 任意の spec URL に対する Swagger UI ページ |

自分でルートをマウントするアプリケーションが使うもので、クライアント生成のために
ビルド手順からドキュメントをファイルに書き出すのにも使えます。

```go
doc, err := pw.AssembleOpenAPI()
```

フラグメントはパスとコンポーネント名でマージされます。異なるスキーマを同じ名前で
定義した 2 つのパッケージは、黙って上書きし合うのではなく名前を変えて分離されます。
同じパッケージ ID を 2 回登録するのはエラーです。

## 対象外のもの

説明されるのは、生成器が静的に見つけられるルートだけです。生成器が追えない変数を
経由してマウントされたハンドラは現れません。サーバーレンダリングの HTML
エンドポイントも説明されません。このドキュメントが対象とするのは JSON と
ストリーミングの面、つまりクライアント生成器が使える範囲です。

バインディングとバリデーションのタグは[ハンドラ](/ja/guides/frontend/handlers/)を、書き出しの
呼び出しは[レスポンス](/ja/guides/frontend/responses/)を、フラグメントが作られるタイミングは
[`pw generate`](/ja/pw/project/generate/)を参照してください。
