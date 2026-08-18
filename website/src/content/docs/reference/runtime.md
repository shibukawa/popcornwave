---
title: Runtime API
description: Everything the pw package exposes at runtime, grouped by the job it does, with each type listed above the calls that produce it and the methods it carries.
sidebar:
  order: 1
---

Two packages carry the runtime, and only one of them belongs in application
code. `pw` is the stable application-facing API, and every symbol on this page
lives there. `pwruntime` holds the narrow contract that generated files compile
against; a handler reaching for it has almost always found something `pw`
already re-exports under a shorter name.

Sections group symbols by the job they do. Inside a section the order is godoc's:
a type comes first, then the calls that hand you one, then its methods, and the
free functions that belong to no type come last.

A second split runs through the list below. Not all of this is written by hand:
`pw generate` emits the calls that register a configuration struct, a document
shell, and a page tree. Those entries are marked **generated**. They appear here
because you will read them in your own repository even though you never type
them.

## Starting and stopping

**`ServeMux`** — `net/http`'s `ServeMux` on ordinary Go builds. A type alias,
not a wrapper.

| | |
| --- | --- |
| `NewServeMux() *ServeMux` | Creates the router |

**`Option`** — a setting `Run` and `Middlewares` both accept.

| | |
| --- | --- |
| `WithPublicFS(fs.FS) Option` | Supplies the embedded public tree, rooted at its `public` directory |

**Functions**

| Function | What it does |
| --- | --- |
| `Run(ctx, handler, ...Option) error` | Parses configuration, initializes the framework, serves, shuts down gracefully, and releases resources |
| `Middlewares(handler, ...Option) (http.Handler, error)` | Same initialization, returning the wrapped stack instead of serving it |
| `ParseConfig() error` | Parses every configuration source without serving anything |
| `SetConfigLoadOptions(configbind.LoadOptions)` | Adjusts where and how configuration loads, before `ParseConfig` |

`Run` is the whole lifecycle in one call, which is why the generated `main`
never mentions the pieces. Reach for `Middlewares` when the application owns its
own listener — a serverless adapter, a test harness, an existing `http.Server`.
The startup summary is emitted from `Middlewares` rather than from `Run`,
because at that point the framework still knows the resolved configuration but
has not yet learned the address anyone will bind.

`ParseConfig` and `SetConfigLoadOptions` matter to a binary that needs
configuration before it decides to serve at all: a CLI subcommand, a migration
runner, a one-shot job.

## Configuration and environment

**Functions**

| Function | What it does |
| --- | --- |
| `RegisterConfig[T](prefix)` | Registers one configuration struct under a TOML prefix (**generated**) |
| `Config[T](r) T` | Returns the parsed struct for this request |
| `ConfigContext[T](ctx) T` | The same below the handler; `nil` is an acceptable context outside a request |
| `Env() string` | The resolved environment token |
| `RegisterSubCommand[T](name, help)` | Registers typed CLI-only input |
| `Command[T]() (T, bool)` | The selected and parsed subcommand, after `ParseConfig` |
| `ScaffoldTOML()`, `ScaffoldEnv()` | Renders a configuration scaffold for every registered prefix |
| `WriteScaffoldTOML(w)`, `WriteScaffoldEnv(w)` | The same scaffolds, written to a writer |

**Constants**

| Constant | What it is |
| --- | --- |
| `EnvVar`, `DefaultEnv` | `APP_ENV`, and the token used when it is unset |
| `EnvDevelopment`, `EnvStaging`, `EnvProduction` | The well-known tokens; any other lowercase token is also valid |

`Config` never fails and never returns an error. A prefix that was registered
but never parsed yields its declared defaults, and an unregistered type yields
the zero value — a handler reading configuration is on the response path, where
a nil check would only postpone the same missing value to a later line.

`SubCommand` remains as a deprecated alias of `RegisterSubCommand`.

Every framework configuration struct is exported too — `ServerConfig`,
`MiddlewareConfig`, `SecurityConfig`, `SessionConfig`, `ObservabilityConfig`,
`HTMLConfig`, `RDBConfig`, and the nested types under them. Their fields and
defaults are listed in [Application Configuration Keys](/reference/configuration/); the
narrative version is
[Application Configuration](/guides/architecture/configuration/).

## Reading a request

**`Authentication`** — the verified result of the authentication middleware,
zero-valued when there is none.

| | |
| --- | --- |
| `RequestAuthentication(r) Authentication` | The verified authentication result |
| `Authenticated(r) bool` | Whether the request carries a verified identity |
| `RequestAuthenticationContext(ctx)`, `AuthenticatedContext(ctx)` | The same below the handler |

