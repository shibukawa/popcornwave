---
title: Runtime API
description: Everything the pw package exposes at runtime, grouped by the job it does.
sidebar:
  order: 1
---

Two packages carry the runtime, and only one of them belongs in application
code. `pw` is the stable application-facing API, and every symbol on this page
lives there. `pwruntime` holds the narrow contract that generated files compile
against; a handler reaching for it has almost always found something `pw`
already re-exports under a shorter name.

A second split runs through the list below. Not all of this is written by hand:
`pw generate` emits the calls that register a configuration struct, a document
shell, and a page tree. Those entries are marked **generated**. They appear here
because you will read them in your own repository even though you never type
them.

## Starting and stopping

| Symbol | What it does |
| --- | --- |
| `Run(ctx, handler, ...Option) error` | Parses configuration, initializes the framework, serves, shuts down gracefully, and releases resources |
| `Middlewares(handler, ...Option) (http.Handler, error)` | Same initialization, returning the wrapped stack instead of serving it |
| `WithPublicFS(fs.FS) Option` | Supplies the embedded public tree, rooted at its `public` directory |
| `NewServeMux() *ServeMux` | Creates the router |
| `ServeMux` | `net/http`'s `ServeMux` on ordinary Go builds — a type alias, not a wrapper |
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

| Symbol | What it does |
| --- | --- |
| `RegisterConfig[T](prefix)` | Registers one configuration struct under a TOML prefix (**generated**) |
| `Config[T](ctx) T` | Returns the parsed struct; `nil` is an acceptable context outside a request |
| `Env() string` | The resolved environment token |
| `EnvVar`, `DefaultEnv` | `APP_ENV`, and the token used when it is unset |
| `EnvDevelopment`, `EnvStaging`, `EnvProduction` | The well-known tokens; any other lowercase token is also valid |
| `RegisterSubCommand[T](name, help)` | Registers typed CLI-only input |
| `Command[T]() (T, bool)` | The selected and parsed subcommand, after `ParseConfig` |
| `ScaffoldTOML()`, `ScaffoldEnv()` | Renders a configuration scaffold for every registered prefix |
| `WriteScaffoldTOML(w)`, `WriteScaffoldEnv(w)` | The same scaffolds, written to a writer |

`Config` never fails and never returns an error. A prefix that was registered
but never parsed yields its declared defaults, and an unregistered type yields
the zero value — a handler reading configuration is on the response path, where
a nil check would only postpone the same missing value to a later line.

`SubCommand` remains as a deprecated alias of `RegisterSubCommand`.

Every framework configuration struct is exported too — `ServerConfig`,
`MiddlewareConfig`, `SecurityConfig`, `SessionConfig`, `ObservabilityConfig`,
`HTMLConfig`, `RDBConfig`, and the nested types under them. Their fields and
defaults are listed in [Application Configuration](/reference/configuration/); the
narrative version is [Configuration](/guides/architecture/configuration/).

## Reading a request

| Symbol | What it does |
| --- | --- |
| `Parse[T](r) (T, error)` | Binds path, query, body, header, cookie, and method into one struct |
| `RequestAuthentication(ctx) Authentication` | The verified authentication result |
| `Authenticated(ctx) bool` | Whether the request carries a verified identity |
| `IsBot(r) bool` | Whether the client will run the boundary runtime |

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

| Symbol | What it does |
| --- | --- |
| `WriteHTML(w, r, leaf)` | Renders one generated fragment inside the registered document shell |
| `WriteHTMLPage(w, r, wrappers, leaf, ...HTMLOption)` | Renders a page inside its layouts and the document shell (**generated**) |
| `WriteHTMLChain(w, r, wrappers, leaf, ...HTMLOption)` | Renders an explicit wrapper chain |
| `WriteHTMLFragment(w, r, fragment)` | Renders one template as the entire response — no shell, no merged head |
| `WriteAPI[T](w, r, value)` | Writes a typed response in the negotiated format |
| `WriteProblem(w, r, err)` | Maps an error to an RFC problem response |
| `NewStream[T](w, r) *Stream[T]` | Opens a streamed response, negotiating SSE, NDJSON, or a JSON array from `Accept` |
| `Stream.Send(value)`, `Stream.Close()` | Writes one value; finalizes the response |
| `RegisterHTMLDocument(wrapper)` | Installs the application document shell (**generated**) |
| `RegisterHTMLErrorPage(resolve)` | Installs the error page resolver; without one, a minimal built-in page is used |
| `RuntimeScriptURL() string` | The absolute path of the boundary runtime module |

Nothing here asks the handler whether the response should stream. A chain that
can open an await boundary streams; one that cannot is buffered and committed
whole. That decision belongs to the templates that were composed, not to the
handler that composed them, which is why adopting progressive rendering changes
a handler's parameters and not its `Write` call.

`WriteHTMLFragment` is the deliberate exception, and it always buffers. It
answers an htmx-style swap into a document that already exists, where the
browser parser never sees the response arrive — the swap library holds the body
and inserts it, so no marker the framework wrote could connect a settled
boundary back to its placeholder. A fragment carrying head contributions is an
error rather than a silent drop: there is no head here to receive it.

## Errors

| Symbol | What it does |
| --- | --- |
| `Problem` | The application-facing problem value: `Status`, `Title`, `Code`, `Message`, `Fields`, `Cause` |
| `BadRequest`, `Unauthorized`, `Forbidden`, `NotFound`, `Conflict`, `PayloadTooLarge`, `InternalServerError`, `ServiceUnavailable` | Constructors for the common statuses |
| `Validation(...FieldError) Problem` | A 400 carrying every detected field failure |
| `Field(field, location, message) FieldError` | One field-level failure |
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

