---
title: Web標準
description: Popcorn WaveがHTTP上で採用するセキュリティ、認証、エラー、API、キャッシュの標準。
sidebar:
  order: 3
---

「標準準拠」と書かれていても、実際の選択がすべてアプリケーション任せなら、通信相手から
見える挙動は揃いません。このページでは、Popcorn Waveが実際に送信または検証する挙動だけを
挙げ、詳細を所有するガイドへリンクします。RFC番号は公開仕様を表します。一方、
`X-RateLimit-*` は広く使われる互換用の慣用ヘッダーであり、IETF標準ではありません。
公開されているかどうかだけが基準でもありません。ひとつのエンジンだけが実装した仕様と、どのエンジンも
実装した仕様は別物です。最後の節は、その差が採用を退けた例を扱います。

## セキュリティヘッダーとブラウザ境界

セキュリティミドルウェアがCSP、HSTS、`X-Content-Type-Options`、
`X-Frame-Options`、`Referrer-Policy`、`Permissions-Policy`を管理します。CSRF対策は
セッションに結び付いたトークンとOrigin検証を組み合わせ、Cookieポリシーが`Secure`、
`HttpOnly`、`SameSite`、署名、暗号化を管理します。

- [セキュリティレスポンスヘッダー](/ja/guides/frontend/security-headers/)
- [CSRFとデプロイ時のセキュリティ](/ja/guides/architecture/security/)
- [Cookieの保護](/ja/guides/backend/cookies/)

## 認証

ブラウザ認証にはOpenID ConnectとWebAuthnパスキーを使えます。API専用アプリケーションは
Bearer JWTを検証でき、セッションと保証レベルのポリシーが、得られた本人性をひとつの
リクエスト契約としてハンドラへ渡します。

- [認証](/ja/guides/backend/authentication/)
- [認証設計](/ja/guides/backend/authentication-design/)
- [セッション](/ja/guides/backend/sessions/)

## エラーレスポンスとレート制限

`pw.WriteProblem`はRFC 9457 Problem Detailsと安全なHTMLエラーページをネゴシエーション
します。HTTP 429はRFC 6585に従い、標準の`Retry-After`を付けられます。さらに、既存の
クライアントとの互換性のため、`X-RateLimit-Limit`、`X-RateLimit-Remaining`、
`X-RateLimit-Reset`も利用できます。ただし、こちらはIETF標準のフィールドではありません。

