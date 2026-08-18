---
title: 1.0 への道
description: すでに設計が終わり、Go の言語機能を待っているだけの API 変更を、呼び出しの前後で並べます。
sidebar:
  order: 4
---

1.0 未満のバージョン番号は、普通どおりの意味です。表面はまだ動きますし、リリースが
名前を引っ込めることも許されています。ただし、すでに設計が終わっていて、しかも待って
いる相手がこのプロジェクトではない変更が 1 束あります。このページはそれを、今どう
書くかと、その後どう書くかを並べて記録します。当日になって驚かないために。

## 何が、何を待っているのか

Go のメソッドは自分の型パラメータを宣言できません。型付きの読み込み、型付きの書き込み、
型付きの設定アクセサ。フレームワークが型パラメータを必要とするところは、設計上の
好みが何であれ、パッケージ関数にするしかありません。

```go
// 構文以外のすべてにおいて、レシーバはストアである。
quote, err := pw.Memo(ctx, store, QuoteKey{Pair: pair}, fetchQuote)
```

言語がメソッドに型パラメータを許した時点で、これらはいずれも、最初から言おうとして
いたメソッドになります。

トリガーはリリース 1 本ではなく 2 本です。その機能を載せた Go のリリースと、**それを
載せた TinyGo のリリース**。ホストの Go でしか使えない変換は、整理ではなくビルドの
分断になります。想定は Go 1.27 と TinyGo 0.42、1.26 が 2026 年 8 月なので 2027 年 2 月
ごろ。とはいえ型パラメータ付きメソッドは確定した言語機能ではありません。ずれれば
このページもずれるだけで、それまでに壊れるものはありません。

**以下の移行はすべて追加的です。** メソッドが本体になり、既存の関数は
`// Deprecated:` 付きのラッパーとして残ります。呼び出し箇所ごとに移ってもいいし、
移らなくてもいい。コンパイルエラーが編集を強制することはありません。どれも呼び出しの
形だけの話で、保存されたもの、生成されたもの、ワイヤ上のものは何も変わりません。

## データキャッシュ

[`pw.MemoStore`](/ja/guides/backend/data-cache/) はすでにストアをハンドルへ解決します。
言語より先にそうしてあるのは意図的で、ストアが値として手元にあるので、操作はそれを
取得した行に触れないままハンドルの上へ移れます。

```go
// 現在
store, err := pw.MemoStore(r, "rates")

quote, err := pw.Memo(ctx, store, QuoteKey{Pair: pair}, fetchQuote)
if pw.MemoHas(ctx, store, key) { /* … */ }
pw.MemoSet(ctx, store, key, quote)
pw.MemoInvalidate(ctx, store, key)
```

```go
// 移行後 — 1 行目が変わらないことこそが要点
store, err := pw.MemoStore(r, "rates")

quote, err := store.Get(ctx, QuoteKey{Pair: pair}, fetchQuote)
if store.Has(ctx, key) { /* … */ }
store.Set(ctx, key, quote)
store.Invalidate(ctx, key)
```

`MemoInvalidateScope` と `MemoInvalidateTag` は型パラメータを取らないので、実は
最初からメソッドにできました。ストアの読み方が二通りに割れないよう、これらも一緒に
移します。

## Firestore のトランザクション

整理以上の価値があるのはこれです。書き込みはすでにトランザクションのメソッドなのに
型付きの読み込みはそうではないので、ひとつのトランザクションが隣り合う行で二通りに
書かれます。

```go
// 現在
tx.Store(user)
user, err := firestorebind.LoadTx[User](ctx, tx, key)
```

```go
// 移行後
tx.Store(user)
user, err := tx.Load[User](ctx, key)
```

対象は `LoadTx`・`LoadAllTx`・`QueryPageTx` の 3 つ。変わるのは綴りだけでは
ありません。トランザクション境界が引数をやめてレシーバになり、その呼び出しが
「何の内側にいるのか」を API 自身が語るようになります。

なお、トランザクションの読み込みをコンテキスト経由ではなくトランザクション値経由で
到達させるのは別の判断で、こちらはこの変更を経ても変わりません。ハンドルを
コンテキストに載せてしまうと、同じ呼び出し箇所が、どのコンテキストが届いたかによって
二つの意味を持ってしまうからです。

## DynamoDB と Firestore のハンドル

Popcorn Web はどちらのストアもラップしていないので、`On` 系はアプリケーションの
作者が文字通り自分で書く API です。そしてハンドルは具象型、つまりレシーバになるのを
待っている型です。

```go
// 現在
h, err := dynamo.Handle(ctx)
note, err := dynamobind.LoadOn[Note](ctx, h, "note", key)
err = dynamobind.StoreOn(ctx, h, "note", note)
```

```go
// 移行後
h, err := dynamo.Handle(ctx)
note, err := h.Load[Note](ctx, "note", key)
err = h.Store(ctx, "note", note)
```

最後の行をもう一度見てください。引数から型が推論できる操作は、型引数そのものが
消えます。保存も、まとめての保存も、削除も、ただの呼び出しになります。

対象は [DynamoDB](/ja/guides/storage/dynamodb/) が `LoadOn`・`LoadAllOn`・`StoreOn`・
`StoreAllOn`・`StoreReturningOn`・`RemoveOn`・`RemoveReturningOn`・`UpdateOn`・
`QueryPageOn`・`QueryOn`・`ScanPageOn`・`ScanOn`、
[Firestore](/ja/guides/storage/firestore/) が `LoadOn`・`LoadAllOn`・`StoreOn`・
`StoreAllOn`・`InsertOn`・`InsertAllOn`・`UpdateOn`・`RemoveOn`・`RemoveAllOn`・
`QueryPageOn`・`QueryOn`。隣に並ぶコンテキスト解決版
（`dynamobind.Load[Note](ctx, …)`）は今のままです。設計上レシーバを持たない側なので、
動かす理由がありません。

## 隔離されたテスト設定

```go
// 現在
testutil.Update[pw.MiddlewareConfig](config, func(middleware *pw.MiddlewareConfig) {
	middleware.CSRF.Enabled = false
})
app := testutil.Get[AppConfig](config)
testutil.Set(config, app)
```

```go
// 移行後
config.Update(func(middleware *pw.MiddlewareConfig) {
	middleware.CSRF.Enabled = false
})
app := config.Get[AppConfig]()
config.Set(app)
```

3 つのうち 2 つは、すでに引数から型を推論しています。それでも塞がっています。呼び出し
側に何も書く必要がなくても、メソッドが型パラメータを宣言すること自体ができないから
です。だから [`testutil`](/ja/productivity/testing/) の設定操作は 3 つとも一緒に
待ちます。

## 動かないもの

コンテキスト解決版のアクセサは、この先もずっと関数のままです。設計上レシーバを持た
ない側、つまりハンドルではなく `context.Context` から値を読む側だからです。
コンストラクタも同じ理由で関数のままです。

言語以上の理由で塞がっているものが 1 つあります。`sqlbind.ScanRows` が取る行カーソルは
インターフェイスで、自分が定義していない型にメソッドを生やせるパッケージはありません。
Go がどう変わっても関数のままです。これは後から誰かが調べ直すより、書いておく価値が
あります。

生成されるレイヤーも一緒に移ります。HTML ビルダーのループ・await・live・プロバイダの
各エントリ、JSON パーサの `ParseSlice` と `ParseMap`、そして `sqlbind.AppendValues`。
どれもアプリケーションが読むコードではないので、変化が見えるのは、もともと手で編集
するものではない `_pw_gen.go` の出力の中だけです。
