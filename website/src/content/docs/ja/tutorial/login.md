---
title: 4. ログインする
description: ログインセッションを入れ、メモのページを保護し、アカウントごとに一覧を分ける。
sidebar:
  order: 4
---

テーブルは1つ、一覧も1つ。ページを開いた人は全員それを見ています。人ごとにメモを
分けるには、いまのアプリケーションには尋ねようのない質問への答えが要ります。
これは誰なのか。

ログインは、ふつうなら3つのルートとプロトコルのライブラリと1週間の調べ物としてやってくる
部分です。Popcorn Wave はルートを自分で提供します。アプリケーション側に残るのは、
列を1つ、絞り込みを1つ、そしてどのパスにセッションを要求するかの判断だけで、
それがこの章です。ウィザードを含めて30分ほど。

:::note[ここから始めるには]
3章の続きです。テーブルに入ったメモ、`ListMemos` と `CreateMemo` を持つ
`queries/memos.pw.sql`、一覧を描画して POST を受けるハンドラ。
:::

:::caution[この章の範囲]
扱うのは**セッションとログイン**です。ログイン状態を保つことと、あるパスにそれを
要求すること。認証方式の選択、パスワードの保管、パスキーは扱いません。それらは
[認証](/ja/guides/backend/authentication/)の担当です。
:::

## 1. 機能を追加する

```sh
pw add auth
```

ウィザードが訊くのは2つです。`auth` を選び、OIDC プロバイダを訊かれたら
**Local emulator** を選びます。書き込む前に、レビュー画面が書き込む内容を並べます。

```
  Review
    Capability     auth
    OIDC provider  Local emulator

    create  devidp.toml
    create  handlers/accounts.go
    create  migrations/00003_init_popcornwave_session.sql
    create  migrations/00004_init_popcornwave_auth.sql
    append  config.dev.toml
    append  popcornwave.toml
    edit    cmd/memoapp/main.go
    then    pw migrate up
```

`devidp.toml` は開発用ユーザーの名簿で、`Administrator` と `Member` が載っています。
`pw dev` はこれをローカルの OpenID Provider から提供し、パスワードは検証しません。
だからこそ開発以外では決して動きません。

### ログインが必要とするストレージは2種類

マイグレーションが2本来たのは偶然ではありません。ログインはサーバー側に2種類の
ものを置きます。

| 置くもの | 中身 | 担当パッケージ |
| --- | --- | --- |
| セッション | 誰がサインインしているか | `sessionstore/sqlite` |
| ceremony レコード | 進行中のログイン1回分の単発の状態 | `authstate/sqlite` |

![auth はサインイン時に一度だけ動いて外部 IdP と往復し、決めたアカウント ID を session に渡す。session はそれ以降の毎リクエストでユーザーの状態を運ぶ。auth の下に外部 IdP と authstate、session の下に sessionstore が並ぶ図](../../../../assets/diagrams/auth-and-session.svg)

**auth は「誰か」を決める役**です。外部の IdP へブラウザを送り出し、戻ってきた身元を
検証して、このアプリケーションのアカウント ID を確定します。動くのはサインインの
ときだけです。

**session は「決まった誰か」を運ぶ役**です。それ以降のすべてのリクエストで、
いま誰なのかと、その人について今なにが成り立っているかを保ちます。

セッションは、ブラウザが持つ中身のないトークンの先にあるレコードです。だから
サーバー側から失効させられます。ceremony レコードは、プロバイダへ送り出してから
戻ってくるまでの1往復を照合するためのもので、使ったら消えます。

`config.dev.toml` の `session.backend = "rdb"` が選ぶのは**どこに置くか**だけです。
**その置き場所を実際にバイナリへ入れるのは import の方**です。ストレージがオプトインで、
アプリケーションが持つのは設定したバックエンドだけ、というのはこのためです。クッキーに
置くプロジェクトはストアを1つもリンクしません（[クッキー](/ja/guides/backend/cookies/)、
`pw init --session` で最初から選べます）。

設定と import が食い違うと、起動時にこう言われます。

```
popcornwave: auth.session: session.backend = "rdb" needs its plugin;
add to the application: import _ "github.com/shibukawa/popcornwave/sessionstore/sqlite"
```

`edit cmd/memoapp/main.go` の行がそれです。承認すると、この形になります。

