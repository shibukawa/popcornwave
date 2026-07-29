package pw

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shibukawa/popcornwave/internal/pwenv"
	"github.com/shibukawa/popcornwave/middlewares"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinybind-go/configbind"
)

// ServerConfig controls the primary HTTP listener and operational endpoints.
type ServerConfig struct {
	Port              int           `opt:"port" env:"PORT" default:"8080" help:"HTTP listen port"`
	ReadHeaderTimeout time.Duration `default:"5s" help:"request header read timeout"`
	ReadTimeout       time.Duration `default:"30s" help:"request read timeout"`
	WriteTimeout      time.Duration `default:"0s" help:"response write timeout; zero permits long-lived streams"`
	IdleTimeout       time.Duration `default:"2m" help:"keep-alive idle timeout"`
	ShutdownTimeout   time.Duration `default:"10s" help:"graceful shutdown timeout"`
	MaxRequestBody    int64         `default:"10485760" help:"maximum request body in bytes"`
	TrustedProxies    []string      `help:"trusted proxy IP or CIDR"`
	// Health, Readiness, and OpenAPI are the paths their endpoints serve, and
	// an unset path serves nothing. They carry no default so that the operator
	// reading a deployment's configuration sees every address it answers on;
	// a default would leave three endpoints running that no file mentions.
	Health     string `help:"liveness endpoint path, e.g. /healthz; unset serves none"`
	Readiness  string `help:"readiness endpoint path, e.g. /readyz; unset serves none"`
	OpenAPI    string `key:"openapi" help:"OpenAPI document path, e.g. /openapi.json; unset serves none"`
	APIDoc     string `help:"API documentation UI: scalar, swagger, or empty to disable"`
	APIDocPath string `default:"/docs" dependon:".api_doc" help:"API documentation UI path"`
	// Public serves the application's embedded static assets.
	Public PublicConfig `help:"framework-owned static asset endpoint"`
}

// HTMLConfig bounds and gates progressive HTML rendering. A template that opens
// an await boundary renders correctly under either setting; this only decides
// whether the fallbacks reach the browser before the work behind them settles.
type HTMLConfig struct {
	// Streaming false forces the buffered branch even when a chain can open a
	// boundary, which is the escape hatch for a proxy that buffers responses.
	Streaming bool `default:"true" help:"Streaming false forces the buffered branch even when a chain can open a boundary, which is the escape hatch for a proxy that buffers responses"`
	// AsyncTimeout bounds one await boundary. Zero leaves the request context as
	// the only deadline.
	AsyncTimeout time.Duration `default:"3s" help:"AsyncTimeout bounds one await boundary. Zero leaves the request context as the only deadline"`
	// AsyncConcurrency bounds simultaneously running boundary work across one
	// render. Zero or less is unbounded.
	AsyncConcurrency int `default:"0" help:"AsyncConcurrency bounds simultaneously running boundary work across one render. Zero or less is unbounded"`
	// BotDetection forces the buffered branch for a client that will not run
	// the boundary runtime, so a crawler indexes the page instead of the
	// fallbacks. False skips classification entirely.
	BotDetection bool `default:"true" help:"render the settled document for crawlers and CLI clients"`
	// BotAsyncTimeout bounds one await boundary on a classified bot request,
	// which waits for every boundary before any byte leaves. Zero falls back to
	// AsyncTimeout rather than meaning unbounded, so a misread key cannot hold a
	// crawler connection open for the whole request deadline.
	BotAsyncTimeout time.Duration `default:"5s" dependon:".bot_detection" help:"await boundary bound for a classified bot request"`
	// BotUserAgents extends the built-in catalog. Entries are appended and
	// matched case-insensitively; they never replace a built-in token.
	BotUserAgents []string `dependon:".bot_detection" help:"additional bot User-Agent substrings"`
}

// defaultHTMLConfig seeds the effective configuration for a runtime that never
// parses a config source. The parse-time defaults live in the struct tags
// above; TestHTMLDefaultsMatchTags holds the two in agreement.
//
// BotAsyncTimeout sits above AsyncTimeout because an indexer waits far longer
// than a browser, and a timeout fallback baked into a buffered document is the
// outcome bot detection exists to prevent. It stays well short of the request
// deadline because a link preview spider abandons a slow response within a few
// seconds, and the buffered branch has no head start to offer it.
var defaultHTMLConfig = HTMLConfig{
	Streaming:       true,
	AsyncTimeout:    3 * time.Second,
	BotDetection:    true,
	BotAsyncTimeout: 5 * time.Second,
}

// PublicConfig controls the framework-owned static asset endpoint.
type PublicConfig = middlewares.PublicAssetConfig

