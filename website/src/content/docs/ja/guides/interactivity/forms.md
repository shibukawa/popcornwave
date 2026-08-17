---
title: フォーム
description: サーバー側の検証と一致するクライアント側フィードバック、フォームを送信するダイアログ、JavaScript 不要の候補リスト。
sidebar:
  order: 4
---

:::note[ブラウザの機能]
制約検証、`:user-invalid`、`<datalist>` はブラウザ由来です。`check` 規則と
描き直されるフォームはフレームワークのものです。このページの大半は、その 2 つを
食い違わせないための話です。
:::

サーバーで描画するフォームは、クライアント側のコードがなくても動きます。ブラウザが
送信し、ハンドラが検証し、エラーがあればレスポンスに表示します。ただし、必須項目の
入力漏れまでサーバーとの往復後に知らせるのは、少し遅く感じられます。

この種の即時フィードバックはブラウザに任せられます。一方、どのリクエストにも適用する
検証はサーバー側に残さなければなりません。両者の役割を混同せず、同じ規則を共有するのが
ここでの設計です。

## 検証は 2 つ、真実は 1 つ

真実はサーバーの `check` 規則です。あなたのフォームに触れていないリクエストも含め、
すべてのリクエストで走ります。

```go
type createInput struct {
	Title    string `payload:"title" check:"required,maxlen=60"`
	Owner    string `payload:"owner" check:"required,maxlen=24"`
	Priority string `payload:"priority" enum:"low,normal,high" default:"normal"`
}
```

HTML の属性は、その規則を読み手のいる場所に置いた反響です。

```html
<input id="title" name="title" value={form.title}
       required maxlength="60" autocomplete="off">
<input id="owner" name="owner" value={form.owner}
       required maxlength="24" autocomplete="off">
```

これは重複であり、払う価値があります。ただし一方向に限ります。属性はサーバーの
規則を言い直してよいのであって、規則が存在する唯一の場所であってはなりません。
サーバーの check より狭い属性はサーバーが黙って受け入れるバグになり、広い属性は
節約し損ねた往復にすぎません。

`required`、`maxlength`、`min`/`max`、`step`、`pattern`、そして入力型
（`email`、`url`、`number`、`date`）で、`check` が表現することの大半は覆えます。

### 空欄ではなく失敗にスタイルを当てる

`:invalid` は、誰もまだ入力していない空の必須フィールドにもマッチします。これで
スタイルを当てたフォームが、開いた瞬間から壊れて見えるのはそのためです。
`:user-invalid` は読み手が実際に操作するまで待ちます。

```html
<head>
<style>
.field input:user-invalid { border-color: crimson }
.field input:user-invalid + .hint { display: block }
.hint { display: none }
</style>
</head>
```

スコープ付きセレクタにはぶら下げるクラスが要ることを忘れないでください。素の
`input:user-invalid` は生成に失敗します。規則は
[ブラウザ標準の部品](/ja/guides/interactivity/browser-controls/)にあります。

## サーバーから返るエラー

クライアント側の検証は分かりきったケースを止めます。それ以外 —— 一意性や、誰かが
何かを変えるまでは妥当だった値 —— はリクエストのあとにしか分かりません。

クラシックな答えは、エラーと、読み手が書いた文字をフィールドに残したままページを
描き直すことです。`pw.Parse` は check が失敗すると型のゼロ値を返すので、その文字は
パース済みの構造体ではなくリクエストから取ります。

```go
input, err := pw.Parse[createInput](r)
if err != nil {
	mapped, ok := httpbind.AsHTTPError(err)
	if !ok || len(mapped.Fields) == 0 {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	form := FormState{Title: r.PostFormValue("title"), Owner: r.PostFormValue("owner")}
	applyFieldErrors(&form, mapped.Fields)
	pw.WriteHTML(w, r, NewTask(NewTaskParams{Form: form}))
	return
}
```

最初の分岐の区別が効いています。フィールド単位の失敗は入力欄の隣に置く価値があり、
読み取れないボディやサイズ超過はそうではありません。後者は problem レスポンスです。

`examples/htmx_fragment` は同じロジックをフラグメント経路で走らせています。そこでは
規則が 1 つ増えます。swap ライブラリは 2xx 以外を無視するので、拒否されたフォームは
**HTML と 200** で答えます。このステータスは妥当性についての嘘ではなく、「表示すべき
ものはこのレスポンスだ」という表明です。

## ダイアログの中のフォーム

`<dialog>` はどちらの種類のフォームも持てて、違いは属性 1 つです。

```html
<dialog id="rename" class="sheet">
  <form method="dialog"><button value="cancel">キャンセル</button></form>
  <form method="post" action="/tasks/rename">
    <input type="hidden" name="id" value={id}>
    <label for="title">新しいタイトル</label>
    <input id="title" name="title" required maxlength="60">
    <button type="submit">変更</button>
  </form>
</dialog>
```

`method="dialog"` は送信せずに閉じ、`returnValue` にボタンの `value` を設定します。
POST のほうはページごと離れ、Post/Redirect/Get が新しいドキュメントを持ち帰ります。
そこではダイアログは閉じています —— 一度も開かれていないからです。閉じ忘れという
概念がありません。

制約検証はダイアログの中でも普通に働きます。ブラウザは送信を拒み、問題のある
フィールドにフォーカスします。トップレイヤーの中で、こちらの手配なしにです。

考える必要があるのは、ダイアログを開いたまま拒否結果を見せたい場合です。これは
遷移ではなくフラグメントの swap になります ——
[フラグメントと島](/ja/guides/interactivity/fragments/)を参照してください。

## スクリプトなしの候補表示

```html
<label for="owner">担当</label>
<input id="owner" name="owner" list="owners" autocomplete="off">
<datalist id="owners">
{for owner in owners}
  <option value={owner}></option>
{/for}
</datalist>
```

`<datalist>` は候補であって制約ではありません。読み手はそこにない値も打てて、
それが狙いであることも多いはずです。向いているのは、ページと一緒に送れる集合
—— チームのメンバー、タグ、最近使った値 —— です。1 万行や、サーバー側で絞り込む
必要のあるリストには向きません。それはフラグメントの swap であり、同じ `input` が
`list` の代わりに `hx-get` とターゲットを持つことになります。

## ルートを 1 つにするか 2 つにするか

JavaScript なしでも動き、あればよくなるフォームは、この階段の要点そのものです。
そしてフレームワークが意図的に答えを出さない問いを連れてきます。swap による送信は、
普通の送信と同じルートに行くべきでしょうか。

フラグメントのレスポンスにクライアント由来のものは何もありません。リクエストは
分類されませんし、ヘッダが代わりに検査されることもありません。1 つのルートで
両方を賄うなら、自分でヘッダを見ます。

```go
if r.Header.Get("HX-Request") == "true" {
	pw.WriteHTMLFragment(w, r, TaskPanel(params))
	return
}
pw.WriteHTML(w, r, Page(pageParams))
```

たいていは 2 つのルートのほうが明快で、ページ経路はリダイレクトを保ったまま、
フラグメント経路はマークアップで答えられます。1 つにする価値があるのは、レスポンス
までのロジックが十分に長く、重複させるほうが分岐より悪くなるときです。