- [レスポンスとProblem Details](/ja/guides/frontend/responses/#エラー)
- [実行時エラーAPI](/ja/reference/runtime/#エラー)

## APIライフサイクル

非推奨と停止予定は近い概念ですが、同じ意味ではありません。RFC 9745の`Deprecation`は、
リソースの挙動を変えずに、いつから利用を推奨しないかを伝えます。RFC 8594の`Sunset`は、
いつ利用できなくなる見込みかを伝えます。`pw.LifecycleHeaders`は両方の日付をひとつの値で
受け取り、前後関係を検証してから、それぞれのRFCが定める形式へ変換します。

```go
lifecycle, err := pw.LifecycleHeaders(pw.Lifecycle{
	DeprecatedAt:     time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	SunsetAt:         time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
	DocumentationURL: "https://example.com/migrations/v2",
})
if err != nil {
	log.Fatal(err)
}
router.Handle("GET /v1/items", lifecycle(http.HandlerFunc(listV1Items)))
```

レスポンスの`Deprecation`にはStructured Field Date、`Sunset`にはHTTP-dateが入り、移行文書は
`Link`関係として示されます。このミドルウェアはライフサイクルを通知するだけで、レスポンスの
意味を変えたり、期限に達したルートを自動停止したりはしません。

## OpenAPI

`pw generate`は、登録済みハンドラ、リクエストバインディング、レスポンスライター、ストリーム、
ProblemコンストラクタからOpenAPI 3.1の操作を導出します。生成された文書と、任意のScalarまたは
Swagger UIは、アプリケーションルートと同じパスガードの内側で運用エンドポイントとして提供されます。

- [APIドキュメント](/ja/productivity/api-documentation/)
- [ハンドラと生成される契約](/ja/guides/frontend/handlers/)
- [運用エンドポイント](/ja/guides/deployment/operational-endpoints/)

## キャッシュとコンテンツネゴシエーション

HTMLは、ドキュメントシェルが公開スコープを宣言しない限り`private, no-store`です。フィンガー
プリント付きアセットはvalidatorとimmutableキャッシュを使い、ナビゲーション差分とライブ配信は
`no-store`を使います。コンテンツエンコーディングと圧縮済みアセットの選択は対応する`Vary`を
維持し、429レスポンスは保存されません。

- [レンダリングキャッシュ](/ja/guides/frontend/rendering-cache/)
- [静的アセット](/ja/guides/frontend/static-assets/)
- [圧縮](/ja/guides/frontend/compression/)
- [レスポンス](/ja/guides/frontend/responses/)

## 運用HTTP

ヘルス、レディネス、OpenAPI、APIドキュメントの各エンドポイントには、異なる可用性とアクセス
規則があります。リクエストID、ボディ制限、タイムアウト、パニック回復、圧縮、リダイレクト、
Graceful Shutdownが、アプリケーションハンドラの外側にあるHTTP境界を完成させます。

- [ミドルウェア](/ja/guides/backend/middlewares/)
- [運用エンドポイント](/ja/guides/deployment/operational-endpoints/)
- [リバースプロキシ](/ja/guides/deployment/reverse-proxy/)

## クライアントヒントを採用しない理由

`Accept-CH`は、読み手が望む状態のページを最初から返すための正攻法に見えます。サーバーが欲しいヒントを
広告し、ブラウザが次のリクエストで`Sec-CH-Prefers-Color-Scheme`を送れば、ダークを好む読み手には最初の
描画からダークが届く。スクリプトは要らず、切り替わりのちらつきも起きません。それでもPopcorn Waveは
`Accept-CH`を送信せず、`Sec-CH-*`リクエストヘッダーも読みません。この仕組みを実装しているエンジンが
ひとつしかないからです。

| ヘッダー | Chromium | Firefox | Safari |
| --- | --- | --- | --- |
| `Sec-CH-Prefers-Color-Scheme` | 93 | — | — |
| `Sec-CH-Prefers-Reduced-Motion` | 108 | — | — |
| `Sec-CH-Viewport-Width` | 97 | — | — |
| `Critical-CH` | 91 | — | — |

いずれも実験的機能の扱いのままです。初回ナビゲーションを再送させてヒントを間に合わせる`Critical-CH`に
至っては、標準化トラックにも乗っていません。これらに依存したアプリケーションは、ChromeとEdgeでは正しく
描画され、それ以外では推測に落ちます。フレームワークがヘルパーを用意しても、この差は埋まりません。狭い
経路が推奨経路に見えるようになるだけです。

決め手は、代替手段のほうが優れていることでした。CSSの`prefers-color-scheme`はOSの設定を最初の描画より
前に反映し、どのブラウザでも動き、往復も増やさず、キャッシュも痛めません。そうなるとヒントが改善できる
のは「読み手がこのサイトで明示的に上書きした場合」だけですが、その選択はCookieならどのブラウザでも
運べます。ビューポート幅はさらに分が悪い。幅は端末を回せば変わり、それを知らせるナビゲーションは発生
しないので、サーバー側の判断は読み手が画面を見ている最中に古くなります。コンテナクエリと`srcset`は要素
ごとに判断し、古くなりません。

議論を終わらせるのはキャッシュです。配色でVaryするレスポンスは表現が2つに増えるだけで、これは払える
代償です。ビューポート幅でVaryすると、ウィンドウ幅の種類だけ表現が増えます。共有キャッシュはほぼ全
リクエストで外れ、ヒントで得たものより失うもののほうが大きくなります。

禁止しているわけではありません。ハンドラが自分でヘッダーを読み、対応する`Vary`を設定することはできます。
ちらつきを実測し、キャッシュの代償を承知のうえでそうするなら、それはアプリケーションが持つべき判断です。
フレームワークはその判断を代行しない、という立場をとっています。
