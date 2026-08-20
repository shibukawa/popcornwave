---
title: CBOR API ボディ
description: 全 API エンドポイントが JSON と並んで application/cbor を読み書きするようになる生成時スイッチと、フォーマットを絞るプロファイルキー。
sidebar:
  order: 7
---

```toml
[generate.api.cbor]
enabled = true
```

`popcornweb.toml` にこれを書いて `pw generate` を実行すると、すべての API
エンドポイントが JSON と並んで [CBOR](https://www.rfc-editor.org/rfc/rfc8949.html)
を話すようになります。`Content-Type: application/cbor` で届いたリクエストはボディを
CBOR としてデコードし、リクエストの `Accept` ヘッダが `application/cbor` を名指し
していればレスポンスも CBOR で返します。それ以外は何も変わりません。JSON・フォーム・
マルチパートのクライアントは今までどおりのバイト列を受け取り、同じハンドラが全員を
さばきます。

デフォルトで無効なのは、ほとんどの API は CBOR クライアントに出会わないうえ、この
スイッチがタダではないからです。API 型ごとに生成されるコーデックのコードがおよそ
倍になり、TinyGo や wasm のビルドはそれをサイズとして感じます。切ったままなら
`pw generate` は今日と同一バイトの出力を再現し、バイナリに CBOR のコードは一切
リンクされません。クライアントが組み込みデバイスや wasm モジュールで、バイナリ
ペイロードが base64-in-JSON に勝つ場面が来たら入れてください。

## ハンドラは変わらない

```go
func createUser(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[CreateUserRequest](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	user, err := store.Create(r.Context(), input)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	pw.WriteAPI(w, r, user)
}
```

ごく普通のハンドラですが、実はこれで CBOR 対応は全部済んでいます。ネゴシエーションは
`pw generate` が `CreateUserRequest` と `user` の型に対して出力するコードの中に
あり、JSON コーデックを生み出すのと同じ呼び出し箇所が CBOR コーデックも生み出す
からです。JSON クライアントは一方の分岐を、CBOR クライアントはもう一方を通ります。

```bash
curl http://localhost:8080/users \
  -H 'Content-Type: application/cbor' \
  -H 'Accept: application/cbor' \
  --data-binary @user.cbor
```

リクエスト側では、ボディはテキストキーの CBOR map 1 つで、JSON ボディが埋めるのと
同じフィールドを埋めます。`payload` タグ、ネストした構造体、バリデーション、その他の
[リクエストバインディング](/ja/reference/request-binding/)はまったく同じに振る舞い、
未知のキーは JSON デコーダと同じように読み飛ばされます。path・query・header・cookie の
入力はその横で変わらずバインドされます。`+cbor` の構造化構文サフィックス
（たとえば `application/senml+cbor`）も CBOR として扱われます。

レスポンス側のルールは意図的に狭くしてあります。**CBOR が返るのは `Accept` ヘッダが
`application/cbor` を明示的に名指ししたときだけ**で、ワイルドカードは数えません。
ブラウザの `fetch` はデフォルトで `Accept: */*` を送るので、ワイルドカードで
フォーマットが切り替わる仕様だと、世のブラウザコードすべてに予期しないバイナリボディを
握らせることになります。CBOR が欲しいクライアントはそう言う。言わなければ JSON の
まま。レスポンスには `Vary: Accept` が付くので、共有キャッシュは 2 つの表現を別の
エントリとして扱います。

Problem ドキュメントは意図的な例外です。バリデーション失敗も 500 も、CBOR
クライアントに対して `application/problem+json` で答えます。problem ライタは
「失敗してはならないパス」に載っており、JSON のエラーはどのクライアントもすでに
パースできるからです。

## プロファイルキー

生成されるコーデックは CBOR のサブセットを実装しており、2 つのキーでさらに絞れます。

```toml
[generate.api.cbor]
enabled = true
reject_floats = false
sorted_keys = false
```

どちらもデフォルトはオフで、普通の API はオフが正解です。

`reject_floats = true` は `float64` フィールドを生成時エラーにし、リクエストボディに
浮動小数点数が届いたらデコードエラーにします。頼む理由がなさそうな制限に見えますし、
たいていのスキーマでは実際そうです。ただ、金額や固定小数点のセンサ値をスケール済み
整数で運ぶプロトコルでは、丸めを黙って持ち込みかねないフィールド種別を、将来の編集者の
記憶力ではなくビルドに拒否させたくなります。そのためのキーです。

`sorted_keys = true` は map のメンバを構造体フィールド順ではなく RFC 8949 の
bytewise キー順で出力します。決定論的エンコーディングを検証するクライアント向けです。
ビルド時の帰結が 1 つあります。`map[string]T` フィールドは生成時に順序を約束できない
ため、このキーが立っている間は生成エラーになります。

プロファイルは生成フィンガープリントの一部です。プロファイルについて意見の合わない
2 台のマシンは異なる生成コードを作り、それは `pw check` がドリフトとして報告します。
つまりプロファイル変更はコミットされる目に見える出来事であって、誰かのフラグが
こっそり起こすものではありません。

## ボディ上限

CBOR ボディはデコード前に丸ごと読み込まれるため、その上限は転送サイズではなく
デコードメモリに答えるものです。だから `server.max_request_body` とは別のキーを
持ちます。

```toml
[server]
cbor_max_body = 4194304
```

デフォルトは 1 MiB で、net/http と fasthttp の両ランタイムが同じ値を共有し、`0` は
その 1 MiB を保ちます。超過したボディはデコードが始まる前に 413 の problem
ドキュメントで拒否されます。

## 落とし穴

`payload:"*"` の [rest マップ](/ja/reference/request-binding/#rest-マップ) には対応する
CBOR の形がなく、そのルートを黙って JSON 専用にする代わりに、生成がエラーとして
報告します。そのルートの入力型を分割するか、そのプロジェクトでは CBOR を切って
ください。

明示的な `Accept` を付け忘れたクライアントには JSON が返ります。ネゴシエーション
ルールが正しく働いているだけなのですが、開発中は「CBOR が効いていない」ように
見えます。何より先にリクエストヘッダを確認してください。

[OpenAPI ドキュメント](/ja/productivity/api-documentation/)には、スイッチを入れた
時点で `application/cbor` のリクエスト・レスポンス content が自動的に載ります。
生成クライアントや API コンソールは、別のアノテーションなしでこのフォーマットを
見つけられます。

## 使わないほうがよいとき

クライアントが全部ブラウザで、`fetch` と JSON を話しているなら、切ったままにして
ください。ブラウザにネイティブの CBOR エンコーダはなく、JSON パスはすでに
リフレクションなしの生成コードで、スイッチはどのリクエストも通らない分岐にバイナリ
サイズを費やすだけです。圧縮ガイドの理屈の縮小版がここでも成り立ちます。誰も使わない
能力は中立ではなく、重さです。
