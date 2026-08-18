package pw

import (
	"context"
	"sync"

	"github.com/shibukawa/popcornweb/contrib/otel/metric"
	"github.com/shibukawa/popcornweb/middlewares"
	"github.com/shibukawa/popcornweb/pwconfig"
	"github.com/shibukawa/popcornweb/pwdatabase"
	"github.com/shibukawa/popcornweb/pwobservability"
	"github.com/shibukawa/popcornweb/pwruntime"
	"github.com/shibukawa/popcornweb/sessionconfig"
	"github.com/shibukawa/tinybind-go/configbind"
)

// The framework's own configuration bindings live in popcornweb/pwconfig, and
// every type below is a true alias of the one declared there.
//
// They moved for the reason the session types moved before them: a settings
// file is not a transport concern, so the runtime that reads it and the runtime
// that serves the request need not be the same one. What did not move is the
// name an application writes, which is why these are aliases.
//
// The alias must stay an alias. A defined type here would be a different
// reflect.Type, and the registry lookup — which is keyed by type — would
// silently miss.
type (
	// ServerConfig controls the primary HTTP listener and operational endpoints.
	ServerConfig = pwconfig.ServerConfig
	// HTMLConfig bounds and gates progressive HTML rendering.
	HTMLConfig = pwconfig.HTMLConfig
	// HTMLUpdateConfig controls partial updates.
	HTMLUpdateConfig = pwconfig.HTMLUpdateConfig
	// HTMLCacheConfig controls the output cache a component asks for with the
	// template's cache annotation.
	HTMLCacheConfig = pwconfig.HTMLCacheConfig
	// SecurityConfig controls framework request and response security policy.
	SecurityConfig = pwconfig.SecurityConfig
	// ObservabilityConfig controls runtime logging, tracing, and service
	// identity.
	ObservabilityConfig = pwconfig.ObservabilityConfig
	// TraceConfig selects the spans the framework opens inside a request.
	TraceConfig = pwconfig.TraceConfig
	// MetricsConfig selects the instruments the framework records.
	MetricsConfig = pwconfig.MetricsConfig
	// OtelExportConfig configures OTLP/HTTP export of traces and logs.
	OtelExportConfig = pwconfig.OtelExportConfig
	// QueryLogConfig controls per-statement query logging.
	QueryLogConfig = pwconfig.QueryLogConfig
	// MiddlewareConfig selects the framework's basic HTTP middleware.
	MiddlewareConfig = pwconfig.MiddlewareConfig
	// RDBConfig controls the framework-owned database pools.
	RDBConfig = pwconfig.RDBConfig
	// RDBConnectionConfig is one pool of the connection set.
	RDBConnectionConfig = pwconfig.RDBConnectionConfig
)

// PublicConfig controls the framework-owned static asset endpoint.
type PublicConfig = middlewares.PublicAssetConfig

// CSRFConfig controls the synchronizer-token check on unsafe browser requests.
type CSRFConfig = middlewares.CSRFConfig

// RateLimitConfig bounds how often one caller, and the process as a whole, may
// arrive within a window. It is its own binding rather than a member of
// SecurityConfig, because a backend selection carrying a DSN does not belong
// beside a list of response headers.
type RateLimitConfig = middlewares.RateLimitConfig

// RateLimitRedisConfig addresses the shared counter server.
type RateLimitRedisConfig = middlewares.RateLimitRedisConfig

// SecurityHeadersConfig controls browser-facing response headers.
type SecurityHeadersConfig = middlewares.SecurityHeadersConfig

// HSTSConfig controls Strict-Transport-Security on verified HTTPS requests.
type HSTSConfig = middlewares.HSTSConfig

// Session storage backends. They name which server backend a server-placed
// slot uses, never whether a slot is server-placed, which
// RegisterSessionStore states instead.
const (
	SessionBackendRDB         = sessionconfig.SessionBackendRDB
	SessionBackendCookie      = sessionconfig.SessionBackendCookie
	SessionBackendDevVolatile = sessionconfig.SessionBackendDevVolatile
	SessionBackendDevPersist  = sessionconfig.SessionBackendDevPersist
	SessionBackendRedis       = sessionconfig.SessionBackendRedis
	SessionBackendDynamo      = sessionconfig.SessionBackendDynamo
	SessionBackendFirestore   = sessionconfig.SessionBackendFirestore
)

// The [session] binding. Every type is a true alias of the one declared in
// sessionconfig, so pw and popcornweb/plugin/auth name one type from two
// packages without depending on each other, and the reflect.Type keyed
// configuration registry resolves both names to one entry.
type (
	SessionConfig            = sessionconfig.SessionConfig
	SessionCookieConfig      = sessionconfig.SessionCookieConfig
	SessionRDBConfig         = sessionconfig.SessionRDBConfig
	SessionRedisConfig       = sessionconfig.SessionRedisConfig
	SessionDynamoConfig      = sessionconfig.SessionDynamoConfig
	SessionCookieStoreConfig = sessionconfig.SessionCookieStoreConfig
	SessionKeyringConfig     = sessionconfig.SessionKeyringConfig
)

// RegisterConfig registers one generated configbind target without parsing it.
func RegisterConfig[T any](prefix string) { pwconfig.Register[T](prefix) }

// SetConfigLoadOptions customizes configbind loading before ParseConfig.
func SetConfigLoadOptions(options configbind.LoadOptions) { pwconfig.SetLoadOptions(options) }

// ParseConfig loads every registered binding.
//
// The load itself is pwconfig's; what this runtime adds is the two things
// around it, installed as hooks below: the framework subcommands come off the
// command line before configbind sees it, and the startup summary is captured
// from the result. Neither is a settings question, which is why neither moved.
func ParseConfig() error { return pwconfig.Parse() }

func init() {
	pwconfig.SetHooks(pwconfig.Hooks{
		Args:   parseFrameworkAction,
		Loaded: captureBootReport,
	})
}

// runtimeCleanup releases something an extension or a pool opened. It is
// per-process rather than per-request, and it is not configuration: it is what
// startup produced.
type runtimeCleanup struct {
	name string
	fn   func(context.Context) error
	once sync.Once
	err  error
}

func (cleanup *runtimeCleanup) run(ctx context.Context) error {
	cleanup.once.Do(func() { cleanup.err = cleanup.fn(ctx) })
	return cleanup.err
}

// runtimeState is what startup left this runtime to release. The settings live
// in pwconfig and the pools in pwdatabase; what is left here is the ordered
// list of closers, which is the one part of shutdown that is this runtime's.
var runtimeState = struct {
	sync.RWMutex
	cleanups []*runtimeCleanup
}{}

// runtimeResources builds the capsule every request is served with. exporting
// says whether telemetry this process produces has anywhere to go, which is what
// the automatic tracing and metrics settings read.
func runtimeResources(backend *pwruntime.LogBackend, meters *metric.Provider, exporting bool) pwruntime.Resources {
	observability := Config[ObservabilityConfig](nil)
	query := resolveQueryDiagnostics(observability, Development())
	tracing := resolveTracing(observability, exporting)
	metrics := pwobservability.MetricsPolicy(observability, meters, exporting)
	registerObservedMetrics(observability, meters, metrics)
	db, driver := pwdatabase.Default()
	return pwruntime.Resources{
		Configs:     pwconfig.Snapshot(),
		Log:         backend,
		DB:          db,
		DBDriver:    driver,
		Connections: pwdatabase.Connections(),
		Query:       query,
		Trace:       tracing,
		Metrics:     metrics,
	}
}
