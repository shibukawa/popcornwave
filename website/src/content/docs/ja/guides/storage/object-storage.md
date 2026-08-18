---
title: オブジェクトストレージ
description: TinyGo でも動く tinygodriver の S3 クライアントで、アップロードを S3 互換ストレージに保存する。
sidebar:
  order: 3
---

アップロードされたファイルの置き場所は、データベースでもコンテナのディスクでも
ありません。そして保存先のほとんどが話す言葉が S3 API です。AWS S3、Cloudflare
R2、MinIO、RustFS、Wasabi のいずれも同じ API を持ちます。Go からこれを叩くのは
本来なら解決済みの問題です。

解決済みでなくなるのは TinyGo です。`aws-sdk-go-v2` は `net/http.Transport` の
API 全体を必要としますが、TinyGo ではこの型は空の構造体として宣言されています。
さらにトランスポート層が `net/http/httputil` を import しており、こちらは TinyGo
ではコンパイルすら通りません。`minio-go` はもっと手前の `net/http/cookiejar` で
失敗します。そのためオブジェクトストレージも、フレームワークのデータベースや TLS
と同じ経路をたどります。[`tinygodriver`](https://github.com/shibukawa/tinygodriver)
の `storage/s3` パッケージが S3 の REST API を直接話し、SigV4 で署名します。

```sh
go get github.com/shibukawa/tinygodriver/storage/s3@latest
```

このパッケージは TinyGo 専用ではありません。通常の Go ビルドでは `net/http` と
`crypto/tls` の上で動作し、アプリケーション側のコードは 2 つのターゲットの間で
変わりません。

## 設定

エンドポイント、リージョン、バケットはデプロイ時の設定なので、ほかの設定と同じく
登録した構造体に置きます。[アプリケーション設定](/ja/guides/architecture/configuration/)を参照してください。

```go
package storage

import "github.com/shibukawa/popcornwave/pw"

type Config struct {
	Endpoint string `help:"S3 endpoint URL; empty selects the AWS regional endpoint"`
	Region   string `help:"signing region"`
	Bucket   string `default:"uploads" help:"bucket that holds uploaded objects"`
}

func RegisterConfig() { pw.RegisterConfig[Config]("storage") }
```

```toml
[storage]
endpoint = "http://127.0.0.1:9000"
region = "us-east-1"
bucket = "uploads"
```

例外は認証情報です。これはファイルに書きません。`s3.New` が環境変数を読むから
です。`AWS_ACCESS_KEY_ID`、`AWS_SECRET_ACCESS_KEY`、`AWS_SESSION_TOKEN` に加えて、
`AWS_REGION`（または `AWS_DEFAULT_REGION`）と `AWS_ENDPOINT_URL_S3`（または
`AWS_ENDPOINT_URL`）を参照します。AWS CLI 用に設定済みのシェルならオプションは
1 つも要りませんし、認証情報を環境変数で注入するデプロイなら、コミットしうる
設定ファイルから認証情報を締め出せます。

下のクライアントが空でない設定にだけオプションを適用しているのもそのためです。
空文字列を渡すと、環境変数の値を「何もない」で上書きしてしまいます。

## プロセスにひとつのクライアント

`s3.Client` は並行利用しても安全で、内部に `http.Client` を持ちます。リクエストは
毎回組み立てるのではなく、すでにあるものを受け取るべきです。

```go
package storage

import (
	"context"
	"sync"

	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/tinygodriver/storage/s3"
)

var (
	once      sync.Once
	client    *s3.Client
	clientErr error
)

// Client returns the process-wide client, built from [storage] on first use.
func Client(ctx context.Context) (*s3.Client, error) {
	once.Do(func() {
		config := pw.Config[Config](ctx)
		var options []s3.Option
		if config.Endpoint != "" {
			options = append(options, s3.WithEndpoint(config.Endpoint))
		}
		if config.Region != "" {
			options = append(options, s3.WithRegion(config.Region))
		}
		client, clientErr = s3.New(options...)
	})
	return client, clientErr
}

// Bucket names the configured bucket.
func Bucket(ctx context.Context) string { return pw.Config[Config](ctx).Bucket }
```

`s3.New` はネットワーク I/O を行わず、認証情報・リージョン・エンドポイントを検証
するだけです。したがって設定ミスは、最初の 1 バイトが流れるときではなく、
ストレージに触れる最初のリクエストで表面化します。認証情報の欠落は
`s3.ErrNoCredentials`、リージョンの欠落は `s3.ErrNoRegion` で、どちらも通信が
プロセスの外に出る前に返ります。

| オプション | 効果 |
| --- | --- |
| `WithEndpoint` | エンドポイント URL。S3 互換サーバー向け |
| `WithRegion` | 署名リージョン |
| `WithCredentials` | 静的な認証情報 |
| `WithCredentialsFromEnv` | 環境変数から認証情報を読む |
| `WithPathStyle` | `bucket.endpoint/key` ではなく `endpoint/bucket/key` |
| `WithUnsignedPayload` | ヘッダーのみ署名し、大きなストリームをバッファしない |
| `WithTimeout` | リクエストごとのタイムアウト。既定は 60 秒 |
| `WithHTTPClient` | `http.Client` を渡す |

アドレッシングは `amazonaws.com` のエンドポイントでは仮想ホスト形式、それ以外では
パス形式が既定です。S3 互換サーバーが期待するのは後者です。

## アップロードを保存する

multipart のフィールドは、ほかの入力と同じように `httpbind.File` にバインドされます
（[ハンドラ](/ja/guides/frontend/handlers/)を参照）。`Content` はすでにメモリ上にあるので、
`bytes.NewReader` は `Put` に巻き戻せるボディを渡せます。

```go
type uploadInput struct {
	Title string        `payload:"title" check:"required,maxlen=80"`
	File  httpbind.File `payload:"file" check:"required"`
}

func upload(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[uploadInput](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	client, err := storage.Client(r.Context())
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}

	// The client controls Filename, so it travels as metadata, never as a key.
	key := "uploads/" + newObjectID() + path.Ext(input.File.Filename)

	if _, err := client.Put(r.Context(), storage.Bucket(r.Context()), key,
		bytes.NewReader(input.File.Content),
		s3.WithContentType(input.File.ContentType),
		s3.WithMetadata(map[string]string{
			"title":    input.Title,
			"filename": input.File.Filename,
		}),
	); err != nil {
		pw.WriteProblem(w, r, storageProblem(err))
		return
	}
	pw.WriteAPI(w, r, uploadResult{Key: key})
}
```

巻き戻せることは些細な話ではありません。SigV4 はペイロードのハッシュに署名するため、
`Put` はボディを 2 回読みます。`io.Seeker` を実装したボディ（`*bytes.Reader` や
`*os.File`）はハッシュを取ってから巻き戻され、それ以外はいったんメモリに
バッファされます。`WithUnsignedPayload` を使えばストリームのまま送れますが、署名が
ボディを覆わなくなる代償を払います。使うのは https の上だけにし、あわせて
`s3.WithContentLength(n)` を渡してください。長さの分からないボディは chunked で
送出され、AWS は chunked な `PutObject` を拒否します。

オブジェクトがクライアントに届くより前に、2 つの上限が効きます。
`server.max_request_body`（既定 10 MiB）と、multipart のボディ上限
`httpbind.SetMaxMultipartBodyBytes`（既定 1 MiB）です。実ファイルを受け付けるなら
両方を引き上げます。

`WithContentType` が効くのはアップロード時ではなくダウンロード時です。S3 は送られた
値を保存し、後でオブジェクトの `Content-Type` として返します。ここでその値はクライアント
由来です。保存したオブジェクトをブラウザに返すアプリケーションは、パートのヘッダーを
信用せず、自分で型を決めるべきです。

## 取り出して返す

```go
object, err := client.Get(r.Context(), storage.Bucket(r.Context()), key)
if err != nil {
	pw.WriteProblem(w, r, storageProblem(err))
	return
}
defer object.Body.Close()

if object.ContentType != "" {
	w.Header().Set("Content-Type", object.ContentType)
}
if object.Size >= 0 {
	w.Header().Set("Content-Length", strconv.FormatInt(object.Size, 10))
}
if object.ETag != "" {
	w.Header().Set("ETag", object.ETag)
}
if _, err := io.Copy(w, object.Body); err != nil {
	pw.Logger(r).Error("download interrupted", pw.String("key", key), pw.Err(err))
}
```

`Get` はレスポンスヘッダーが届いた時点で戻るので、ボディはハンドラの中に溜まるので
はなく通り抜けていきます。`Head` は転送なしで同じメタデータを取得し、
`GetRange(ctx, bucket, key, offset, length)` は一部だけを要求します。`length` が
0 以下なら末尾まで読みます。

```go
object, err := client.GetRange(r.Context(), bucket, key, 0, 1<<20)
```

このパッケージはリクエストに署名しますが、署名付き URL は発行しません。したがって
ブラウザがダウンロードするバイトはすべてアプリケーションを通ります。公開バケットを
CDN の背後に置くのが回避策で、そのリダイレクトはアプリケーション側で書きます。

## 一覧する

`List` が返すのは 1 ページです。切り詰められたページは `NextToken` を持ち、
`WithContinuationToken` でそれを戻します。

```go
var keys []string
for token := ""; ; {
	page, err := client.List(r.Context(), bucket,
		s3.WithPrefix("uploads/"),
		s3.WithMaxKeys(1000),
		s3.WithContinuationToken(token),
	)
	if err != nil {
		return err
	}
	for _, object := range page.Objects {
		keys = append(keys, object.Key)
	}
	if !page.IsTruncated {
		break
	}
	token = page.NextToken
}
```

`WithDelimiter("/")` を渡すと、一覧をツリーとしてたどれます。現在のプレフィックス
直下のキーは `Objects` に、その下のプレフィックスは `CommonPrefixes` に入ります。
`WithStartAfter` は既知のキーの続きから再開します。

## エラー

S3 のエラーコードはセンチネルに対応づけられているので、ハンドラは文字列ではなく
`errors.Is` で分岐できます。

```go
func storageProblem(err error) error {
	switch {
	case errors.Is(err, s3.ErrNoSuchKey):
		return pw.NotFound("no such object")
	case errors.Is(err, s3.ErrInvalidRange):
		return pw.Problem{
			Status:  http.StatusRequestedRangeNotSatisfiable,
			Title:   "Range Not Satisfiable",
			Code:    "range_not_satisfiable",
			Message: "the requested byte range lies outside the object",
		}
	default:
		// 5xx detail is logged in full and never reaches the client.
		return err
	}
}
```

default 節は意図的です。`ErrAccessDenied`、`ErrBadCredentials`、接続拒否はクライアント
の間違いではなく運用側の問題であり、`pw.WriteProblem` は認識できないエラーを
500 に変換して、内容は全文をログに残し、クライアントには `internal error` として
返します。[レスポンス](/ja/guides/frontend/responses/)を参照してください。

| センチネル | 発生元 |
| --- | --- |
| `ErrNoSuchKey` | 存在しないオブジェクト、および 404 全般 |
| `ErrNoSuchBucket` | 存在しないバケット |
| `ErrAccessDenied` | 拒否されたリクエスト、および 403 全般 |
| `ErrBucketExists`, `ErrBucketNotEmpty` | `CreateBucket`, `DeleteBucket` |
| `ErrInvalidRange` | オブジェクトの範囲外のレンジ |
| `ErrBadCredentials` | 署名またはキーの拒否 |
| `ErrNoCredentials`, `ErrNoRegion` | `s3.New`。リクエストの前 |
| `ErrTooManyRedirect` | エンドポイントの設定ミス |

センチネルの背後にある詳細は `*s3.Error` が運びます。ステータス、コード、メッセージ、
そしてストレージの提供元が問い合わせ時に求めるリクエスト ID です。

```go
var storageErr *s3.Error
if errors.As(err, &storageErr) {
	pw.Logger(ctx).Error("s3 failed",
		pw.String("op", storageErr.Op), pw.String("code", storageErr.Code),
		pw.Int("status", storageErr.StatusCode), pw.String("request_id", storageErr.RequestID))
}
```

なお `Delete` は存在しないキーに対しても成功します。これは S3 自体の挙動です。

## ローカル開発

S3 互換サーバーであれば何でも動き、本番との違いはエンドポイントの設定だけです。
[RustFS](https://rustfs.com/) はコマンド 1 つで起動します。

```sh
docker run -d --name rustfs -p 9000:9000 \
  -e RUSTFS_ACCESS_KEY=rustfsadmin -e RUSTFS_SECRET_KEY=rustfsadmin \
  -e RUSTFS_VOLUMES=/data rustfs/rustfs
```

```toml
# config.dev.toml
[storage]
endpoint = "http://127.0.0.1:9000"
region = "us-east-1"
bucket = "uploads"
```

```sh
AWS_ACCESS_KEY_ID=rustfsadmin AWS_SECRET_ACCESS_KEY=rustfsadmin pw dev
```

バケットは `client.CreateBucket(ctx, bucket)` で作れます。`ErrBucketExists` は前回の
実行がすでに作ったという意味なので、起動時のブートストラップを毎回実行しても
安全です。

コンテナを必要としないテストでは、クライアントを `httptest.Server` に向けます。
ほかと変わらないエンドポイントであり、AWS 以外のホストではパス形式が既定です。

```go
client, err := s3.New(
	s3.WithEndpoint(server.URL),
	s3.WithRegion("us-east-1"),
	s3.WithCredentials(s3.Credentials{AccessKeyID: "id", SecretAccessKey: "secret"}),
)
```

## TinyGo では

署名、リクエスト構築、XML デコードは共通のコードです。ビルドによって変わるのは、
リクエストがネットワークに届く経路だけです。

| ビルド | HTTP スタック（`s3.Backend`） |
| --- | --- |
| 通常の Go | `net/http` と `crypto/tls` |
| TinyGo、または `-tags force_tinygo_logic` | `tinygodriver/https`。TLS は OS 側 |

2 行目こそが、`crypto/tls` がスタブである TinyGo でこのパッケージが動く理由です。
TLS は OS が担当します。macOS では Network.framework、Windows では Schannel、Linux
では同梱の mbedTLS で、インストールすべきライブラリも同梱すべき証明書バンドルも
ありません。それ以外の TinyGo ターゲットは `https.ErrPlatformNotSupported` を返します。
`force_tinygo_logic` タグはホストの Go で同じ経路を選ぶもので、TinyGo ツールチェーン
なしにこの経路を試すための手段です。

どちらのビルドも `http.Client` にリダイレクトを追わせません。リダイレクト先は別の
ホストであり、署名は署名時のホストを含むからです。クライアントが自分でリダイレクトを
追い、ホップごとに署名し直すため、別リージョンのバケットも両方のターゲットで同じ
ように扱えます。

## このパッケージがやらないこと

| 制約 | 帰結 |
| --- | --- |
| マルチパートアップロードなし | `Put` は 1 リクエストで送るため、エンドポイントの単一リクエスト上限が適用される（AWS では 5 GiB） |
| 署名付き URL なし | ダウンロードはアプリケーション経由、または公開バケットの前段の CDN 経由 |
| 認証情報は静的な値か環境変数 | 共有認証情報ファイル、SSO、IMDS には非対応 |
| TinyGo では接続を再利用しない | `https` トランスポートはリクエストごとに接続を開くため、毎回 TLS ハンドシェイクを払う |
| オブジェクト全体の操作のみ | バージョニング、ACL、タグ、ライフサイクルの API はない |

設計を左右することが多いのは 1 つめです。任意の大きさのファイルを受け付けたい
アプリケーションが求めるのはブラウザからストレージへの直接アップロードで、それには
署名付き URL が必要になり、通常の Go でしかビルドできない別のクライアントを選ぶ
ことになります。あるいは、1 リクエストで収まる程度のアップロード上限を、正直に
掲げることです。
