---
title: 2. フォームとバリデーション
description: POST を受け取り、入力が満たすべき規則を宣言し、弾いたフォームを人に向かって説明する。
sidebar:
  order: 2
---

1章で残ったのは、挨拶を返し、保存のたびにリロードするプロジェクトでした。
クエリパラメータを読むのと、人が入力したものを受け取るのとは別の話です。
人は何も書かずに送信ボタンを押しますし、200行を貼り付けることもあります。

この章でフォームを足します。メモの置き場所はいったんメモリの上です。テーブルに移すのは
3章なので、変更は3ファイル、20分ほどで終わります。

:::note[ここから始めるには]
1章の続きです。`pw init memoapp` を実行し、`handlers/home.pw.html` が雛形の
`Home(name: string)` に戻っている状態。1章を飛ばした場合、`pw init memoapp` を
実行すればちょうどその状態になります。
:::

## 1. フォームのあるページ

ページの仕事が2つになりました。書かれたものを並べることと、次の1件を書く箱を出すこと。
`handlers/home.pw.html` を置き換えます。

```html
package handlers

type Memo {
  id: int
  body: string
}

export component Home(memos: Memo[], draft: string, error: string): html {
<h1>Memos</h1>
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

3つのものが同時に入りました。`type Memo` は Go の構造体になる複合型の宣言です。
テンプレートが描画する行とハンドラが組み立てる行が1つの型になるので、2つの型を
足並みそろえて保守する必要がありません。`Memo[]` はそのスライスです。
`draft` と `error` は、まだ書いていない場合のためにあります。弾かれて戻ってきた送信を、
入力した文字を箱に残したまま表示する場合です。

`.pw.html` の条件は真偽値だけで、真偽値らしさのような概念はありません。
エラーの判定が `error` ではなく `error != ''` なのはそのためです。

## 2. 置き場所

メモはリクエストをまたいで残る必要があります。1章ぶんならミューテックス付きの
スライスで足ります。

```go
// handlers/memos.go
package handlers

import "sync"

// memos はメモリ上のリスト。3章でテーブルに置き換わる。
var memos = &store{}

type store struct {
	mu     sync.Mutex
	nextID int
	items  []Memo
}

func (s *store) list() []Memo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Memo(nil), s.items...)
}

func (s *store) add(body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	s.items = append([]Memo{{Id: s.nextID, Body: body}}, s.items...)
}
```

ここでの `Memo` はテンプレートが宣言した型です。表示用の型と保存用の型の変換が
どこにもないのは、この規模では型が1つしかないからです。2つ目の型と、それに伴う変換は
3章で出てきます。

## 3. ルートを2つ

```go
// handlers/home_handler.go
package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
)

func init() {
	mux.HandleFunc("GET /{$}", home)
	mux.HandleFunc("POST /memos", createMemo)
}

func home(w http.ResponseWriter, r *http.Request) {
	pw.WriteHTML(w, r, Home(HomeParams{Memos: memos.list()}))
}

type createMemoInput struct {
	Body string `payload:"body" check:"required,maxlen=200"`
}

