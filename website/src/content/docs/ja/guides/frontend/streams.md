---
title: ストリーム
description: レスポンスを型付きイベントの並びとして送る。SSE、NDJSON、JSON 配列のどれになるかを決めるのはハンドラではなくクライアント。
sidebar:
  order: 3
---

レスポンスの中には、計算して送る「値」ではないものがあります。時間をかけて届く
「並び」——モデルが返すトークン、ジョブが吐く行、キューから来るイベント。それを 1 つの
ボディにまとめてしまうと、クライアントは最初のひとつを見るために最後のひとつを待つ
ことになります。

`pw.NewStream[T]` は、繰り返し書き込めるレスポンスを開きます。

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

呼び出しは 3 つだけで、残りは型パラメータが運びます。`NewStream` がワイヤ形式を
ネゴシエートし、ステータスとヘッダを確定します。`Send` は値を 1 つ書いて flush します。
`Close` がレスポンスを終わらせます。

`defer stream.Close()` は儀式ではありません。JSON 配列形式では閉じ括弧がここで書かれ、
閉じられなかったストリームはどのパーサも受け付けないボディになります。

## 形式を選ぶのはクライアント

同じハンドラが、ブラウザの `EventSource` にも、`curl` のパイプラインにも、
`fetch().then(r => r.json())` にも応えます。どれから呼ばれたかを知る必要はありません。

| 形式 | メディアタイプ | フレーミング |
| --- | --- | --- |
| Server-Sent Events | `text/event-stream` | `data: {…}` と空行 |
| NDJSON | `application/x-ndjson`、`application/ndjson`、`application/jsonl` | 1 行 1 JSON オブジェクト |
| JSON 配列 | `application/json` | 1 つの `[…]` 文書。届いた順に要素を追記 |

選択は 4 段階で、最初に答えが出たところで止まります。

1. **`?stream=`** —— 明示的な指定。`sse`、`event-stream`、`events`、`eventstream` が
   SSE、`ndjson`、`jsonl`、`nd`、`lines` が NDJSON、`json`、`array`、`json-array`、
   `jsonarray` が JSON 配列です。
2. **`Accept`** —— ヘッダの中で上の表に一致する最も左のメディアタイプが勝ちます。
3. **`User-Agent`** —— ブラウザのトークンなら SSE、`curl`・`wget`・`httpie` なら
   NDJSON。
4. **NDJSON** —— 既定値。シェルのパイプラインが 1 行ずつ読める形式だからです。

クエリパラメータがあるのは、ブラウザのアドレスバーからストリームを覗くときに、
ブラウザ自身の `Accept` に決めさせないためです。

形式ごとに必要なヘッダも付きます。SSE には `Cache-Control: no-cache`、
`Connection: keep-alive`、そして `X-Accel-Buffering: no`。最後のひとつは、前段の nginx に
「転送すべきレスポンスをバッファリングするのをやめろ」と伝えるためのものです。JSON 系の
2 つには `Cache-Control: no-cache` とそれぞれの content type が付きます。

## 受け入れられる形式がない場合

リクエストの `Accept` ヘッダがサポート対象をすべて除外している場合、ストリームは
そもそも始まりません。`406 Not Acceptable` を
[problem レスポンス](/ja/guides/frontend/responses/#エラー)として返し、以降の `Send` は
すでに送ったステータスと矛盾するボディを書く代わりに同じエラーを返します。だから上の
ループはエラー時に `return` するだけで済みます。

この判定が読むのは `Accept` だけです。`Accept: text/html` は `?stream=sse` を付けても
406 になります。指定はクライアントが受け取ると言った形式の*中から*選ぶものであり、
このクライアントはどれも受け取らないと言ったからです。

## 長時間のレスポンス

`server.write_timeout` の既定値が `0s` なのは、これが理由です。レスポンス全体への
期限は、ストリーム全体への期限になります。数分間開いたままであるべきストリームが、
並びの途中で切られてしまいます。他のルートのためにこのキーを設定するときは、ここにも
効くことを思い出してください。

`Send` は毎回 flush するので、値はバッファが埋まったときではなく送ったときにクライアントへ
届きます。それでも止められる可能性があるのは、あなたとクライアントの間にいるもの——
バッファリングするプロキシや、flush を指示されていない圧縮層です。

## 段階的な HTML レンダリングとは別物

シェルを先に送り、データが揃った領域から順に送る HTML ページは、別の仕組みです。
そちらは呼び出しではなく、組み合わせたテンプレートが決めるもので、
[非同期レンダリング](/ja/advanced/async-rendering/)で扱っています。`pw.NewStream` は
*中身*が並びであるレスポンスのためのもので、テンプレートを一切レンダリングしません。

## OpenAPI 文書に現れるもの

`pw.NewStream[T]` の呼び出し箇所も、他の型付きレスポンスと同じように生成される文書の
入力になります。オペレーションはストリーミングのサーフェスとして、`T` をイベントの
スキーマとし、ネゴシエーションが選びうる各メディアタイプにわたって記述されます。
クライアントジェネレータが読むのはこれです。
[API ドキュメント](/ja/productivity/api-documentation/)を参照してください。