// SecurityConfig controls framework request and response security policy.
type SecurityConfig struct {
	Headers SecurityHeadersConfig
}

// SecurityHeadersConfig controls browser-facing response headers.
type SecurityHeadersConfig = middlewares.SecurityHeadersConfig

// HSTSConfig controls Strict-Transport-Security on verified HTTPS requests.
type HSTSConfig = middlewares.HSTSConfig

// SessionConfig selects login-session behavior, cookie policy, and storage.
// Sessions are opaque and server-side, so no signing secret is configured.
type SessionConfig struct {
	Enabled bool `default:"false"`
	// Backend selects the storage plugin. Only rdb is implemented.
	Backend         string              `default:"rdb" dependon:".enabled" help:"session storage backend"`
	TTL             time.Duration       `default:"24h" dependon:".enabled" help:"absolute session lifetime"`
	IdleTimeout     time.Duration       `default:"0s" dependon:".enabled" help:"inactivity expiry; zero disables it"`
	RenewalInterval time.Duration       `default:"0s" dependon:".enabled" help:"minimum interval between idle expiry renewals"`
	Cookie          SessionCookieConfig `dependon:".enabled"`
	RDB             SessionRDBConfig    `dependon:".enabled"`
}

// SessionCookieConfig is the browser cookie policy of the session middleware.
type SessionCookieConfig struct {
	Name     string `default:"pw_session"`
	Path     string `default:"/"`
	Domain   string
	Secure   bool   `default:"true" help:"disable only for loopback development"`
	HTTPOnly bool   `default:"true"`
	SameSite string `default:"lax"`
}

// SessionRDBConfig configures the database-backed session store. The middleware
// source reuses the pool owned by middleware.rdb; the dedicated source opens
// its own pool from DSN.
type SessionRDBConfig struct {
	Source string `default:"middleware" help:"middleware reuses middleware.rdb; dedicated opens rdb.dsn"`
	// Group names the middleware-source connection group holding the session
	// table. Empty resolves to middleware.rdb.write_group.
	Group string `help:"connection group holding the session table"`
	DSN   string `secret:"mask" help:"dedicated session database DSN"`
	Table string `default:"popcornwave_session"`
}

// ObservabilityConfig controls runtime logging and service identity.
type ObservabilityConfig struct {
	MinimumLevel string `default:"info"`
	ServiceName  string `env:"OTEL_SERVICE_NAME"`
	// BootLog selects the startup summary format: auto, tree, record, or off.
	BootLog string `default:"auto" help:"startup summary: auto, tree, record, or off"`
	// Query configures the development query diagnostics.
	Query QueryLogConfig `help:"Query configures the development query diagnostics"`
}

// QueryLogConfig controls per-statement query logging, the slow-statement
// EXPLAIN, and the rerun snippet. It lives under observability because it
// produces log records; middleware.rdb stays pool configuration.
type QueryLogConfig struct {
	// Enabled is auto, on, or off. Auto resolves to on in the development
	// environment and off everywhere else. Off disables every key below it.
	Enabled string `default:"auto" falsy:"off" help:"log every generated SQL statement: auto, on, or off; auto is on in dev"`
	// Level is the severity of an ordinary statement record.
	Level string `default:"info" dependon:".enabled" help:"Level is the severity of an ordinary statement record"`
	// SlowThreshold is the duration above which a statement is slow. Zero
	// disables slow detection, and with it EXPLAIN and reproduction; falsy is
	// what lets those three name it as their parent.
	SlowThreshold time.Duration `default:"200ms" falsy:"0s" dependon:".enabled" help:"duration above which a statement is explained; zero disables it"`
	// SlowLevel is the severity of a slow statement record.
	SlowLevel string `default:"warn" dependon:".slow_threshold" help:"SlowLevel is the severity of a slow statement record"`
	// BindValues is auto, on, or off, and follows the same environment rule as
	// Enabled. It is the only path by which row values reach a query record.
	BindValues string `default:"auto" falsy:"off" dependon:".enabled" help:"log argument values: auto, on, or off; auto is on in dev"`
	// Explain captures a plan-only EXPLAIN for a slow statement.
	Explain bool `default:"true" dependon:".slow_threshold" help:"capture a plan-only EXPLAIN for a slow statement"`
	// Reproduction renders a paste-able rerun snippet for a slow statement.
	Reproduction bool `default:"true" dependon:".slow_threshold" help:"emit a paste-able rerun snippet for a slow statement"`
	// MaxSQLLength bounds the logged statement text.
	MaxSQLLength int `default:"4096" dependon:".enabled" help:"MaxSQLLength bounds the logged statement text"`
	// MaxValueLength bounds each logged argument value.
	MaxValueLength int `default:"256" dependon:".enabled" help:"MaxValueLength bounds each logged argument value"`
}