**Functions**

| Function | What it does |
| --- | --- |
| `Parse[T](r) (T, error)` | Binds path, query, body, header, cookie, and method into one struct |
| `IsBot(r) bool` | Whether the client will run the boundary runtime |
| `Context(r) context.Context` | The request's context, for the layers below the handler |

`Context` is the supported crossing between the two currencies: a handler holds
the request, and generated SQL, a service function, and anything else callable
without a request take a `context.Context`. `r.Context()` returns the same value
and stays legal — it is a method on the `net/http` request type, which is the
one read a second transport cannot follow, so `pw.Context` is what a fasthttp
build rewrites cleanly.

Authorization must consume `RequestAuthentication`, never the presence of a
cookie. A request that passed through no authentication middleware, and an
anonymous request, both report the same explicitly unauthenticated zero value,
so the check has one shape rather than two.

`IsBot` reads the `User-Agent` header and verifies nothing, which is acceptable
only because of what it decides: which render branch runs. Both branches render
one chain with one set of data, so a forged header buys a slower first byte and
nothing else. Keep it out of access decisions, and out of anything that changes
what a page *says* — varying delivery by client is not cloaking, varying content
is.

## Writing a response

Nothing here asks the handler whether the response should stream. A chain that
can open an await boundary streams; one that cannot is buffered and committed
whole. That decision belongs to the templates that were composed, not to the
handler that composed them, which is why adopting progressive rendering changes
a handler's parameters and not its `Write` call.

### HTML

**`HTMLFragment`** — a generated template with its parameters bound. Generated
code produces one; these calls consume it.

| | |
| --- | --- |
| `WriteHTML(w, r, leaf)` | Renders one fragment inside the registered document shell |
| `WriteHTMLFragment(w, r, fragment)` | Renders one template as the entire response — no shell, no merged head |

**`HTMLWrapper`** — a generated template wrapper: a layout, or the document
shell.

| | |
| --- | --- |
| `WriteHTMLPage(w, r, wrappers, leaf, ...HTMLOption)` | Renders a page inside its layouts and the document shell (**generated**) |
| `WriteHTMLChain(w, r, wrappers, leaf, ...HTMLOption)` | Renders an explicit wrapper chain |
| `RegisterHTMLDocument(wrapper)` | Installs the application document shell (**generated**) |

**`HTMLOption`** — tunes one render, extending the options `HTMLConfig` already
supplies.

**Functions**

| Function | What it does |
| --- | --- |
| `RegisterHTMLErrorPage(resolve)` | Installs the error page resolver; without one, a minimal built-in page is used |

`WriteHTMLFragment` is the deliberate exception to the streaming rule above, and
it always buffers. It answers an htmx-style swap into a document that already
exists, where the browser parser never sees the response arrive — the swap
library holds the body and inserts it, so no marker the framework wrote could
connect a settled boundary back to its placeholder. A fragment carrying head
contributions is an error rather than a silent drop: there is no head here to
receive it.

### Typed and problem responses

| Function | What it does |
| --- | --- |
| `WriteAPI[T](w, r, value)` | Writes a typed response in the negotiated format |
| `WriteProblem(w, r, err)` | Maps an error to an RFC problem response |
| `LifecycleHeaders(Lifecycle) (Middleware, error)` | RFC 9745 Deprecation and RFC 8594 Sunset middleware |

