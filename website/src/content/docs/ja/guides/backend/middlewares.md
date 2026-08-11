---
title: ミドルウェア
description: フレームワークが全リクエストに何をどの順で巻いているか、そして自前のミドルウェアをどこに差し込むか。
sidebar:
  order: 4
---

どのリクエストも、普段は目に入らないスタックを通り抜けています。目に入らないのは狙い
どおりです。panic ハンドラを自分で設置した覚えはなくても、あなたの panic はちゃんと
捕まえてくれる。ただし、レスポンスにヘッダが1つ足りないとき、ログの行に request ID が
無いとき、ヘルスプローブが `503` を返したとき、話は変わります。ソケットとハンドラの
あいだに何が挟まっているのか、知らないままでは調べようがありません。このページはその
地図です。何が入っていて、どの順で並び、どのスイッチがどれを動かし、自前のミドルウェアが
同じスタックの中で番号つきの位置をどう取るか。

## スタックを外側から

すべてのフレームは1本の番号線の上にいて、外側から内側へ昇順に並びます。フレームワーク
自身のフレームは10の倍数を占めます — BASIC の行番号と同じで、理由も同じです。隙間は
あなたのもの。各番号には export された定数(`pw.SlotRequestID`、`pw.SlotAccessLog`、…)が
あるので、位置は裸の整数ではなく名前からの相対で書けます。

| スロット | フレーム | 役割 | スイッチ |
| --- | --- | --- | --- |
| — | リクエスト追跡 | graceful shutdown のため処理中リクエストを数える | 常時 |
| 10 | OpenTelemetry | リクエストのルートスパンを開く | トレースの出力先があるときだけ |
| 20 | リソース注入 | ロガー・データベース・設定をコンテキストへ | 常時 |
| 30 | リクエスト ID | 全ログ行が携える ID を検証または発行 | `middleware.request_id` |
| 40 | アクセスログ | 1リクエスト1行、所要時間つき | `middleware.access_log` |
| 50 | recover | panic を交渉済みのエラーレスポンスへ変換 | `middleware.recovery` |
| 60 | セキュリティヘッダ | CSP や HSTS を、何かが書き込む前に | `security.headers.enabled` |
| 70 | リクエストタイムアウト | リクエスト全体を時間で縛る | `middleware.request_timeout` |
| 80 | ボディ上限 | リクエストボディの読み取り量に蓋をする | `server.max_request_body` |
| 90 | 公開アセット | 動的処理の前に静的ツリーを返す | `server.public.enabled` |
| 100 | プローブ | health と readiness を、認証より上で | `server.health`, `server.readiness` |
| 110–150 | 拡張 | ストレージ・セッション・認証・CSRF・ガード | 拡張ごと |
| 160 | API ドキュメント | OpenAPI ドキュメントと UI を、ガードの下で | `server.openapi`, `server.apidoc` |
| — | あなたのハンドラ | `pw.Run` に渡した mux | — |

リクエスト追跡だけは線の外、最外周にいます。shutdown の計数は番号つきの全ステップを
観測しなければならないからです。ハンドラは定義上の最内端。そのあいだは、番号だけが
順序を決めます。

並び順はアルファベット順でも歴史的経緯でもありません。1つ1つの位置が論証です。
リクエスト ID がアクセスログの外側にあるのは、ログの行が ID を載せられるように。
アクセスログが recover の外側にあるのは、panic したリクエストも所要時間と `500` つきで
記録に残るように。プローブが拡張チェーンより上にあるのは、セッションストアが落ちていても
liveness チェックは成功するように — 依存先の障害は readiness を落とすべきであって、
再起動ループに化けるべきではありません。そして OpenAPI ドキュメントはガードの*下*に
います。API 表面全体の地図には、そこに描かれたルートと同じ保護がかかるべきだからです。

圧縮はこれらの隣で設定しますが(`middleware.compression`)、チェーンのフレームでは
ありません。適用されるのはレスポンスを書く場所です。いつ有効にすべきかは
[圧縮のガイド](/ja/guides/backend/compression/)にあります。

## 拡張スロット

110 から 150 の区間もハードコードされていません。import された機能がそこに自分を登録し、
チェーンは同じ番号で組み上がります。

| スロット | 定数 | 登録するもの |
| --- | --- | --- |
| 110 | `pw.SlotStorage` | ストレージクライアントを開くセッションバックエンド |
| 120 | `pw.SlotSession` | フレームワークのセッション解決 |
| 130 | `pw.SlotAuthentication` | `plugin/auth` |
| 140 | `pw.SlotCSRF` | `security.csrf` が有効にする CSRF 検査 |
| 150 | `pw.SlotGuard` | 認証ガード |

