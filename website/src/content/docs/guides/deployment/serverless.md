---
title: Serverless Hosting
description: Which scale-to-zero and function runtimes a Popcorn Wave application can use, and where an HTTP adapter is required.
sidebar:
  order: 3
---

“Serverless” describes several incompatible startup models. The useful question
is whether the host starts an HTTP process, asks for an exported handler, or
delivers a provider-specific event. Popcorn Wave supports the first model and
HTTP adapters to it without changing application code.

| Host shape | Examples | Status |
| --- | --- | --- |
| HTTP container with an assigned `PORT` | Cloud Run services, AWS App Runner, Azure Container Apps | supported by the normal Dockerfile |
| Invocation-to-HTTP adapter | AWS Lambda Web Adapter | supported; add the adapter to the deployment |
| HTTP-forwarding custom handler | Azure Functions | supported for HTTP-only functions |
| Exported Go handler, remotely built | Vercel Go, Cloud Run functions | supported by generated source staging |
| Provider event function | DigitalOcean Functions and non-HTTP triggers | deferred |
| Fetch-event Wasm | Cloudflare Workers | targeted, currently blocked on adapter build compatibility |
| Component-model Wasm | Fastly Compute and WASI HTTP hosts | deferred with WASI HTTP support |

Container services are not a separate runtime. They start the scaffolded image
and set `PORT`; [`pw.Run`](/reference/runtime/) already binds it. This includes
platforms that scale the container to zero.

Builds have two independent axes. `--target` selects the deployment host and
`--backend` selects `nethttp` or `fasthttp`; `pw dev` remains unchanged.

```shell
pw build --target=lambda --backend=nethttp
pw build --target=azure-functions --backend=fasthttp
pw build --target=google-cloud-run-functions --backend=nethttp
pw build --target=vercel-go --backend=fasthttp
```

Every result is written under `.pw/build/<target>/<backend>/` with a
`deployment.json` manifest. `config.prod.toml` is required. A fasthttp build
also requires `project.fasthttp = true`.

## AWS Lambda

Use the [AWS Lambda Web Adapter](https://github.com/aws/aws-lambda-web-adapter)
instead of changing `main` into a Lambda event handler. For an image deployment,
copy the adapter into the Lambda extensions directory in the runtime stage:

```dockerfile
COPY --from=public.ecr.aws/awsguru/aws-lambda-adapter:1.0.1 \
  /lambda-adapter /opt/extensions/lambda-adapter
```

The application keeps its normal entry point. The adapter forwards requests to
`AWS_LWA_PORT`, then `PORT`, then `8080`; Popcorn Wave follows the same order for
its listener. The generated directory contains a Linux `bootstrap`,
`config.prod.toml`, and a Dockerfile pinned to the adapter version. It sets
`APP_ENV=prod` and is the Docker build context.

This intentionally does not embed a Lambda Runtime API client in the framework.
The adapter supports Function URLs, API Gateway, ALB, buffered responses, and
response streaming while leaving one portable image usable outside Lambda.

## Azure Functions

Run the binary as a custom handler and enable HTTP request forwarding. The host
publishes the assigned listener as `FUNCTIONS_CUSTOMHANDLER_PORT`; `pw.Run`
recognizes it automatically.

```json
{
  "version": "2.0",
  "customHandler": {
    "description": { "defaultExecutablePath": "run.sh" },
    "enableProxyingHttpRequest": true
  }
}
```

The generated directory contains the Linux handler, `run.sh`, `host.json`, and
the catch-all `http/function.json`. Upload that directory with Azure Functions
Core Tools or your infrastructure workflow. Queue triggers and
extra input/output bindings use Azure's custom payload, not ordinary HTTP, and
are outside this adapter-free path. Azure also cautions that Functions is not a
general reverse proxy; for a full web application, Container Apps or App Service
usually has fewer routing and cold-start constraints.

## Vercel Go and Cloud Run functions

Vercel's Go runtime requires a `.go` file under `api/` exporting an
`http.HandlerFunc`. Cloud Run functions requires registration with the Go
Functions Framework. Both remote-build source rather than starting the
application's configured `main`, so a port alias cannot support them.

`pw build` copies the application module into an isolated source tree and
transforms the selected `main` into an initialization function. Vercel receives
`api/Handler`; Cloud Run functions receives the `PopcornWave` Functions
Framework registration. Initialization is guarded once per warm instance.

For `nethttp`, the generated handler uses `pw.Middlewares`. For `fasthttp`, it
uses `pwfast.Start` and the framework's in-memory HTTP/1 bridge so the provider
still receives the required `http.HandlerFunc`. The staged source is formatted,
its module is tidied and vendored, and its provider package is compiled from
that vendor tree before the build is reported ready. Deploy the generated
directory, not the application checkout.

## Cloudflare Workers

Cloudflare support remains a target. The intended bridge is a fetch-event
adapter that invokes the same `net/http` handler returned by `pw.Middlewares`;
it does not use `pw.Run` or open a listener.

The current candidate is [`github.com/syumai/workers`](https://github.com/syumai/workers),
which is explicitly experimental and currently does not build with the Popcorn
Wave application dependency graph. We therefore do not generate a Wrangler
project or claim runtime support yet. This is a tracked compatibility blocker,
not a decision to drop Cloudflare.

The unblock test is deliberately small: build one handler with both the
upstream standard-Go and TinyGo templates, run it under `wrangler dev`, then
verify request bodies, duplicate headers, cookies, redirects, and streaming.
Only after one compiler path passes that test will `pw` generate the Wasm,
JavaScript loader, and Wrangler configuration. Cloudflare's JavaScript-hosted
Wasm path is separate from the component-model WASI HTTP work deferred for
other edge hosts.

## Runtime limits still apply

Function hosts may buffer responses, cap duration, freeze an idle instance, and
provide only ephemeral local storage. Configure `html.streaming = false` where
the ingress buffers, bound live responses below the provider duration, and use a
shared session or rate-limit backend whenever requests may land on different
instances.