```go
// cmd/memoapp/main.go
package main

import (
	"context"
	"log"

	"memoapp/handlers"

	// 追加: session.backend = "rdb" を実際に提供するのがこれ。
	_ "github.com/shibukawa/popcornwave/sessionstore/sqlite"
	// 追加: 単発のログインレコードを置く先。
	_ "github.com/shibukawa/popcornwave/authstate/sqlite"

	"github.com/shibukawa/popcornwave/pw"
)

func main() {
	// 追加: Run より前に入れる。OIDC のコールバック中にフレームワークが呼ぶ。
	handlers.RegisterAccounts()
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

`pw init` で最初からログインを選んだプロジェクトでは、この3行は雛形に入っています。
`pw add` が同じものを書くのはそのためです。初期化で断った機能を後から入れても、
最初から選んだのと同じファイルに行き着く、というのがこのコマンドの約束です。

`main.go` はアプリケーションの持ち物なので、勝手に書き換えられるのは本来避けたいところ
です。それが許されるのは、書き込む前にレビュー画面がこのファイルを名指しするからです。
編集はパーサーが見つけた位置に差し込むだけで、コメントも並びも書式もそのまま残ります。

`RegisterAccounts` は、いまウィザードが書いた `handlers/accounts.go` にあります。
中身は `auth.SetAccountResolver(resolveAccount)` の1行で、選んだモードが要求する
継ぎ目をまとめて据え付ける関数です。フレームワークはプロバイダで身元を検証したあと、
それがどのローカルアカウントなのかを `resolveAccount` に尋ねます。初期版は身元そのものから
アカウントを導出するので、アカウントのテーブルを持つ前からログインできます。

## 2. 何にセッションが要るかを決める

`pw add auth` が `config.dev.toml` に足した `[auth]` は、何も保護していません。

```toml
protection.include = []
```

このリストに現れるまで、すべてのパスは公開です。メモのページはそうであるべきではありません。

```toml
protection.include = ["/", "/memos"]
protection.unauthenticated = "redirect"
```

パターンはパスのセグメント単位で照合し、末尾が `**` でない限り完全一致です。`"/"` は
ルートだけで、その下は含みません。サブツリーなら `"/memos/**"` と書きます。
このアプリケーションが実際に持つ2つのパスだけを挙げれば、`/healthz` などの運用エンドポイントは
公開のままです。ロードバランサが必要としているのはそちらです。

`popcornwave.toml` の方は、`pw add auth` が書いた内容をそのまま使います。

```toml
[dev.idp]
enabled = true
config = "devidp.toml"
port = 18080
```

`port` が入っているのは意図的です。これがなければプロバイダは空きポートを取り、
`pw dev` のたびに issuer の URL が変わります。issuer はアカウントを識別する要素の
半分で、4節で見る `resolveAccount` はアカウント ID を `issuer + "|" + subject` から
作ります。ポートが動けば同じ人に新しいアカウントが渡り、3手先で言えば、再起動のたびに
メモの一覧が空になります。すでに 18080 で何かが待ち受けているなら、ここを変えてください。

マイグレーションを適用します。

```sh
pw migrate up
```

## 3. メモに持ち主を持たせる

テーブルは持ち主という概念より前に作られています。適用済みのマイグレーションを編集するのではなく、
列の追加を別のマイグレーションにします。`migrations/00005_add_memo_author.sql` を作ります。

```sql
-- +goose Up
ALTER TABLE memos ADD COLUMN author TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE memos DROP COLUMN author;
```

既定値が効いています。3章で書いた行に持ち主はいませんし、行のあるテーブルに既定値なしの
`NOT NULL` 列は足せません。古いメモは空文字列の持ち主になります。空文字列であるアカウントは
存在しないので、テーブルには残ったまま、どのページにも出てこなくなります。

`queries/memos.pw.sql` の2つのステートメントが持ち主を取るようになります。

```sql
export statement ListMemos(author: string): sql.many<Memo> {
SELECT id, body FROM memos WHERE author = {author} ORDER BY id DESC
}

export statement CreateMemo(author: string, body: string): sql.exec {
INSERT INTO memos (author, body) VALUES ({author}, {body})
}
```

ステートメントのパラメータを変えると生成される関数のシグネチャが変わるので、新しい引数を
渡すまで呼び出し側はコンパイルできません。1章がテンプレートで見せたのと同じ境界です。

## 4. ユーザーを読む

セッションはどのハンドラよりも先にフレームワークが解決しています。`auth.User` はその結果を
答えます。

```go
// handlers/home_handler.go
import (
	"net/http"

	"memoapp/queries"

	"github.com/shibukawa/popcornwave/plugin/auth" // 追加
	"github.com/shibukawa/popcornwave/pw"
)