番号には意図があります。150 のガードは、120 で解決されたセッションと 130 で確定した
認証を必ず観測できる。機能を提供するパッケージ — 再利用するコンポーネントパッケージでも、
自分のアプリ内のパッケージでも — は `init` から登録します。

```go
package audit

import (
	"context"
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
)

func init() {
	pw.RegisterExtension(pw.Extension{
		Name: "audit",
		Slot: pw.SlotGuard + 1, // 認証の後、ガードの後
		Setup: func(ctx context.Context) (pw.Middleware, error) {
			return func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// セッションと認証はこの上で解決済み。
					next.ServeHTTP(w, r)
				})
			}, nil
		},
	})
}
```

`Setup` は起動時に一度、設定のパースとデータベースの起動の後に走り、ハンドラが見るのと
同じリソースを受け取ります。設定を間違えた拡張は最初のリクエストではなく起動を失敗させる、
というわけです。nil のミドルウェアを返せば何も設置されません。無効化された機能は
そうやって抜けます。

## 自前のミドルウェアを作る

`pw.RegisterMiddleware` はスロット・名前・素の `func(http.Handler) http.Handler` を
受け取り、`pw.Run` と `pw.Middlewares` が組むチェーンのその位置に収まります。呼ぶのは
`main` から、全パッケージの `init` の後、チェーンが組まれる前。`pw.RegisterSessionStore`
と同じタイミングで、理由も同じです。チェーンは一度だけ組まれるので、後から登録しても
どこにも入りません。

小さなミドルウェアの一番おいしい使い道は、リクエストごとの事実を一度だけ導出して、
下の全員に読ませることです。[`session.RequestScope`](/ja/guides/backend/sessions/) は
まさにこのための配置で、代表例はリクエスト時刻です。書き込みのたびに `time.Now()` を
呼ぶハンドラは、タイムスタンプをリクエスト内に撒き散らします。1回のフォーム送信で
更新した3行が、ハンドラの処理時間ぶんずつずれた3つの `updated_at` を持つことになる。
かわりに、瞬間を一度だけ捕まえます。型から登録までのプログラム全体はこうなります。

```go
// cmd/myapp/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/popcornwave/session"

	"myapp/handlers"
)

type RequestTime struct {
	At time.Time `json:"at"`
}

func withRequestTime(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handle, ok := session.Value[RequestTime](r.Context()); ok {
			handle.Set(RequestTime{At: time.Now()})
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	// このミドルウェアはセッション状態に書き込むので、
	// 120 のセッション解決より下に置く。
	pw.RegisterSessionStore[RequestTime]("request_time", session.RequestScope)
	pw.RegisterMiddleware(pw.SlotSession+5, "request_time", withRequestTime)

	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

以後、そのリクエストの書き込みは `session.Load[RequestTime]` を `updated_at` に使い、
1回の送信は1つの瞬間を刻みます。同じ形は「ちょうど1リクエストのあいだ真である事実」
なら何にでも使えます。bearer トークンが解決するスコープ集合。リクエスト冒頭で取った
フィーチャーフラグのスナップショット — 処理の途中でフラグの切り替えが見えてしまわない
ように。

この例が開いたままにしている判断は番号だけで、それはミドルウェアが何を観測したいかで
選びます。20 より上ではコンテキストにリソースが無い。50 より下なら panic は recover が
受け止める。120 より下ならセッションは解決済み。150 より後ろにはガードが通した
リクエストしか来ない。リクエスト時刻が `pw.SlotSession+5` にいるのは、120 より上には
存在しないセッション状態へ書き込むからです。ヘッダを読むだけのミドルウェアなら
ずっと上でいい — たとえば `pw.SlotAccessLog-5` なら、30 のリクエスト ID は発行済みで、
40 のアクセスログがこの先を計時してくれます。同じ番号の2つは登録順に走るので、
順序に依存しない組なら相乗りで構いません。

登録を拒否する位置が2つあります。100 と 160、プローブと API ドキュメントです。これらは
ミドルウェアではなくハンドラで、同じ位置を誰かと分け合えません。panic は移動先の基準に
なる定数の名前を挙げます。

線の外に残る継ぎ目は1つ。`pw.Middlewares` の戻り値を包む位置で、フレームワークが
答えてしまうリクエスト — プローブ込み — まで観測したい稀なミドルウェアのためのものです。

```go
handler, err := pw.Middlewares(mux)
if err != nil {
	log.Fatal(err)
}
err = http.ListenAndServe(":8080", myOutermost(handler))
```

この位置はスタックが提供するものを全部手放します。リクエスト ID も recover も、
コンテキストのリソースも無い。生のリクエストを観測すること自体が目的のときだけ選んで
ください。それ以外は、線の上の番号が意図を語り、チェーンがそれを守ります。