// MiddlewareConfig selects the framework's basic HTTP middleware.
type MiddlewareConfig struct {
	Recovery       bool          `default:"true"`
	RequestID      bool          `default:"true"`
	AccessLog      bool          `default:"true"`
	Compression    bool          `default:"false"`
	RequestTimeout time.Duration `default:"0s"`
	RDB            RDBConfig
}

// RDBConfig controls the framework-owned database pools.
//
// A single database is configured with DSN and the pool fields below. A
// reader-writer topology is configured with Connections instead, one element
// per pool. Declaring both is a configuration error rather than a merge.
type RDBConfig struct {
	Enabled         bool          `default:"false"`
	DSN             string        `secret:"mask"`
	ConnectTimeout  time.Duration `default:"5s" dependon:".enabled"`
	MaxOpenConns    int           `default:"0" dependon:".enabled"`
	MaxIdleConns    int           `default:"0" dependon:".enabled"`
	ConnMaxLifetime time.Duration `default:"0s" dependon:".enabled"`
	ConnMaxIdleTime time.Duration `default:"0s" dependon:".enabled"`
	// DefaultGroup serves statements that pin no group. It is normally the
	// replica group. Required once more than one group is configured.
	DefaultGroup string `dependon:".enabled" help:"connection group for statements that pin none"`
	// WriteGroup serves framework-owned writes. Empty resolves to the only
	// group holding a writable connection.
	WriteGroup string `dependon:".enabled" help:"connection group for framework-owned writes"`
	// MigrationGroup receives migrations and seed data. Empty resolves to
	// WriteGroup.
	MigrationGroup string `dependon:".enabled" help:"connection group for migrations and seeds"`
	// Connections is the array-of-tables form. An element has no CLI option, no
	// environment variable, and no dependon, because its identity is its
	// position in the file rather than a stable key.
	Connections []RDBConnectionConfig `help:"connection set, one element per pool"`
}

// RDBConnectionConfig is one pool of the connection set.
type RDBConnectionConfig struct {
	// Group is the name this connection is addressed by. Several connections
	// may share one group, which is what makes round robin expressible.
	Group string `help:"name this connection is addressed by"`
	DSN   string `secret:"mask"`
	// ReadOnly marks a replica: it opens read-only transactions and never
	// serves a framework-owned write.
	ReadOnly        bool          `key:"readonly" default:"false" help:"open read-only transactions and serve no framework write"`
	ConnectTimeout  time.Duration `default:"5s"`
	MaxOpenConns    int           `default:"0"`
	MaxIdleConns    int           `default:"0"`
	ConnMaxLifetime time.Duration `default:"0s"`
	ConnMaxIdleTime time.Duration `default:"0s"`
}

type configEntry struct {
	prefix string
	ptr    any
}

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

var configState = struct {
	sync.RWMutex
	entries  map[reflect.Type]configEntry
	parsed   bool
	parseErr error
	options  configbind.LoadOptions
	// db and dbDriver mirror the default group's connection for callers that
	// predate the connection set.
	db          *sql.DB
	dbDriver    string
	connections *pwruntime.ConnectionSet
	cleanups    []*runtimeCleanup
}{
	entries: make(map[reflect.Type]configEntry),
	options: configbind.LoadOptions{
		Vendor:   "popcornwave",
		FileName: "config.toml",
	},
}

// RegisterConfig registers one generated configbind target without parsing it.
func RegisterConfig[T any](prefix string) {
	if strings.TrimSpace(prefix) == "" {
		panic("popcornwave: empty configuration prefix")
	}
	typ := reflect.TypeFor[T]()
	configState.Lock()
	defer configState.Unlock()
	if configState.parsed {
		panic("popcornwave: configuration registered after ParseConfig")
	}
	if existing, ok := configState.entries[typ]; ok {
		if existing.prefix != prefix {
			panic(fmt.Sprintf("popcornwave: %v already registered with prefix %q", typ, existing.prefix))
		}
		return
	}
	configState.entries[typ] = configEntry{prefix: prefix, ptr: configbind.Bind[T](prefix)}
}

// SetConfigLoadOptions customizes configbind loading before ParseConfig.
func SetConfigLoadOptions(options configbind.LoadOptions) {
	configState.Lock()
	defer configState.Unlock()
	if configState.parsed {
		panic("popcornwave: config options changed after ParseConfig")
	}
	configState.options = options
}