func createMemo(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[createMemoInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	memos.add(input.Body)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
```

雛形は `GET /` でした。Go の mux では、これはより具体的なパターンが拾わなかった
すべてのパスにマッチします。`GET /{$}` はルートだけにマッチするので、URL を打ち間違えたら
メモ一覧ではなく 404 が返るようになります。

`payload:"body"` はクエリ文字列ではなくリクエストボディを読みます。フォームは
`application/x-www-form-urlencoded` で送りますが、同じ宣言のまま API クライアントからの
JSON ボディも受け取れます。分岐は増えません。`check` の規則は生成時にコンパイルされるので、
`required` も `maxlen=200` もリクエスト時のリフレクションを必要としません。
規則の一覧は[ハンドラ](/ja/guides/frontend/handlers/#バリデーション)にあります。

リダイレクトは `303 See Other` で、見た目より重要です。POST にページを返すと、
ブラウザは再送信できるリクエストを抱えたままになります。リロードすればメモは2件になります。
`GET /` へのリダイレクトを返せば、リロードは一覧を読み直すだけです。

保存してください。`pw dev` が再生成、リビルド、再起動します。箱に何か書いて **Add** を
押せば、一覧に出ます。

## 4. 空のまま送信する

箱を空にしたまま **Add** を押してください。ブラウザにはこれが出ます。

```json
{
  "code": "validation_failed",
  "detail": "Validation failed",
  "errors": [{ "field": "body", "location": "payload", "message": "required" }],
  "status": 400,
  "title": "Validation failed",
  "type": "about:blank"
}
```

これは正しい応答で、API クライアントに対してはこれが正解です。RFC 9457 の
Problem Details、ステータス 400、問題のあるフィールドの名前。`pw.WriteProblem` は
バインディングの失敗をここまで写し取ります。もしエラーを包んでいたら —— 雛形のハンドラが
そうしているように `pw.BadRequest(err)` と書いていたら —— ステータスは同じでも
`errors` は消えます。包んだ側がバリデーションエラーを持ち歩かずに置き換えるからです。

一方、フォームを打ち間違えた人にとって、JSON 文書は行き止まりです。必要なのは、
フィールドの隣にメッセージが出て、入力した文字が残ったままのページです。
宣言はそのままに、失敗をどう扱うかをハンドラが決めます。

```go
import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
	httpbind "github.com/shibukawa/tinybind-go"
)

func createMemo(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[createMemoInput](r)
	if err != nil {
		mapped, fieldError := httpbind.AsHTTPError(err)
		if !fieldError || len(mapped.Fields) == 0 {
			// フィールド単位の失敗ではない。読めないボディか、大きすぎるボディ。
			// ページ上に描き直して意味のあるものが何もない。
			pw.WriteProblem(w, r, pw.BadRequest(err))
			return
		}
		pw.WriteHTML(w, r, Home(HomeParams{
			Memos: memos.list(),
			Draft: r.PostFormValue("body"),
			Error: mapped.Fields[0].Message,
		}))
		return
	}
	memos.add(input.Body)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
```

2つの状況を分けているのが `httpbind.AsHTTPError` です。`check` の失敗は想定内の通信で
—— 誰かが空の箱を送っただけで —— ページの上に出すべきものです。そもそもボディが
読めなかったのはそうではないので、問題文書のまま返します。

描き直す文字が `input` ではなく `r.PostFormValue` から来ているのには理由があります。
`check` が落ちたとき `pw.Parse` はゼロ値を返すので、打ち込まれた文字が残っているのは
リクエスト自身だけです。バインディングがすでにボディを解析しているため、読み直しは
ただ同然です。

保存して、もう一度空のまま送信すると、箱の下に `required` が出たページが返ります。
200文字を超えて送れば、メッセージがそれに応じて変わります。

正直に書いておくと、ここには1つ限界があります。`pw.WriteHTML` はステータスコードを
受け取らず、`200` を返します。つまり上の応答は、入力を弾いたことを伝えるページを、
成功のステータスで送っていることになります。ブラウザのフォームとしては問題ありません
（どちらでも描画されます）。同じルートを叩く API クライアントには問題文書が要る、
というのが JSON 側の分岐の意味です。

## ここまでで手元にあるもの

- POST するフォーム、それを受けるルート、リロードに耐えるリダイレクト。
- 構造体に宣言され、ハンドラ本体より前にコンパイル済みの規則として効くバリデーション。
- 1つの失敗に対する2通りの応答。クライアントには問題文書、人にはページ。

一覧は再起動のたびに消えたままです。3章でテーブルを与えます。

- [3. メモを保存する](/ja/tutorial/database/) — 次の章。
- [ハンドラ](/ja/guides/frontend/handlers/) — 取得元タグと check 規則の全体。
- [レスポンス](/ja/guides/frontend/responses/) — 問題詳細、JSON、フラグメント。