// home lists the signed-in account's memos.
//
// 変更: 3章の home に、ユーザーの取得と作者での絞り込みが加わる。
func home(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.User(r.Context())
	if !ok {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	var list []Memo
	// 変更: 作者が引数に増えた。ここが誰の一覧かを決めている。
	for row, err := range queries.ListMemos(r.Context(), user.AccountID) {
		if err != nil {
			pw.WriteProblem(w, r, err)
			return
		}
		list = append(list, Memo{Id: row.Id, Body: row.Body})
	}
	pw.WriteHTML(w, r, Home(HomeParams{DisplayName: user.DisplayName, Memos: list}))
}
```

手順2で設定したガードが、匿名のリクエストをこのパスから追い返しています。つまり `!ok` の
分岐には到達しないはずです。それでも書いてください。3行の値段で、
`protection.include` をあとから触ったときに `user.AccountID` が黙って空文字列になり、
持ち主のない行すべてに一致する、という事態を防げます。

`createMemo` も同じ形に変わります。先にユーザーを読み、`user.AccountID` を
`queries.CreateMemo` に渡します。

```go
// handlers/home_handler.go

// createMemo は署名した本人のメモとして1件保存する。
func createMemo(w http.ResponseWriter, r *http.Request) {
	// 変更: ここから2行。誰のメモかを決めるのがこれ。
	user, ok := auth.User(r.Context())
	if !ok {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	input, err := pw.Parse[createMemoInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	// 変更: 作者が引数に増えた。
	if _, err := queries.CreateMemo(r.Context(), user.AccountID, input.Body); err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
```

ページは名前を受け取り、出口を用意します。

```html
// handlers/home.pw.html
package handlers

type Memo {
  id: int
  body: string
}

export component Home(displayName: string, memos: Memo[]): html {
  <h1 class="text-3xl font-bold">{displayName}'s memos</h1>
  <form method="post" action="/auth/logout"><button type="submit">Sign out</button></form>
  <form method="post" action="/memos" class="mt-6 space-y-2">
    <textarea name="body" rows="3" required maxlength="200"
      class="w-full rounded-lg border border-slate-300 p-3"></textarea>
    <button type="submit"
      class="rounded-lg bg-indigo-600 px-4 py-2 font-medium text-white">Add</button>
  </form>
  <ul class="mt-8 space-y-2">
  {for memo in memos}
    <li class="rounded-lg border border-slate-200 p-3">{memo.body}</li>
  {/for}
  </ul>
}
```

サインアウトがリンクではなくフォームなのは、エンドポイントが `POST` しか受け付けないからです。
セッションを終わらせるリンクは、ブラウザの先読みや他人のページの画像タグから叩けてしまいます。
`GET /auth/logout` は `405` を返します。

## 5. ログインする

```sh
pw dev
```

いつもの出力に加えて、ループが起動したプロバイダを報告します。

```
devidp: development identity provider on http://127.0.0.1:18080; no password is checked
pw dev: identity provider http://127.0.0.1:18080
pw dev:   login screen http://127.0.0.1:18080/login
pw dev:   client pw-dev-xCD4SA98_as (secret injected as AUTH_OIDC_CLIENT_SECRET)
pw dev:   users admin, member
```

issuer とクライアント認証情報は環境変数としてアプリケーションのプロセスに注入されます。
プロバイダに関する値は、コミットされる設定ファイルには何も書かれません。

<http://localhost:8080/> を開いてください。`pw dev` が表示したホストであり、雛形の
`auth.oidc.redirect_url` が指しているホストでもあります。どちらも同じ一つの事情です。
あるオリジンで始めたログインは、別のオリジンの Cookie へ戻ってきます。検証すべきものを
何も持たないコールバックが届き、拒否されて終わります。リクエストはプロバイダの
ログイン画面へリダイレクトされ、そこに2人のユーザーが待っています。**Member** を選ぶと、
ブラウザは **Member's memos** という見出しのメモページに戻ってきます。

![Administrator と Member のアカウントが並び、パスワードを検証しないことを示す開発用 IdP のログイン画面](../../../../assets/screenshots/tutorial-login.png)

メモを1件書いてください。サインアウトして **Administrator** で入り直すと、一覧は空です。
**Member** で入り直せば、さっきのメモがあります。

その間に起きたことのうち、アプリケーションが実装したものは1つもありません。プロバイダへの
リダイレクト、コールバック、検証、アカウントの照合、セッションレコード、Cookie。
`/auth/login`、`/auth/callback`、`/auth/logout` の3つのルートは機能と一緒に来たもので、
このプロジェクトのどのハンドラもそれらに言及していません。

## あえて足りていないもの

開発用プロバイダは代役です。このアプリケーションをデプロイするなら、実在の OpenID Provider を
指定し、`AUTH_OIDC_ISSUER`、`AUTH_OIDC_CLIENT_ID`、`AUTH_OIDC_CLIENT_SECRET`、そして
`SESSION_SECRET` を環境から与えることになります。これらがなければアプリケーションは
起動を拒否します。安全でない何かにフォールバックするのではなく。

初期のリゾルバは、アカウントを保存せずに検証済みの身元から導出します。実際のアプリケーションは
アカウントのテーブルを持ち、issuer と識別クレームに紐づけ、見たことのない身元をどう扱うかを
決めます。それは[認証](/ja/guides/backend/authentication/)の担当で、リポジトリの
`examples/oidclogin` がその動く実装です。

そして `auth.User` が答えるのは*誰か*であって、*何をしてよいか*ではありません。認可は
アプリケーションに残ります。この章が WHERE 句で答えた問いも、その1つです。

## 4章かけて作ったもの

雛形から動くプロジェクトへ。規則を一度だけ宣言し、ハンドラ本体より前に効かせるフォームへ。
バージョン管理されたスキーマと、その上の型付き SQL へ。そして、プロトコルの仕事が
どこか別の場所で済んでいるログインへ。

ここまでは、ごく普通のサーバーレンダリングのアプリケーションです。専用のテンプレート言語が
なくても、ここまでは作れました。[第5章](/ja/tutorial/page-tree/)でそれが変わります。
ファイルシステムが記述するルートと、一度描画されたあともページが変わり続ける3つの方法です。
