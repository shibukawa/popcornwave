---
title: サーバーレスホスティング
description: Popcorn Wave が対応する scale-to-zero / Functions ランタイムと、HTTP アダプターが必要になる境界。
sidebar:
  order: 3
---

「サーバーレス」には互換性のない複数の起動方式があります。区別すべきなのは、ホストが
HTTP プロセスを起動するのか、export されたハンドラーを要求するのか、プロバイダー固有の
イベントを渡すのかです。Popcorn Wave は最初の方式と、そこへ HTTP 変換できる方式に、
アプリケーションコードを変えずに対応します。

| ホストの形 | 例 | 対応状況 |
| --- | --- | --- |
| `PORT` が割り当てられる HTTP コンテナ | Cloud Run services、AWS App Runner、Azure Container Apps | 通常の Dockerfile で対応済み |
| invocation を HTTP に変換するアダプター | AWS Lambda Web Adapter | 対応済み。デプロイ物にアダプターを追加 |
| HTTP forwarding custom handler | Azure Functions | HTTP-only function に対応済み |
| リモートビルドされる export 済み Go handler | Vercel Go、Cloud Run functions | source staging 生成で対応済み |
| プロバイダー固有イベント | DigitalOcean Functions、非 HTTP trigger | 保留 |
| Fetch-event Wasm | Cloudflare Workers | 対応対象。現在は adapter の build 互換性で停止中 |
| Component-model Wasm | Fastly Compute などの WASI HTTP host | WASI HTTP 対応まで保留 |

コンテナサービスは別ランタイムではありません。生成済みイメージを起動して `PORT` を
設定するだけで、[`pw.Run`](/ja/reference/runtime/) がそのポートを listen します。
コンテナをゼロまで scale down するサービスも同じです。

build には独立した二つの軸があります。`--target` はデプロイ先、`--backend` は
`nethttp` または `fasthttp` を選択します。`pw dev` の動作は変わりません。

```shell
pw build --target=lambda --backend=nethttp
pw build --target=azure-functions --backend=fasthttp
pw build --target=google-cloud-run-functions --backend=nethttp
pw build --target=vercel-go --backend=fasthttp
```

成果物は `.pw/build/<target>/<backend>/` に生成され、`deployment.json` が付属します。
`config.prod.toml` は必須です。fasthttp build には `project.fasthttp = true` も必要です。

## AWS Lambda

`main` を Lambda イベントハンドラーへ変えるのではなく、
[AWS Lambda Web Adapter](https://github.com/aws/aws-lambda-web-adapter) を使います。
コンテナデプロイなら、runtime stage でアダプターを extensions ディレクトリへ追加します。

```dockerfile
COPY --from=public.ecr.aws/awsguru/aws-lambda-adapter:1.0.1 \
  /lambda-adapter /opt/extensions/lambda-adapter
```

アプリケーションの entry point はそのままです。アダプターは `AWS_LWA_PORT`、`PORT`、
`8080` の順で転送先を決め、Popcorn Wave の listener も同じ割り当てに従います。生成先には
Linux `bootstrap`、`config.prod.toml`、adapter version を固定した Dockerfile が入り、
Dockerfile が `APP_ENV=prod` を設定します。

フレームワーク自身が Lambda Runtime API client を持たないのは意図的です。Web Adapter は
Function URLs、API Gateway、ALB、buffered response、response streaming を扱いながら、
Lambda 外でも動く一つのイメージを維持できます。

## Azure Functions

バイナリを custom handler として起動し、HTTP request forwarding を有効にします。
ホストが割り当てた listener は `FUNCTIONS_CUSTOMHANDLER_PORT` に入り、`pw.Run` が自動で
認識します。

```json
{
  "version": "2.0",
  "customHandler": {
    "description": { "defaultExecutablePath": "run.sh" },
    "enableProxyingHttpRequest": true
  }
}
```

生成先には Linux handler、`run.sh`、`host.json`、catch-all の `http/function.json` が
入ります。このディレクトリを Functions Core Tools または infrastructure workflow で
upload します。Queue trigger や追加の
input/output binding は通常の HTTP ではなく Azure 独自 payload を使うため、この経路の
対象外です。また Azure Functions は汎用 reverse proxy ではありません。Web アプリ全体なら、
Container Apps または App Service のほうが routing と cold start の制約が少なくなります。

## Vercel Go と Cloud Run functions

Vercel の Go runtime は `api/` 以下に `http.HandlerFunc` を export した `.go` ファイルを
要求します。Cloud Run functions は Go Functions Framework への登録を要求します。どちらも
設定済みの `main` を起動せずソースをリモートビルドするため、ポート名の読み替えだけでは
対応できません。

`pw build` は application module を隔離した source tree へコピーし、選択 backend の `main` を
初期化関数へ変換します。Vercel には `api/Handler`、Cloud Run functions には Functions
Framework の `PopcornWave` 登録を生成し、warm instance ごとに一度だけ初期化します。

`nethttp` は `pw.Middlewares`、`fasthttp` は `pwfast.Start` と framework の in-memory HTTP/1
bridge を使うため、どちらも provider が要求する `http.HandlerFunc` を公開できます。生成 source は
format、module tidy、vendor 作成、vendor tree からの provider package compile まで成功してから
ready と報告されます。
application checkout ではなく生成ディレクトリを deploy してください。

## Cloudflare Workers

Cloudflare 対応は対象から外しません。想定する境界は fetch event を
`pw.Middlewares` が返す同じ `net/http` handler へ渡す adapter です。`pw.Run` は使わず、
listener も開きません。

現在の候補は [`github.com/syumai/workers`](https://github.com/syumai/workers) ですが、
upstream 自身が experimental としており、現状の Popcorn Wave の dependency graph では
build が通っていません。そのため、今は Wrangler project を生成せず、runtime 対応済みとも
表示しません。これは Cloudflare を諦める判断ではなく、追跡する compatibility blocker です。

解除テストは小さく固定します。一つの handler を upstream の標準 Go template と TinyGo
template の両方で build し、`wrangler dev` 上で request body、重複 header、cookie、redirect、
streaming を検証します。少なくとも一方の compiler path が通ってから、Wasm、JavaScript loader、
Wrangler configuration の生成を `pw` に追加します。この JavaScript-hosted Wasm 経路は、他の
edge host 向けに保留している component-model WASI HTTP とは別物です。

## ランタイム制限は残る

Functions ホストは response を buffer し、実行時間を制限し、idle instance を freeze し、
ローカルには一時ストレージしか提供しないことがあります。Ingress が buffer する場合は
`html.streaming = false`、live response はプロバイダーの上限未満に設定してください。
別 instance に到達し得る request の session と rate limit には共有 backend が必要です。
