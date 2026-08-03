---
title: クッキー
description: クライアントが書き換えられるもの、改ざんを検知できるもの、暗号化されたもの——3つの保護を1つの型付き API で扱う。
sidebar:
  order: 5
---

クッキーはアプリケーションの状態のうち、クライアントが手元に持つ唯一のものです。
だから最初の問いは「どう書くか」ではなく「クライアントにどこまで許すか」になります。
Popcorn Wave はその答えを、1つの型付き API の上の3つのモードとして持ちます。

| モード | クライアントが読める | クライアントが書ける | 用途 |
| --- | --- | --- | --- |
| `session.CookiePlain` | 読める | 書ける | クライアントのものである表示設定 |
| `session.CookieSigned` | 読める | 書けない | 見えてよいが選ばせたくない値 |
| `session.CookieSealed` | 読めない | 書けない | クライアントに読ませてはいけないもの |

API はモードによって変わりません。開発中は plain で始めたクッキーも、モードを
書き換えるだけで signed に、さらに sealed になります。読み書きするハンドラは
そのままです。

## Jar を宣言する

```go
keys, err := session.ParseKeyring(os.Getenv("COOKIE_SECRET"))

type Preferences struct {
	Theme string `json:"theme"`
	Rows  int    `json:"rows"`
}

prefs, err := session.NewJar[Preferences](nil, session.JarOptions{
	Mode:   session.CookieSigned,
	Keys:   keys,
	Cookie: session.CookieOptions{Name: "pw_prefs", Secure: true, HTTPOnly: true},
	MaxAge: 30 * 24 * time.Hour,
})
```

コーデックの `nil` は JSON を意味します。`session.Codec[T]` を満たすものを渡せば
差し替えられるので、ペイロードが育ったらコンパクトなバイナリ表現をここに置きます。

ミドルウェアを1度差し込めば、クッキーはリクエストごとに1度だけデコードされます。

```go
handler = prefs.Middleware()(handler)
```

```go
func Settings(w http.ResponseWriter, r *http.Request) {
	current, ok := prefs.Read(r.Context())
	if !ok {
		current = Preferences{Theme: "system", Rows: 20}
	}
	value, _ := prefs.Value(r.Context())
	_ = value.Set(Preferences{Theme: "dark", Rows: current.Rows})
}
```

`Set` は `Set-Cookie` ヘッダをその場で書くので、ほかのヘッダ書き込みと同じく
レスポンスボディより前に置きます。書いた値はそのリクエストの以降の読み出しに
そのまま反映されます。`Clear` はクッキーを失効させます。ミドルウェアの外側——
バックグラウンドジョブやテスト——では `Load`、`Save`、`Clear` がリクエストや
ライターを直接受け取ります。

## それぞれのモードが保証すること

plain の値はペイロードの base64 です。ブラウザは `atob` 1回で読めますし、好きな
内容に差し替えられます。返ってきたものはクエリパラメータと同じリクエスト入力として
検証してください。

signed の値は HMAC を伴います。ペイロード自体はクライアントから読めるので秘密を
置く場所ではありませんが、書き換えられた値はデコードされずに拒否されます。sealed の
値は AES-256-GCM で、クライアントには暗号文しか見えません。

拒否はもう1方向にも効きます。モードは読み手が決めるので、sealed のクッキーを自分で
作れる plain に落として送ってきても、plain として読まれることはなく拒否されます。
クッキー名も、セッションレコードならそれが属するトークンも認証対象なので、値を別の
クッキーに移したり、別のユーザーに対して再生したりはできません。

その Jar が書いたのではない値や、スタンプされた寿命を過ぎた値をリクエストが運んで
きたときは、ミドルウェアがブラウザからそれを消し、何も持っていなかったリクエストと
して続行します。古くなったクライアント状態は、アプリケーションが処理すべきエラーでは
ありません。

## 秘密鍵とローテーション

秘密鍵は 32 バイト以上のランダム値を base64 で持ちます。

```bash
openssl rand -base64 32
```

`ParseKeyring` は書き込みに使う秘密鍵を先頭に、引退した秘密鍵をいくつでもその後ろに
取ります。書くのは先頭だけで、残りは読み出しには通ります。先週書かれた値を持っている
ブラウザから見てローテーションが起きなかったように見えるのは、このためです。引退した
秘密鍵をリストから外せば、それで書かれた値は一斉に受け付けられなくなります。漏洩時に
未回収のクッキーをすべて終わらせるレバーも同じものです。

## クッキーを前提に設計する前に知っておく2つの限界

ブラウザは大きすぎるクッキーを黙って捨てます。それは「値が二度と返ってこない」ように
見えます。そうならないよう、名前とエンコード後の値の合計がおよそ 3.8 KB を超えると
`Save` は `session.ErrCookieTooLarge` で書き込みを断ります。

`MaxAge` は両側で効きます。ブラウザには忘れるよう伝え、同じ期限を認証済みペイロードの
中にも刻むので、期限切れの値を送り続けるクライアントは信用されずに拒否されます。

## セッションは同じ仕組みのひとつ上

ログイン状態は Jar ではありません。それはセッションマネージャのもので、ブラウザには
不透明なトークンだけを持たせ、レコードは設定されたストレージに置きます。このページの
仕組みがそこで効くのは、ストレージをまったく必要としない出発点としてです。
`session.backend = "cookie"` は、上と同じ AES-256-GCM でレコードに封をし、自分のセッション
トークンのハッシュに結び付け、隣の2つ目のクッキーに置きます。

```toml
[session]
enabled = true
backend = "cookie"
keyring.secret = "${SESSION_KEYRING_SECRET}"
```

この秘密鍵は cookie バックエンド固有の設定ではありません。1つの鍵が `session.ReadOnly` の
スロット全部に署名し、`session.Private` のスロット全部に封をします。`rdb` や `redis` の
デプロイでも宣言します。

封も、ローテーションも、サイズの上限も同じです。ログインのときだけ効いてくる制限がひとつ
あります。封をしたレコードは失効させられません。消すべきサーバー側のコピーが無いからです。
データベースや Redis のバックエンドとの比較、それぞれに必要な設定は
[セッション](/ja/guides/backend/sessions/)にあります。
