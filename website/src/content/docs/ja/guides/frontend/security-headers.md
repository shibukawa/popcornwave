---
title: セキュリティヘッダー
description: 既定のブラウザ向けセキュリティポリシー、安全に変更する方法、HTTPS を確認してから HSTS を有効にする理由。
sidebar:
  order: 9
---

何も設定しなくても、3 つのセキュリティヘッダーがすべてのレスポンスに付いています。

```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: strict-origin-when-cross-origin
```

いずれも広く安全に使える既定値です。`nosniff` は Content-Type の推測を防ぎ、`DENY` は
アプリケーションが明示的に許可するまでフレーム内の表示を拒否します。
`strict-origin-when-cross-origin` が完全なリファラを送るのは、同一オリジン内だけです。

Content-Security-Policy にも、適用範囲を絞った既定値があります。

```
script-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'
```

ほぼどのアプリケーションでも受け入れられる 4 つのディレクティブだけを縛り、そうでない
ものには触れません。画像・フォント・スタイル・通信先は制限しないので、CDN からロゴを
読むページも設定を書き換えずにそのまま動きます。

中心になるのは `script-src 'self'` です。インラインイベントハンドラ、インラインの
`<script>`、`javascript:` URL を拒否し、HTML の差し込みからコードが実行される経路を
まとめて塞ぎます。[CSRF](/ja/guides/architecture/security/) の同伴クッキーは、意図的に
スクリプトから読めるため、同一オリジンで不正なスクリプトが動けば正当なトークンを
作れてしまいます。フレームワーク自身のランタイムは同一オリジンの module タグなので、
`'self'` だけで読み込めます。

ただしこれは一次防御ではなく二次防御です。`javascript:` URL は、ヘッダを見るより前に、
書かれた場所でテンプレートが拒否します（[URL 属性](/ja/guides/frontend/templates/#属性)）。
ヘッダが効くのは、マークアップがそれ以外の経路でページに入った場合です。

サードパーティのスクリプトを読み込む場合は、その配信元を明示したポリシーに
置き換えてください。

Permissions-Policy は空のままです。使うかどうかも分からない機能について、既定値は
推測にしかならないからです。どちらかのポリシーをリバースプロキシ側ですでに管理している
場合は、ここで重ねて設定せず、設定元を一方に揃えます。

## キー

```toml
[security.headers]
enabled = true
content_type_options = true
frame_options = "deny"
referrer_policy = "strict-origin-when-cross-origin"
content_security_policy = "script-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'"
content_security_policy_report_only = ""
permissions_policy = ""
```

`content_security_policy` を設定すると、既定値への追加ではなく、ポリシー全体を
置き換えます。`"off"` を指定するとポリシーを送信しません。空文字は既定値として扱われる
ため、ポリシーを無効にする場合は `"off"` を明示してください。

`frame_options` は `deny`、`sameorigin`、`off` を取ります。`referrer_policy` は
`no-referrer`、`same-origin`、`strict-origin`、`strict-origin-when-cross-origin` です。
それ以外の値は、無視するだけのブラウザに届く前に起動を失敗させます。綴りを間違えた
ポリシーは持っていないポリシーであり、それを動いているサイトから知るのでは遅すぎます。

値は制御文字も検査されるので、設定ファイル由来の何かでヘッダが分割されることもありません。

3 つのキーには環境変数の割り当てがありません。`content_security_policy`、その
`_report_only` の対、そして `permissions_policy` です。TOML に書いてください。長く、
シェルのクォートが壊す文字を含み、diff で読めるファイルに置くべきものだからです。

## ポリシーを書く

report-only の側は、ポリシーが何を壊すかを、何も壊さないうちに知るための手段です。

```toml
[security.headers]
content_security_policy_report_only = "default-src 'self'; report-uri /csp-report"
```

両方を同時に設定できます。ブラウザは一方を強制し、もう一方については報告します。
一斉切り替えの日を作らずにポリシーを締めていく方法です。

[API ドキュメント UI](/ja/productivity/api-documentation/) を配信している場合でも、
そのためにポリシーを緩める必要はありません。このエンドポイントは自身のレスポンスに限って
必要なポリシーに差し替えるので、必要とする CDN ホストやインライン許可が他のルートに
及ぶことはありません。

## HSTS

```toml
[security.headers.hsts]
enabled = true
max_age = "8760h"   # 1 年。duration に日の単位は無い
include_subdomains = true
preload = false
```

`Strict-Transport-Security` は既定で無効で、有効にしても HTTPS で到着したリクエストに
しか送られません。平文の上で「このサイトは HTTPS のみだ」と告げるのは、その接続が
保証できない主張をブラウザに覚えさせることです。

「HTTPS で到着した」とは、直接の TLS 接続のことです。TLS を終端するプロキシの後ろでは
それが無いので、フレームワークは `X-Forwarded-Proto` を読みます——ただし
`server.trusted_proxies` に挙げられたピアからのものだけです。このリストが無ければヘッダは
無視されます。どのクライアントでも送れるヘッダだからです。

```toml
[server]
trusted_proxies = ["10.0.0.0/8"]
```

値には 2 つのガードがあります。HSTS を有効にするなら `max_age` は正であること。そして
`preload` はさらに `include_subdomains` と、最低 1 年の `max_age` を要求します。これは
ブラウザの preload リスト側の要件そのもので、満たさないヘッダを申請しても無駄になります。

`max_age` は短い値から始めて上げてください。後悔する `max_age` とは、再訪する
すべてのブラウザが期限切れまで守り続ける `max_age` です。

## 無効にする

`enabled = false` はミドルウェアそのものを登録しません。これらのヘッダをすでに付けている
ゲートウェイの後ろにいるアプリケーションのためにあります。2 つの層が異なる
`X-Frame-Options` を設定するのは、1 つの層が設定するより悪い状態です。

[運用エンドポイント](/ja/guides/deployment/operational-endpoints/)——ヘルスチェックと
readiness——については、ここの話はあまり関係しません。プレーンテキストで状態だけを返す
ものだからです。
