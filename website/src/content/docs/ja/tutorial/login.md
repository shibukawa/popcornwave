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
    by hand call handlers.RegisterAccountResolver() in ./cmd/memoapp before pw.Run
    by hand add import _ "github.com/shibukawa/popcornwave/plugin/session/rdb" to ./cmd/memoapp
    then    pw migrate up
```

`pw add auth` はセッションをサーバー側、データベースに保存します。ログインと一緒に
マイグレーションが2つ来るのも、データベースのないプロジェクトでは動かないのもそのため
です。ブラウザが受け取るのは中身のないトークンで、その先にあるレコードはサーバーから
失効させられます。この import がデータベースバックエンドをバイナリに入れます。ストレージ
はオプトインなので、アプリケーションが持つのは設定したバックエンドだけです。残る2つの
選択肢は[クッキー](/ja/guides/backend/cookies/)にあり、`pw init --session` なら最初から
選べます。

`devidp.toml` は開発用ユーザーの名簿で、`Administrator` と `Member` が載っています。
`pw dev` はこれをローカルの OpenID Provider から提供し、パスワードは検証しません。
だからこそ開発以外では決して動きません。

**by hand** と書かれた行が、このコマンドが代わりにやらない唯一の編集です。
`cmd/memoapp/main.go` を開きます。

```go
func main() {
	// Run より前に入れる。OIDC のコールバック中にフレームワークが呼ぶ。
	handlers.RegisterAccountResolver()
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

`RegisterAccountResolver` は、いまウィザードが書いた `handlers/accounts.go` にあります。
フレームワークはプロバイダで身元を検証したあと、それがどのローカルアカウントなのかを
この関数に尋ねます。初期版は身元そのものからアカウントを導出するので、アカウントの
テーブルを持つ前からログインできます。

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

設定ファイルを開いたついでに、`popcornwave.toml` で開発用プロバイダのポートを固定します。

```toml
[dev.idp]
enabled = true
config = "devidp.toml"
# ポートを固定すると、再起動しても issuer の URL が変わらない。
port = 18080
```

これがないとプロバイダは空きポートを取り、`pw dev` のたびに issuer の URL が変わります。
issuer はアカウントを識別する要素の半分なので、ポートが変わると同じ人に新しいアカウントが
渡ります。3手先で言えば、再起動のたびにメモの一覧が空になります。

そのうえでマイグレーションを適用します。

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
	"context"
	"net/http"

	"memoapp/queries"

	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/pw"
	httpbind "github.com/shibukawa/tinybind-go"
)

func home(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.User(r.Context())
	if !ok {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	list, err := listMemos(r.Context(), user.AccountID)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	pw.WriteHTML(w, r, Home(HomeParams{DisplayName: user.DisplayName, Memos: list}))
}

func listMemos(ctx context.Context, author string) ([]Memo, error) {
	var list []Memo
	for row, err := range queries.ListMemos(ctx, author) {
		if err != nil {
			return nil, err
		}
		list = append(list, Memo{Id: row.Id, Body: row.Body})
	}
	return list, nil
}
```

手順2で設定したガードが、匿名のリクエストをこのパスから追い返しています。つまり `!ok` の
分岐には到達しないはずです。それでも書いてください。3行の値段で、
`protection.include` をあとから触ったときに `user.AccountID` が黙って空文字列になり、
持ち主のない行すべてに一致する、という事態を防げます。

`createMemo` も同じ形に変わります。先にユーザーを読み、`user.AccountID` を
`queries.CreateMemo` と、バリデーション分岐の `listMemos` に渡します。

```go
func createMemo(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.User(r.Context())
	if !ok {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	input, err := pw.Parse[createMemoInput](r)
	if err != nil {
		mapped, fieldError := httpbind.AsHTTPError(err)
		if !fieldError || len(mapped.Fields) == 0 {
			pw.WriteProblem(w, r, pw.BadRequest(err))
			return
		}
		list, listErr := listMemos(r.Context(), user.AccountID)
		if listErr != nil {
			pw.WriteProblem(w, r, listErr)
			return
		}
		pw.WriteHTML(w, r, Home(HomeParams{
			DisplayName: user.DisplayName,
			Memos:       list,
			Draft:       r.PostFormValue("body"),
			Error:       mapped.Fields[0].Message,
		}))
		return
	}
	if _, err := queries.CreateMemo(r.Context(), user.AccountID, input.Body); err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
```

ページは名前を受け取り、出口を用意します。

```html
package handlers

type Memo {
  id: int
  body: string
}

export component Home(displayName: string, memos: Memo[], draft: string, error: string): html {
<h1>{displayName}'s memos</h1>
<form method="post" action="/auth/logout"><button type="submit">Sign out</button></form>
<form method="post" action="/memos">
  <textarea name="body" rows="3">{draft}</textarea>
  {if error != ''}<p class="error">{error}</p>{/if}
  <button type="submit">Add</button>
</form>
<ul>
{for memo in memos}
  <li>{memo.body}</li>
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

<http://127.0.0.1:8080/> を開いてください。`localhost` ではなくこのホストなのは、
雛形の `auth.oidc.redirect_url` がこちらを指しているからです。リクエストはプロバイダの
ログイン画面へリダイレクトされ、そこに2人のユーザーが待っています。**Member** を選ぶと、
ブラウザは **Member's memos** という見出しのメモページに戻ってきます。

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

- [テスト](/ja/productivity/testing/) — ハンドラのテスト。ログインの全工程を1リクエストで
  済ませるヘルパーもあります。
- [プロジェクト構成](/ja/guides/architecture/project-structure/) — `handlers` 1パッケージで
  足りなくなったときの分割。
- [設定](/ja/guides/architecture/configuration/) — `dev` とデプロイ先で何が変わるか。
- [pw build](/ja/pw/project/build/) — デプロイするバイナリを作る。