func ParseConfig() error {
	configState.Lock()
	defer configState.Unlock()
	if configState.parsed {
		return configState.parseErr
	}
	configState.parsed = true
	options, env, envErr := resolveLoadOptions(configState.options)
	if envErr != nil {
		configState.parseErr = envErr
		return envErr
	}
	setEnv(env)
	var actionErr error
	options.Args, actionErr = parseFrameworkAction(commandArgs(options.Args))
	if actionErr != nil {
		configState.parseErr = actionErr
		return actionErr
	}
	result, err := configbind.Load(options)
	configState.parseErr = err
	if err != nil {
		return err
	}
	captureBootReport(result)
	return nil
}

// resolveLoadOptions completes options for the active runtime environment.
// Project-local candidates are environment-specific and searched in the working
// directory before its config/ directory; the user and system configuration
// directories keep the environment-neutral file name.
func resolveLoadOptions(options configbind.LoadOptions) (configbind.LoadOptions, string, error) {
	if options.Tool == "" {
		options.Tool = executableName()
	}
	env, err := pwenv.Resolve(options.Environ)
	if err != nil {
		return options, "", err
	}
	if options.FileName == "" {
		options.FileName = pwenv.NeutralFileName
	}
	if options.ExtraConfigReadPaths == nil {
		options.ExtraConfigReadPaths = pwenv.ReadPaths(env)
	}
	return options, env, nil
}

func executableName() string {
	name := filepath.Base(os.Args[0])
	if name == "" || name == "." {
		return "app"
	}
	return name
}

func registeredConfig[T any]() (T, bool) {
	var zero T
	configState.RLock()
	entry, ok := configState.entries[reflect.TypeFor[T]()]
	configState.RUnlock()
	if !ok {
		return zero, false
	}
	ptr, ok := entry.ptr.(*T)
	if !ok || ptr == nil {
		return zero, false
	}
	return *ptr, true
}

func runtimeResources(logger *slog.Logger) pwruntime.Resources {
	// Resolved before the lock because reading a registered binding takes the
	// same read lock, and a waiting writer would deadlock the reacquisition.
	query := resolveQueryDiagnostics(Config[ObservabilityConfig](nil), Env())
	configState.RLock()
	defer configState.RUnlock()
	configs := make(map[reflect.Type]any, len(configState.entries))
	for typ, entry := range configState.entries {
		value := reflect.ValueOf(entry.ptr)
		if value.Kind() == reflect.Pointer && !value.IsNil() {
			configs[typ] = value.Elem().Interface()
		}
	}
	return pwruntime.Resources{
		Configs:     configs,
		Logger:      logger,
		DB:          configState.db,
		DBDriver:    configState.dbDriver,
		Connections: configState.connections,
		Query:       query,
	}
}

// seedConfigDefaults gives a registered binding its documented values before
// anything parses a configuration source.
//
// Config reports a registered target even while it is unparsed, so a zero value
// there reads as an explicit setting rather than as "not configured yet". For
// HTMLConfig that distinction matters: a zero Streaming would turn progressive
// rendering off in every test and in every embedding that never calls
// ParseConfig, which is the opposite of the documented default.
// isSecretKey masks a value inside an array of tables. Provenance applies a
// secret tag and its own name check to every stable key, but an array element
// has no stable key, so this covers what the startup summary expands by hand.
func isSecretKey(key string) bool {
	key = strings.ToLower(key)
	for _, fragment := range []string{"secret", "password", "token", "credential", "dsn", "private_key"} {
		if strings.Contains(key, fragment) {
			return true
		}
	}
	return false
}

func seedConfigDefaults[T any](value T) {
	configState.Lock()
	defer configState.Unlock()
	entry, ok := configState.entries[reflect.TypeFor[T]()]
	if !ok {
		return
	}
	if ptr, ok := entry.ptr.(*T); ok && ptr != nil {
		*ptr = value
	}
}

func parseConfigDuration(overlay *configbind.Overlay, key string) (time.Duration, error) {
	raw := valueOf(overlay, key)
	duration, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return duration, nil
}

func parseConfigInt64(overlay *configbind.Overlay, key string) (int64, error) {
	raw := valueOf(overlay, key)
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return value, nil
}

func parseConfigInt(overlay *configbind.Overlay, key string) (int, error) {
	raw := valueOf(overlay, key)
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return value, nil
}

func configBool(overlay *configbind.Overlay, key string) bool {
	value, _ := strconv.ParseBool(valueOf(overlay, key))
	return value
}

func valueOf(overlay *configbind.Overlay, key string) string {
	value, _ := overlay.GetString(key)
	return value
}
