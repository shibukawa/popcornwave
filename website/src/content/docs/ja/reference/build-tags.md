---
title: ビルドタグ
description: Popcorn Wave、tinygodriver、tinybind-go の3リポジトリが定義するビルドタグの一覧。何を選ぶか、誰が渡すか、どれをツールチェインが勝手に立てるか。
sidebar:
  order: 9
---

Popcorn Wave は Go のプロジェクトとしてはビルドタグをかなり重く使っています。1つのソースツリーから、ランタイムをほとんど共有しないバイナリが出てくるからです。net/http と fasthttp、ホスト Go と TinyGo、開発ビルドと出荷ビルド。あるファイルがそのどれに属してどれに属さないかを決めるのがタグです。

渡す可能性のあるタグは3つのリポジトリに散っていて、しかも1回のビルドで混ざります。ここに全部並べます。

表では言えないことが2つあるので、先に書きます。

**タグが除外するのはファイル単位で、呼び出し単位ではありません。** なので net/http のハンドラを持つファイルは、fasthttp ビルドが必要とするものを何ひとつ持てません。型もヘルパーも mux の配線も別ファイルに出ます。`pw` の下のパッケージの配置は、ほぼこの制約で決まっています。

**タグの付かないファイルは全ビルドに入ります。** したがって、そこで名指してよいのは全ビルドがリンクするパッケージだけです。`pw` はそれではありません。タグ無しのファイルが `pw` を import すると、その1行で net/http ランタイム一式が fasthttp バイナリに入ります。代わりに `pwruntime`、`pwconfig`、`pwsession`、`pwdatabase`、`pwobservability`、`pwextension`、`pwratelimit`、`pwbrowser` を使ってください。`pw` が再エクスポートしている元がそこにあります。

## Popcorn Wave

| タグ | 選ぶもの | 渡す人 |
|---|---|---|
| `fasthttp` | 第二のビルド。`pwfast` が `pw` を置き換え、バイナリは `pw` を一切リンクしません。`popcornwave.toml` の `project.fasthttp = true` が前提です。 | `pw build --target fasthttp` |
| `pwdev` | 開発側の半分。dev コンソール、storybook、dev data、`--pw-print-dsn`。 | `pw dev`、`pw storybook`、`pw migrate` が `go run` に `-tags=pwdev` を渡します |
| `force_tinygo_logic` | TinyGo 用のコードパスをホスト Go でコンパイルし、TinyGo 無しでテストできるようにします。定義は tinygodriver 側で、Popcorn Wave も圧縮とマイグレーションの分岐でこの規約に従っています。 | あなた（テスト時） |
| `tinybind_no_openapi` | 生成された OpenAPI 断片をビルドから外します。定義は tinybind-go 側で、`pw generate` が書くファイルに付いてきます。 | あなた |

`pw_nozstd` と `pw_nogzip` は削除し、`middleware.compression` に一本化しました。
レスポンスエンコーダを落とすタグでしたが、それが要ったのは zstd を持つことが
エンコーダの10倍あるデコーダをリンクすることだった頃の話です。いまはエンコーダが
独立したパッケージになり、2つ合わせて 387 KB。問いより小さい数字になりました。
渡してもエラーにはなりません。何も選ばないだけです。

## tinygodriver

| タグ | 選ぶもの |
|---|---|
| `force_tinygo_logic` | ネイティブ実装をホスト Go で動かし、TinyGo 無しでテスト可能にします。`tinygo` と対で使われ、`!tinygo && !force_tinygo_logic` が標準側、`(tinygo \|\| force_tinygo_logic)` がネイティブ側です。 |
| `fasthttp_nozstd` | fasthttp フォークから `klauspost/compress/zstd` を落とします。TinyGo ビルドでは**必須**です。後述します。 |
| `darwinstarttlswith13` | macOS の TLS バックエンドを両方とも vendored mbedTLS に差し替えます。TLS 1.3 とクライアント証明書が手に入る代わりに、OS のトラストポリシーを失います。 |
| `nosqlite` | SQLite の amalgamation をリンクさせません。付けないと、import しただけで SQLite がコンパイルされます。 |
| `jwt_no_rsa` | JWT パッケージから RSA を落とします。 |
| `nopgxregisterdefaulttypes` | pgx の既定型登録を飛ばします。pgx 本家のタグが vendored コピーを通って出てきたものです。 |

