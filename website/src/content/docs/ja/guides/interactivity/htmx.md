---
title: htmx の統合
description: Popcorn Wave のコンポーネントを最初のページと差し替え後のフラグメントで共有し、htmx にページの一部だけを更新させる。
sidebar:
  order: 6
---

htmx を使うためのサーバー側アダプタはありません。必要なのは、すでにあるページの
一部分と同じ形の HTML を返すことです。Popcorn Wave では、その差は
`pw.WriteHTML` と `pw.WriteHTMLFragment` の 1 呼び出しに収まります。

ただし、レスポンスを短くしただけでは統合になりません。最初の描画と差し替え後で
別々のマークアップを持つと、2 つはすぐにずれます。同じコンポーネントをページと
フラグメントの両方から呼ぶことが、この組み合わせの要点です。

## htmx を読み込む

小さなアプリケーションなら、バージョンを固定した `htmx.min.js` を `public/` に置く
のが単純です。ビルド時にバイナリへ埋め込まれ、外部オリジンを CSP に追加する必要も
ありません。

```html
package templates

export component Document(children: html?): html {
<!doctype html>
<html lang="ja">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Tasks</title>
    <script defer src="/public/htmx.min.js"></script>
  </head>
  <body><slot /></body>
</html>
}
```

CDN から読むこともできます。その場合はバージョンを URL で固定し、Subresource
Integrity と `crossorigin` を付けてください。リポジトリにある
`examples/htmx_fragment` は、固定した CDN 版と `public/` へ置く版の両方を示して
います。

## 1 つのコンポーネントを 2 つのレスポンスで使う

一覧の外側の要素には、差し替え先として安定した `id` を付けます。

```html
package tasks

type Task { id: string, title: string }

export component TaskList(tasks: Task[]): html {
<ul id="task-list">
  {for task in tasks}
    <li>{task.title}</li>
  {/for}
</ul>
}

export component TasksPage(query: string, tasks: Task[]): html {
<main>
  <h1>Tasks</h1>
  <form
    action="/tasks"
    method="get"
    hx-get="/tasks/list"
    hx-target="#task-list"
    hx-swap="outerHTML">
    <label for="query">絞り込み</label>
    <input id="query" name="q" value={query}>
    <button type="submit">検索</button>
  </form>

  <TaskList tasks={tasks} />
</main>
}
```

JavaScript が動けば、htmx は `/tasks/list` のレスポンスで `#task-list` 全体を
置き換えます。動かなければ、ブラウザはフォームの `action` で `/tasks` へ普通に
遷移します。強化前の経路を残すため、`action` と `hx-get` は意図的に別々です。

ハンドラも同じ `TaskList` を呼びます。

```go
func register(mux *pw.ServeMux) {
	mux.HandleFunc("GET /tasks", tasksPage)
	mux.HandleFunc("GET /tasks/list", taskList)
}

func tasksPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	matched := findTasks(query)
	pw.WriteHTML(w, r, TasksPage(TasksPageParams{
		Query: query,
		Tasks: matched,
	}))
}

func taskList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	pw.WriteHTMLFragment(w, r, TaskList(TaskListParams{
		Tasks: findTasks(query),
	}))
}
```

`pw.WriteHTMLFragment` が返すのは `<ul id="task-list">…</ul>` だけです。ドキュメント
シェル、`head`、ラッパー、非同期境界のフレーミングは付きません。htmx 側の
`hx-target` と `hx-swap="outerHTML"` が、レスポンスの形と一致しています。

1 つのルートで両方を返したい場合は、アプリケーション側で
`r.Header.Get("HX-Request") == "true"` を判定できます。フレームワークはこのヘッダを
自動判定しません。ページとフラグメントでリダイレクトやエラー処理が違うことが多いため、
まずは別ルートにするほうが分岐を追いやすくなります。

## フラグメント側の 3 つの制約

フラグメントにはドキュメントがありません。その事実から、次の制約が決まります。