| Symbol | What it does |
| --- | --- |
| `Pending[T]` | A value the handler started before rendering and a template awaits |
| `Go[T](ctx, work) Pending[T]` | Starts work in its own goroutine and returns the handle |
| `Resolved[T](value) Pending[T]` | A handle already settled to a value |
| `Failed[T](err) Pending[T]` | A handle already settled to an error |
| `HTMLFragment` | A generated template with its parameters bound |
| `HTMLWrapper` | A generated template wrapper |
| `HTMLOption` | Tunes one render, extending the options `HTMLConfig` already supplies |

A template parameter declared `async T` becomes a `Pending[T]` field in the
generated `Params` struct. The context passed to `Go` bounds the work and stays
the caller's to cancel — a render bounds only how long it waits. A panic inside
the work becomes the handle's error and surfaces through the boundary's recover
clause instead of taking the process down.

`Resolved` is what a test passes instead of starting a goroutine. See
[Async rendering](/advanced/async-rendering/).

## Database

| Symbol | What it does |
| --- | --- |
| `DB(ctx) (*sql.DB, bool)` | The pool of the effective connection group |
| `DBDriver(ctx) (string, bool)` | The driver scheme of that pool |
| `SelectDB(ctx, group) context.Context` | Pins a named connection group |
| `SelectWriteDB(ctx) (context.Context, error)` | Pins the group framework-owned writes use |
| `SelectSessionDB(ctx) (context.Context, error)` | Pins the group holding the session table |
| `Transaction(ctx, fn, ...TxOption) error` | Runs `fn` in a transaction whose context the generated SQL uses |
| `OnGroup(group) TxOption` | Runs that transaction against a named group |

A nested `Transaction` opens a savepoint rather than a second transaction, so
its failure rolls back only its own work and leaves the outer one usable.
Without `OnGroup`, a transaction runs on the effective group of its context —
unpinned SQL inside it stays there instead of falling back to the default group.

`SelectDB` reports an unknown group name at the first statement that uses the
returned context, not at the call itself. `SelectWriteDB` can never select a
replica, which is what lets a caller that must write stay ignorant of the
deployment topology. See [Queries](/guides/backend/queries/).

## Logging

| Symbol | What it does |
| --- | --- |
| `Logger(ctx) Log` | The logger bound to the request, its stable attributes, and the active span |
| `Log` | The context-bound logger type |
| `WithLogAttributes(ctx, ...Attribute) context.Context` | Adds attributes to every record taken from the returned context |
| `String`, `Int`, `Int64`, `Float64`, `Bool`, `Duration`, `Err` | Attribute constructors |
| `Attribute` | One scalar key-value pair — the same type a span attribute uses |
| `Level`, `LevelTrace`…`LevelOff` | Severities; trace sits one step below debug, which `slog` does not name |

`Logger` never returns something that cannot be called, so no handler needs a
nil check. Acquire it again inside a child span to correlate records with that
span:

```go
ctx, span := pw.StartSpan(ctx, "load-user")
defer span.End()
pw.Logger(ctx).Info("loaded", pw.Int("rows", n))
```

There is no `Fatal` and no `Panic`. Logging reports what happened; it does not
decide whether the process keeps running.

Attribute constructors cover scalars only. A record must never fail to encode,
and a value that needs a structure belongs in attributes of its own.

## Tracing

| Symbol | What it does |
| --- | --- |
| `StartSpan(ctx, name, ...Attribute) (context.Context, *Span)` | Opens a child of the active span |
| `StartSpanKind(ctx, name, kind, ...Attribute)` | The same, for work that is not internal |
| `Span`, `SpanKind`, `SpanKindInternal`…`SpanKindConsumer` | The span type and its kinds |
| `StatusUnset`, `StatusOK`, `StatusError` | Span status codes |
| `TraceID(ctx) string`, `SpanID(ctx) string` | The current identifiers, or empty outside a trace |
| `Traced(ctx) bool` | Whether the context carries a valid span context |

Outside a trace, or with tracing disabled, `StartSpan` returns a span that
records nothing and costs nothing to end — the `defer span.End()` needs no
guard. The framework creates the request root span, so a handler starts only the
spans describing its own work.

`TraceID` is the value to show a user on an error page, since it is what
correlates their report with the records the server kept.

## OpenAPI and API documentation

| Symbol | What it does |
| --- | --- |
| `SetOpenAPIInfo(OpenAPIInfo) error` | Sets the document's title, version, and description |
| `AssembleOpenAPI() ([]byte, error)` | Builds the document from every registered operation |
| `OpenAPIJSON(w, r)` | Serves that document as a handler |
| `ScalarUI(specURL) http.Handler` | A Scalar reference page over the document at `specURL` |
| `SwaggerUI(specURL) http.Handler` | A Swagger UI page over the same |

Both UIs load their assets from a public CDN; nothing is embedded in the binary.
Mounting one by hand is the exception rather than the rule — `server.openapi`
and `server.api_doc` serve both without a route. See
[API Documentation](/productivity/api-documentation/).

## Extensions

| Symbol | What it does |
| --- | --- |
| `RegisterExtension(Extension)` | Adds one capability to the framework chain |
| `Extension` | `Name`, `Slot`, `Setup`, and `Close` |
| `Slot`, `SlotSession`, `SlotAuthentication`, `SlotGuard` | Position in the request chain; smaller runs earlier |
| `Middleware` | `func(http.Handler) http.Handler` |

An imported package calls `RegisterExtension` from an `init` function, so only
linked capabilities contribute configuration and code. `Setup` runs once during
framework initialization — after configuration parsing and database startup —
and returns the middleware to install. Returning a nil middleware installs
nothing, which is how a disabled extension opts out without a branch anywhere
else.

The slot ordering exists so a guard always observes state that was established
before it: the session at 10, authentication at 20, the guard at 30.
