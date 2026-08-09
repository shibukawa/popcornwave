---
title: ストリーム
description: レスポンスを型付きイベントの並びとして送る。SSE、NDJSON、JSON 配列のどれになるかを決めるのはハンドラではなくクライアント。
sidebar:
  order: 3
---

レスポンスの中には、計算して送る「値」ではないものがあります。時間をかけて届く
「並び」——モデルが返すトークン、ジョブが吐く行、キューから来るイベント。それを 1 つの
ボディにまとめてしまうと、クライアントは最初のひとつを見るために最後のひとつを待つ
ことになります。逆に、値全体がすでに揃っているならストリーミングには適していません。
待ち時間は変わらないまま、切断や途中まで送信したレスポンスの扱いだけが増えます。

`pw.WriteStream[T]` はレスポンスを開き、それをコールバックに渡します。

```go
func events(w http.ResponseWriter, r *http.Request) {
	pw.WriteStream(w, r, func(stream *pw.Stream[ChatEvent]) error {
		for event := range source {
			if err := stream.Write(event); err != nil {
				return err
			}
		}
		return nil
	})
}
```

`WriteStream` が転送形式を決め、ステータスとヘッダを確定し、コールバックを実行して、
どう終わろうと最後にストリームを閉じます。`Write` は値を 1 つ書いてすぐに送信します。

閉じるのはあなたではなくランタイムの仕事で、そこがこの形の要点です。JSON 配列形式は
閉じ括弧が書かれて初めて文書として成立しますが、途中で `return` するハンドラ——エラー時、
あるいはクライアントが切ったとき——はそれを書き忘れました。いまは、コールバックが
どう戻ろうと括弧は書かれます。

## 形式を選ぶのはクライアント

同じハンドラが、ブラウザの `EventSource` にも、`curl` のパイプラインにも、
`fetch().then(r => r.json())` にも応えます。どれから呼ばれたかを知る必要はありません。

| 形式 | メディアタイプ | フレーミング |
| --- | --- | --- |
| Server-Sent Events | `text/event-stream` | `data: {…}` と空行 |
| NDJSON | `application/x-ndjson`, `application/ndjson`, `application/jsonl` | 1 行 1 JSON オブジェクト |
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
[problem レスポンス](/ja/guides/frontend/responses/#エラー)として返し、コールバックは
実行されません。problem レスポンスになりうるストリームの失敗はこれだけです。開いた後は
ステータスが送信済みなので、コールバックが返したエラーはもうそれを変えられません。
そちらは `pw.SetStreamErrorHandler` で登録したハンドラに届き、途中の失敗はそこで
記録します。

この判定が読むのは `Accept` だけです。`Accept: text/html` は `?stream=sse` を付けても
406 になります。指定はクライアントが受け取ると言った形式の*中から*選ぶものであり、
このクライアントはどれも受け取らないと言ったからです。

## 長時間のレスポンス

`server.write_timeout` の既定値が `0s` なのは、これが理由です。レスポンス全体への
期限は、ストリーム全体への期限になります。数分間開いたままであるべきストリームが、
並びの途中で切られてしまいます。他のルートのためにこのキーを設定するときは、ここにも
効くことを思い出してください。

`Write` は毎回 flush するので、値はバッファが埋まったときではなく送ったときにクライアントへ
届きます。それでも止められる可能性があるのは、あなたとクライアントの間にいるもの——
バッファリングするプロキシや、flush を指示されていない圧縮層です。

## 段階的な HTML レンダリングとは別物

シェルを先に送り、データが揃った領域から順に送る HTML ページは、別の仕組みです。
そちらは呼び出しではなく、組み合わせたテンプレートが決めるもので、
[非同期レンダリング](/ja/guides/cross-layer/async-rendering/)で扱っています。`pw.WriteStream` は
*中身*が並びであるレスポンスのためのもので、テンプレートを一切レンダリングしません。

## OpenAPI 文書に現れるもの

`pw.WriteStream[T]` の呼び出し箇所も、他の型付きレスポンスと同じように生成される文書の
入力になります。オペレーションはストリーミングのサーフェスとして、`T` をイベントの
スキーマとし、ネゴシエーションが選びうる各メディアタイプにわたって記述されます。
クライアントジェネレータが読むのはこれです。
[API ドキュメント](/ja/productivity/api-documentation/)を参照してください。