| 制約 | htmx 側での結果 |
| --- | --- |
| `head` へ寄与できない | スタイルとスクリプトは最初のページが読み込む |
| ストリーミングしない | `await` はサーバー側で完了し、待機表示は `hx-indicator` が受け持つ |
| HTML 以外の封筒を持たない | 古いレスポンスの破棄やリクエスト順序は htmx の設定で扱う |

`head` ブロックを持つコンポーネントを `pw.WriteHTMLFragment` に渡すと 500 になります。
黙ってスタイルを落とすより、差し替え前の DOM を残して失敗を見えるようにするためです。
共通規則とダイアログ、トースト、待機表示のレシピは
[フラグメントと島](/ja/guides/interactivity/fragments/)にあります。

## バリデーションエラーのステータス

htmx は通常、2xx レスポンスを差し替え、4xx/5xx はそのままでは差し替えません。
そのため、フォームの入力ミスを同じフォームへ戻す場合は、検証失敗を埋め込んだ HTML を
**200** で返します。読めないボディ、認可失敗、存在しない対象まで 200 に変える必要は
ありません。それらは本来の 4xx/5xx と problem レスポンスのままです。

```go
_, err := pw.Parse[createInput](r)
if err != nil {
	fields, ok := validationFields(err)
	if !ok {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	pw.WriteHTMLFragment(w, r, TaskForm(formWithErrors(r, fields)))
	return
}
```

これは検証結果を成功扱いするという意味ではありません。返した HTML が、いま表示すべき
レスポンスだと htmx に伝えるステータスです。別案として `htmx:beforeSwap` で 422 を
差し替え対象にできますが、その規約は全画面で統一しないとハンドラごとに挙動が変わります。

## CSRF を有効にした書き込み

`<form method="post">` には、テンプレートコンパイラが CSRF の hidden フィールドを
自動で加えます。htmx がそのフォームを送信する限り、追加設定は要りません。

一方、フォームの外にある `hx-delete` や `hx-patch` のボタンには hidden フィールドが
ありません。`security.csrf.enabled = true` のアプリケーションでは、既定名の
`pw_csrf` クッキーをリクエスト時に読み、`X-CSRF-Token` ヘッダへ移します。
`cookie_name` や `header` を変更した場合は、この 2 つの文字列も同じ値に合わせます。

```js
// public/htmx-csrf.js
const unsafe = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

function cookie(name) {
  const prefix = `${name}=`;
  const item = document.cookie.split('; ').find((part) => part.startsWith(prefix));
  return item ? item.slice(prefix.length) : '';
}

document.addEventListener('htmx:configRequest', (event) => {
  if (!unsafe.has(event.detail.verb.toUpperCase())) return;
  const token = cookie('pw_csrf');
  if (token) event.detail.headers['X-CSRF-Token'] = token;
});
```

このファイルを htmx の後に `defer` で読み込んでください。トークンを HTML から取らず、
リクエスト直前にクッキーから読むのは、別タブでログインしてセッションがローテートした
場合にも新しい値を使うためです。仕組み全体は
[セキュリティ](/ja/guides/architecture/security/#csrf-の仕組み)で説明しています。

```html
<script defer src="/public/htmx.min.js"></script>
<script defer src="/public/htmx-csrf.js"></script>
```

## React の島と同じページに置く

htmx と React を同じページで使うことはできます。ただし、同じ子ノードを両方に管理させては
いけません。htmx が差し替えるのは React ルートの外側、または React ルートを丸ごと含む
要素にします。後者では差し替え前に React を unmount し、新しく挿入された要素を mount
し直す必要があります。[React の統合](/ja/guides/interactivity/react/)では、その境界を
カスタム要素の `connectedCallback` / `disconnectedCallback` に置いています。

htmx のために Popcorn Wave の追加ビルド機能は必要ありません。ライブラリ 1 ファイルと
`pw.WriteHTMLFragment` で統合は完結します。あると有用なのは、上の CSRF イベント処理を
フレームワークが配信する小さな任意アダプタにすることです。これなら `hx-delete` ごとに
ヘッダ処理を複製せずに済み、htmx 本体をフレームワークへ組み込む必要もありません。