The problem values `WriteProblem` accepts are [below](#errors).

### Streams

**`Stream[T]`** — the open response a callback writes into.

| | |
| --- | --- |
| `WriteStream[T](w, r, fn)` | Opens one, negotiating SSE, NDJSON, or a JSON array from `Accept`, and runs `fn` against it |
| `Stream.Write(value) error` | Writes and flushes one value; the runtime closes the stream when `fn` returns |

**Functions**

| Function | What it does |
| --- | --- |
| `SetStreamErrorHandler(fn)` | Receives a stream or socket failure raised after the status was sent |

### WebSockets

**`Socket[In, Out]`** — the upgraded connection a callback runs against.

| | |
| --- | --- |
| `WebSocket[In, Out](w, r, fn) error` | Upgrades the request and runs `fn` against one; returns the handshake error alone |
| `WebSocketWith[In, Out](w, r, opts, fn) error` | The same with per-call `SocketOptions` |
| `Socket.Read() (In, error)` | Reads one message, decoded into `In` (**generated**); call from one goroutine |
| `Socket.Write(Out) error` | Writes one message, encoded from `Out` (**generated**); safe from any goroutine |
| `Socket.Close() error` | Ends the socket with a close handshake; the runtime also does this when `fn` returns |
| `Socket.Subprotocol() string` | The subprotocol the handshake negotiated, or `""` |

**`SocketOptions`** — limits, deadlines, and the origin policy one socket runs
under.

| | |
| --- | --- |
| `SocketDefaults() SocketOptions` | The effective defaults, with every unset field resolved |
| `SetSocketDefaults(SocketOptions)` | Installs the process-wide limits, deadlines, and origin policy |

### Asset URLs

| Function | What it does |
| --- | --- |
| `RuntimeScriptURL() string` | The absolute path of the boundary runtime module |
| `PublicAssetURL(name) string` | The URL this build serves one static asset under, revision segment included |

## Errors

**`Problem`** — the application-facing problem value: `Status`, `Title`, `Code`,
`Message`, `Fields`, `Cause`, `RateLimit`.

| | |
| --- | --- |
| `BadRequest`, `Unauthorized`, `Forbidden`, `NotFound`, `Conflict`, `PayloadTooLarge`, `TooManyRequests`, `InternalServerError`, `ServiceUnavailable` | Constructors for the common statuses |
| `RateLimited(RateLimit, ...any) Problem` | A 429 with `Retry-After` and `X-RateLimit-*` metadata |
| `Validation(...FieldError) Problem` | A 400 carrying every detected field failure |

**`FieldError`** — one field-level failure inside a `Problem`.

| | |
| --- | --- |
| `Field(field, location, message) FieldError` | One field-level failure |

**Types**

| Type | What it is |
| --- | --- |
| `RateLimit` | The retry metadata a 429 carries |
| `HTMLErrorPage` | `func(Problem) HTMLFragment` — the shape `RegisterHTMLErrorPage` accepts |
| `PublicError` | Implemented by an error that supplies its own safe projection |
| `AsyncError` | The presentation-safe failure a `recover` clause renders |
| `UnsetPendingError` | A required async value the handler never set |

The boundary between what the server knows and what a page shows is drawn by
type, not by discipline. `HTMLErrorPage` receives the mapped `Problem` rather
than the original error, so a template cannot render a cause the server meant to
keep. An error that is not a `PublicError` reaches a recover clause as an
internal code with no message; the original stays server-side and reaches the
logger.

`UnsetPendingError` is raised before any byte commits, so it becomes an ordinary
problem response rather than a torn page.

## Progressive rendering

**`Pending[T]`** — a value the handler started before rendering and a template
awaits.

| | |
| --- | --- |
| `Go[T](ctx, work) Pending[T]` | Starts work in its own goroutine and returns the handle |
| `Resolved[T](value) Pending[T]` | A handle already settled to a value |
| `Failed[T](err) Pending[T]` | A handle already settled to an error |

A template parameter declared `async T` becomes a `Pending[T]` field in the
generated `Params` struct. The context passed to `Go` bounds the work and stays
the caller's to cancel — a render bounds only how long it waits. A panic inside
the work becomes the handle's error and surfaces through the boundary's recover
clause instead of taking the process down.

`Resolved` is what a test passes instead of starting a goroutine. See
[Async rendering](/guides/cross-layer/async-rendering/).

## Database

| Function | What it does |
| --- | --- |
| `DB(r) (*sql.DB, bool)` | The pool of the effective connection group |
| `DBDriver(r) (string, bool)` | The driver scheme of that pool |
| `SelectDB(r, group) context.Context` | Pins a named connection group |
| `SelectWriteDB(r) (context.Context, error)` | Pins the group framework-owned writes use |
| `SelectSessionDB(r) (context.Context, error)` | Pins the group holding the session table |
| `Transaction(r, fn) error` | Runs `fn` in a transaction whose context the generated SQL uses |
| `DBContext`, `DBDriverContext`, `SelectDBContext`, `SelectWriteDBContext`, `SelectSessionDBContext`, `TransactionContext` | Each of the above below the handler, taking a `context.Context` — including one `SelectDB` already pinned |

A nested `Transaction` opens a savepoint rather than a second transaction, so
its failure rolls back only its own work and leaves the outer one usable.
A transaction runs on the effective group of its context, so `SelectDB` names
the group for a whole transaction exactly as it does for one statement — and
unpinned SQL inside it stays there instead of falling back to the default group.

`SelectDB` reports an unknown group name at the first statement that uses the
returned context, not at the call itself. `SelectWriteDB` can never select a
replica, which is what lets a caller that must write stay ignorant of the
deployment topology. See [Relational databases](/guides/storage/rdb/).

## Logging

**`Log`** — the context-bound logger.

| | |
| --- | --- |
| `Logger(r) Log` | The logger bound to the request, its stable attributes, and the active span |
| `LoggerContext(ctx) Log` | The same below the handler, and inside a child span |

**`Attribute`** — one scalar key-value pair, the same type a span attribute uses.

| | |
| --- | --- |
| `String`, `Int`, `Int64`, `Float64`, `Bool`, `Duration`, `Err` | Attribute constructors |
| `WithLogAttributes(ctx, ...Attribute) context.Context` | Adds attributes to every record taken from the returned context |

**Constants**

| Constant | What it is |
| --- | --- |
| `Level`, `LevelTrace`…`LevelOff` | Severities; trace sits one step below debug, which `slog` does not name |

`Logger` never returns something that cannot be called, so no handler needs a
nil check. Acquire it again inside a child span to correlate records with that
span:

```go
ctx, span := pw.StartSpan(r, "load-user")
defer span.End()
pw.LoggerContext(ctx).Info("loaded", pw.Int("rows", n))
```

There is no `Fatal` and no `Panic`. Logging reports what happened; it does not
decide whether the process keeps running.

Attribute constructors cover scalars only. A record must never fail to encode,
and a value that needs a structure belongs in attributes of its own.

## Tracing

**`Span`** — the started span. `End` closes it.

| | |
| --- | --- |
| `StartSpan(r, name, ...Attribute) (context.Context, *Span)` | Opens a child of the request span |
| `StartSpanKind(r, name, kind, ...Attribute)` | The same, for work that is not internal |
| `StartSpanContext(ctx, …)`, `StartSpanKindContext(ctx, …)` | The same below the handler, nesting under the span the context carries |

**Constants**

| Constant | What it is |
| --- | --- |
| `SpanKind`, `SpanKindInternal`…`SpanKindConsumer` | What kind of work a span describes |
| `StatusUnset`, `StatusOK`, `StatusError` | Span status codes |

**Functions**

| Function | What it does |
| --- | --- |
| `TraceID(r) string`, `SpanID(r) string` | The current identifiers, or empty outside a trace |
| `Traced(r) bool` | Whether the request carries a valid span context |
| `TraceIDContext(ctx)`, `SpanIDContext(ctx)`, `TracedContext(ctx)` | The same below the handler |

Outside a trace, or with tracing disabled, `StartSpan` returns a span that
records nothing and costs nothing to end — the `defer span.End()` needs no
guard. The framework creates the request root span, so a handler starts only the
spans describing its own work.

`TraceID` is the value to show a user on an error page, since it is what
correlates their report with the records the server kept.

## OpenAPI and API documentation

**`OpenAPIInfo`** — the document's title, version, and description.

| | |
| --- | --- |
| `SetOpenAPIInfo(OpenAPIInfo) error` | Sets them on the document |

**Functions**

| Function | What it does |
| --- | --- |
| `AssembleOpenAPI() ([]byte, error)` | Builds the document from every registered operation |
| `OpenAPIJSON(w, r)` | Serves that document as a handler |
| `ScalarUI(specURL) http.Handler` | A Scalar reference page over the document at `specURL` |
| `SwaggerUI(specURL) http.Handler` | A Swagger UI page over the same |

Both UIs load their assets from a public CDN; nothing is embedded in the binary.
Mounting one by hand is the exception rather than the rule — `server.openapi`
and `server.api_doc` serve both without a route. See
[API Documentation](/productivity/api-documentation/).

## Extensions

**`Extension`** — `Name`, `Slot`, `Setup`, and `Close`.

| | |
| --- | --- |
| `RegisterExtension(Extension)` | Adds one capability to the framework chain |

**`Middleware`** — `func(http.Handler) http.Handler`.

| | |
| --- | --- |
| `RegisterMiddleware(slot, name, Middleware)` | Adds one application middleware at that slot; call it from `main`, before the chain is built |

**Types**

| Type | What it is |
| --- | --- |
| `Slot`, `SlotSession`, `SlotAuthentication`, `SlotGuard` | Position in the request chain; smaller runs earlier |

An imported package calls `RegisterExtension` from an `init` function, so only
linked capabilities contribute configuration and code. `Setup` runs once during
framework initialization — after configuration parsing and database startup —
and returns the middleware to install. Returning a nil middleware installs
nothing, which is how a disabled extension opts out without a branch anywhere
else.

The slot ordering exists so a guard always observes state that was established
before it: the session at 10, authentication at 20, the guard at 30.