## tinybind-go

| タグ | 選ぶもの |
|---|---|
| `tinybind_no_openapi` | 生成された OpenAPI 断片を除外します。定義元はここで、Popcorn Wave の生成ファイルがそれを持っています。 |
| `goexperiment.jsonv2` | ベンチマーク用のフィクスチャだけ。アプリケーションが立てるものではありません。 |

## ツールチェインが立てるタグ

制約の中で目にはするが、自分では渡さないものです。

| タグ | 立てるもの |
|---|---|
| `tinygo` | TinyGo |
| `gc` | gc コンパイラ、**そして TinyGo** |
| `scheduler.threads` | TinyGo が `-scheduler=threads` から導出します |
| `wasip2`、`illumos` | `GOOS` |
| `appengine` | 現役のものは無し。`klauspost/compress` と websocket フォークが純 Go 側のスイッチとして今も見ています |

知っておく価値があるのは `gc` です。TinyGo もこれを立てます。なので、純 Go のフォールバックが `!gc` で守られている依存——「gc を名乗らないコンパイラ」向けに書かれた制約——は、TinyGo でアセンブリの側が選ばれ、リンクに失敗します。

## TinyGo の `-scheduler=threads`

これは `-tags` に渡す値ではありません。TinyGo が自分の `-scheduler` フラグから `scheduler.threads` というビルドタグを導出していて、そのおかげでフレームワーク側は「付け忘れ」をコンパイルエラーにできます。

```bash
tinygo build -scheduler=threads -o app ./cmd/app
```

**ネットワークプロトコルを話すデータベースエンジンは、例外なくこれを要求します。** 協調スケジューラの下ではブロッキングするソケット呼び出しがランタイム全体を握ってしまい、ドライバのキャンセル監視が走りません。500ms のデッドラインの下でサーバ側 5s の sleep を投げたら、5秒まるごと待ってから nil エラーで返ってきました。ログにも何も出ません。

なので `database/postgres` と `database/mysql` は、これ無しではコンパイルを拒否します。判定は import グラフに紐づいているので、そのエンジンをリンクするプログラムだけで、どうビルドを起動したかによらず発火します。診断は、存在しない識別子の名前そのものです。

```
undefined: build_this_program_with_tinygo_scheduler_threads
```

`pw build` は TinyGo を駆動しないので、コマンドラインで代わりに渡してくれるものはありません。`pw init` が書く `Dockerfile.tinygo` には最初から入っています。

## TinyGo と fasthttp を同時に使う

tinygodriver v1.2.4 以降、ターゲットのタグ以外に要るものはありません。

```bash
tinygo build -tags fasthttp -scheduler=threads -o app ./cmd/app
```

v1.2.4 より前は `fasthttp_nozstd` も要りました。付けないと `klauspost/compress/zstd` の arm64 アセンブリをリンカが解決できず、上の `gc` の話がそのまま効いていたためです。解決できなかった2つのシンボルはどちらもデコード側で、net/http ビルドがずっと通っていたのはエンコードしかせず、DCE が残りを落としていたからでした。いまは fasthttp フォークが TinyGo の下で tinygodriver 自身の zstd を通してエンコードするので、到達すべき klauspost のアセンブリがそもそも残っていません。

`fasthttp_nozstd` は今も効きますが、節約は約 40 KB です。渡す理由はほとんどありません。`-scheduler=threads` の方は、ネットワーク越しのデータベースドライバをリンクしないバイナリでのみ外せます。前節を見てください。

ビルドターゲットごとの実行時コストとバイナリサイズは [ビルドターゲット](/ja/guides/architecture/performance/) にあります。
