---
title: Object storage
description: Storing uploads in S3-compatible object storage with tinygodriver's TinyGo-capable S3 client.
sidebar:
  order: 3
---

Uploads belong neither in the database nor on the container's disk, and the S3
API is what almost every store speaks — AWS S3, Cloudflare R2, MinIO, RustFS,
Wasabi. Reaching it from Go is normally a solved problem.

TinyGo is where it stops being one. `aws-sdk-go-v2` reaches for the full
`net/http.Transport` API, which TinyGo declares as an empty struct, and its
transport layer imports `net/http/httputil`, which does not compile there at
all; `minio-go` fails earlier still, on `net/http/cookiejar`. Object storage
therefore takes the same route as the framework's database and TLS support:
[`tinygodriver`](https://github.com/shibukawa/tinygodriver), whose
`storage/s3` package speaks the S3 REST API directly and signs it with SigV4.

```sh
go get github.com/shibukawa/tinygodriver/storage/s3@latest
```

Nothing about the package is TinyGo-only. On a standard Go build it runs over
`net/http` and `crypto/tls`, and the code you write does not change between the
two targets.

## Configuration

Endpoint, region, and bucket are deployment settings, so they belong in a
registered configuration struct like any other — see
[Application Configuration](/guides/architecture/configuration/):

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

Credentials are the exception. They stay out of the file, because `s3.New`
already reads the environment: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`,
and `AWS_SESSION_TOKEN`, plus `AWS_REGION` (or `AWS_DEFAULT_REGION`) and
`AWS_ENDPOINT_URL_S3` (or `AWS_ENDPOINT_URL`). A shell already configured for
the AWS CLI needs no options at all, and a deployment that injects credentials
as environment variables keeps them out of every config file you might commit.

That is also why the client below applies an option only when the setting is
non-empty: passing an empty string would override the environment with nothing.

## One client per process

`s3.Client` is safe for concurrent use and owns an `http.Client`, so a request
should find one rather than build one:

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

`s3.New` performs no network I/O — it validates credentials, region, and
endpoint — so a misconfiguration surfaces on the first request that touches
storage, not on the first byte transferred. Missing credentials are
`s3.ErrNoCredentials` and a missing region is `s3.ErrNoRegion`, both before any
call leaves the process.

| Option | Effect |
| --- | --- |
| `WithEndpoint` | endpoint URL, for S3-compatible servers |
| `WithRegion` | signing region |
| `WithCredentials` | static credentials |
| `WithCredentialsFromEnv` | read credentials from the environment |
| `WithPathStyle` | `endpoint/bucket/key` instead of `bucket.endpoint/key` |
| `WithUnsignedPayload` | sign headers only, so large streams are not buffered |
| `WithTimeout` | per-request timeout, default 60s |
| `WithHTTPClient` | supply the `http.Client` |

Addressing defaults to virtual-host style for `amazonaws.com` endpoints and
path style everywhere else, which is what S3-compatible servers expect.

## Storing an upload

A multipart field binds to `httpbind.File` like any other input — see
[Handlers](/guides/frontend/handlers/) — and its `Content` is already in memory, so
`bytes.NewReader` hands `Put` a body it can rewind:

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

Rewinding is not an aesthetic detail. SigV4 signs a hash of the payload, so
`Put` reads the body twice: a body that implements `io.Seeker` — a
`*bytes.Reader`, an `*os.File` — is hashed and rewound, and anything else is
buffered in memory first. `WithUnsignedPayload` streams instead, at the cost of
a signature that no longer covers the body; use it only over https, and pass
`s3.WithContentLength(n)` with it, because a body of unknown length goes out
chunked and AWS rejects a chunked `PutObject`.

Two limits apply before the object ever reaches the client:
`server.max_request_body` (10 MiB by default) and the multipart body limit
`httpbind.SetMaxMultipartBodyBytes` (1 MiB). Raise both when the endpoint
accepts real files.

`WithContentType` matters at download time rather than upload time — S3 stores
what you send and returns it as the object's `Content-Type` later. Here that
value came from the client, so an application serving those objects back to a
browser should decide the type itself rather than trust the part header.

## Serving one back

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
	pw.Logger(r.Context()).Error("download interrupted", pw.String("key", key), pw.Err(err))
}
```

`Get` returns as soon as the response headers arrive, so the body streams
through the handler instead of accumulating in it. `Head` fetches the same
metadata without the transfer, and `GetRange(ctx, bucket, key, offset, length)`
asks for a slice — a length of zero or less reads to the end:

```go
object, err := client.GetRange(r.Context(), bucket, key, 0, 1<<20)
```

The package signs requests but does not mint presigned URLs, so every byte a
browser downloads passes through the application. Public objects fronted by a
CDN are the way around that, and the redirect is yours to write.

## Listing

`List` returns one page. A truncated page carries `NextToken`, which
`WithContinuationToken` feeds back:

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

`WithDelimiter("/")` walks the listing as a tree instead: keys under the current
prefix arrive in `Objects`, and the prefixes below it in `CommonPrefixes`.
`WithStartAfter` resumes from a known key.

## Errors

S3 error codes map onto sentinels, so a handler branches with `errors.Is`
rather than on strings:

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

The default branch is deliberate. `ErrAccessDenied`, `ErrBadCredentials`, and a
refused connection are operator problems, not client mistakes, and
`pw.WriteProblem` turns any unrecognised error into a 500 that is logged in
full and reported as `internal error` — see [Responses](/guides/frontend/responses/).

| Sentinel | Raised by |
| --- | --- |
| `ErrNoSuchKey` | a missing object, and any 404 |
| `ErrNoSuchBucket` | a missing bucket |
| `ErrAccessDenied` | a denied request, and any 403 |
| `ErrBucketExists`, `ErrBucketNotEmpty` | `CreateBucket`, `DeleteBucket` |
| `ErrInvalidRange` | a range beyond the object |
| `ErrBadCredentials` | a rejected signature or key |
| `ErrNoCredentials`, `ErrNoRegion` | `s3.New`, before any request |
| `ErrTooManyRedirect` | a misconfigured endpoint |

`*s3.Error` carries the detail behind the sentinel — status, code, message, and
the request ID an object storage provider asks for:

```go
var storageErr *s3.Error
if errors.As(err, &storageErr) {
	pw.Logger(ctx).Error("s3 failed",
		pw.String("op", storageErr.Op), pw.String("code", storageErr.Code),
		pw.Int("status", storageErr.StatusCode), pw.String("request_id", storageErr.RequestID))
}
```

`Delete` succeeds on a key that does not exist, which is how S3 itself behaves.

## Local development

Any S3-compatible server works, and the endpoint setting is the only difference
from production. [RustFS](https://rustfs.com/) starts in one command:

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

`client.CreateBucket(ctx, bucket)` creates the bucket, and `ErrBucketExists`
means a previous run already did — which makes a first-use bootstrap safe to run
every time.

Tests that should not need a container point the client at an `httptest.Server`
instead: it is an endpoint like any other, and path-style addressing is already
the default for a non-AWS host.

```go
client, err := s3.New(
	s3.WithEndpoint(server.URL),
	s3.WithRegion("us-east-1"),
	s3.WithCredentials(s3.Credentials{AccessKeyID: "id", SecretAccessKey: "secret"}),
)
```

## On TinyGo

Signing, request building, and XML decoding are shared code. The builds differ
only in how a request reaches the network:

| Build | HTTP stack (`s3.Backend`) |
| --- | --- |
| Standard Go | `net/http` with `crypto/tls` |
| TinyGo, or `-tags force_tinygo_logic` | `tinygodriver/https`, TLS through the host OS |

The second row is what makes the package work at all under TinyGo, whose
`crypto/tls` is a stub: TLS is performed by the OS — Network.framework on macOS,
Schannel on Windows, vendored mbedTLS on Linux — with no library to install and
no certificate bundle to ship. Other TinyGo targets report
`https.ErrPlatformNotSupported`. The `force_tinygo_logic` tag selects that same
path under host Go, which is how you exercise it without a TinyGo toolchain.

Neither build lets `http.Client` follow redirects, because a redirected request
goes to another host and its signature covers the host it was signed for. The
client follows them itself and signs each hop, so a bucket in another region
behaves identically on both targets.

## What the package does not do

| Constraint | Consequence |
| --- | --- |
| No multipart upload | `Put` sends one request, so the endpoint's single-request limit applies — 5 GiB on AWS |
| No presigned URLs | downloads pass through the application, or through a CDN in front of a public bucket |
| Credentials are static or environment values | no shared credentials file, no SSO, no IMDS |
| No connection reuse on TinyGo | the `https` transport opens a connection per request, so every call pays a TLS handshake |
| Whole-object operations only | no versioning, ACL, tagging, or lifecycle APIs |

The gap that most often decides the design is the first one. An application
that accepts arbitrarily large files wants a direct browser-to-storage upload,
which needs presigned URLs and therefore a different client on a
standard-Go-only path — or an upload limit small enough that one request is
honest about it.
