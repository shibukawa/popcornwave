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
| `pw_nozstd` | zstd のレスポンスエンコーダを外します。約 247 KB。 | あなた |
| `pw_nogzip` | gzip のレスポンスエンコーダを外します。 | あなた |
| `force_tinygo_logic` | TinyGo 用のコードパスをホスト Go でコンパイルし、TinyGo 無しでテストできるようにします。定義は tinygodriver 側で、Popcorn Wave も圧縮とマイグレーションの分岐でこの規約に従っています。 | あなた（テスト時） |
| `tinybind_no_openapi` | 生成された OpenAPI 断片をビルドから外します。定義は tinybind-go 側で、`pw generate` が書くファイルに付いてきます。 | あなた |

圧縮の2つは同じ配置のために在ります。アプリケーションの手前で既に何かが圧縮しているなら、エンコーダはリンクされて一度も走らないコードです。切っても結果を偽ることはありません。出せない coding を広告するのではなく、その coding を提示しなくなります。

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
| `wasip2`、`illumos` | `GOOS` |
| `appengine` | 現役のものは無し。`klauspost/compress` と websocket フォークが純 Go 側のスイッチとして今も見ています |

知っておく価値があるのは `gc` です。TinyGo もこれを立てます。なので、純 Go のフォールバックが `!gc` で守られている依存——「gc を名乗らないコンパイラ」向けに書かれた制約——は、TinyGo でアセンブリの側が選ばれ、リンクに失敗します。

## TinyGo と fasthttp を同時に使う

TinyGo で fasthttp ターゲットをビルドするには `fasthttp_nozstd` が要ります。今のところ誰も代わりに渡してくれません。

```bash
tinygo build -tags "fasthttp fasthttp_nozstd" -o app ./cmd/app
```

付けないと、`klauspost/compress/zstd` の arm64 アセンブリをリンカが解決できません。上の `gc` の話がそのまま効いています。解決できない2つのシンボルはどちらもデコード側で、net/http ビルドが通るのはエンコードしかせず、TinyGo の DCE が残りを落とすからです。

`-tags noasm` でも通ります。klauspost のアセンブリを純 Go の実装に差し替えるからですが、zstd 自体は残るので約 2.5 MiB 余計に払います。`fasthttp_nozstd` を選んでください。

落とす前に1つ。`middleware.compression_codings` の既定は `zstd,gzip` です。zstd の無いビルドはそうしたクライアントに identity を返します。動作としては正しいのですが、設定が言っていることとは違うので、気になるなら既定を狭めてください。

ビルドターゲットごとの実行時コストとバイナリサイズは [ビルドターゲット](/ja/guides/architecture/performance/) にあります。
