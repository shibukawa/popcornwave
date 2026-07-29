---
title: pw new
description: ハンドラとそのルート、テンプレートをまとめて作る。
sidebar:
  order: 3
---

```sh
pw new [handler]
```

ハンドラには、互いに一致していなければならないものが 3 つあります。リテラルの
ルートパターン、パッケージ mux への登録、そしてページの場合はハンドラが描画する
コンポーネント名と揃ったテンプレートです。`pw new` はこれらをまとめて書きます。

[`pw add`](/ja/pw/project/add/) と同じく、既存プロジェクトの中で動き、ウィザードで
質問し、レビュー画面を承認してから書き込みます。

実装されている kind は `handler` だけです。`pw new <kind>` という形にしてあるので、
2 つめの追加コストはエントリ 1 つ、ステップ一覧 1 つ、生成分岐 1 つで済みます。

## 質問

| ステップ | 答えるもの |
| --- | --- |
| Package | ハンドラの置き場所。`generate.handlers` のディレクトリだけが並ぶ |
| Method | `GET`、`POST`、`PUT`、`PATCH`、`DELETE` |
| Path | Go 1.22 のパターン。`/tasks`、`/tasks/{id}`、サブツリーなら `/assets/` |
| Name | 関数名とファイル名の語幹。空ならルートから導出したものを使う |
| Response | HTML ページか、`pw.WriteAPI` による JSON か |
| Request input | 型付き入力と、それを埋める `pw.Parse` 呼び出しを作るか |

**Package の既定はカレントディレクトリ**です（ハンドラの用途の中にある場合）。
そこに既にいるからです。`generate.handlers` の外は候補に出しません。そこのルートは
何にも解析されず、生成される OpenAPI から抜け落ちるためです。HTML を返す場合は
さらに `generate.templates` の中である必要があります。ページテンプレートは別の用途が
読むからです。[`pw generate`](/ja/pw/project/generate/#読み込む対象) を参照してください。

## 書き出すもの

```
  create  handlers/getTasks_handler.go
  create  handlers/getTasks.pw.html
```

ハンドラは `init()` でルートを登録します。これがルート探索の読む形です。

```go
func init() { mux.HandleFunc("GET /tasks", getTasks) }

func getTasks(w http.ResponseWriter, r *http.Request) {
	pw.WriteHTML(w, r, GetTasks(GetTasksParams{Name: "World"}))
}
```

mux をまだ持たないパッケージには `index.go` も作ります。そのパッケージを `main.go`
にマウントするのはアプリケーション側の判断なので、注入せず手作業のステップとして
表示します。

書き込みのあとに [`pw generate`](/ja/pw/project/generate/) が走るので、次のビルドの
前に `_pw_gen.go` が揃います。

## 拒否する場合

ルートの重複は、対象パッケージが既に登録しているリテラルパターンを読んで、何も書く
前に検出します。

```
pw: GET /tasks is already registered in handlers/tasks_handler.go
```

既存の出力先ファイルは衝突であり、上書きしません。既存のハンドラソース、`main.go`、
ドキュメントシェルを編集することもありません。

生成ステップだけが失敗した場合、書いたソースは残します。あなたが所有して直す、
手書きの Go と `.pw.html` だからです。

## 終了ステータス

| 状況 | 終了コード |
| --- | --- |
| ハンドラを書いた | 0 |
| ウィザードをキャンセルした | 0、何も書かない |
| 端末がない | 非ゼロ、usage を表示 |
| ルート重複・衝突・不正なパターン | 非ゼロ、パスと理由を表示 |
