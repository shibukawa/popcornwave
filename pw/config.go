package pw

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shibukawa/popcornwave/middlewares"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinybind-go/cliparser"
	"github.com/shibukawa/tinybind-go/configbind"
)

// ServerConfig controls the primary HTTP listener and operational endpoints.
type ServerConfig struct {
	Port              int `opt:"port" env:"PORT" default:"8080" help:"HTTP listen port"`
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxRequestBody    int64
	TrustedProxies    []string
	Health            EndpointConfig
	Readiness         EndpointConfig
	OpenAPI           EndpointConfig
	Public            PublicConfig
}

// EndpointConfig enables one framework-owned HTTP endpoint.
type EndpointConfig struct {
	Enabled bool
	Path    string
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

// SessionConfig contains the currently available session runtime settings.
type SessionConfig struct {
	Enabled bool
	TTL     time.Duration
	Secret  string
}

// ObservabilityConfig controls runtime logging and service identity.
type ObservabilityConfig struct {
	MinimumLevel string
	ServiceName  string
}

// MiddlewareConfig selects the framework's basic HTTP middleware.
type MiddlewareConfig struct {
	Recovery       bool
	RequestID      bool
	AccessLog      bool
	Compression    bool
	RequestTimeout time.Duration
	RDB            RDBConfig
}

// RDBConfig controls the framework-owned database pool.
type RDBConfig struct {
	Enabled         bool
	DSN             string
	AutoTransaction bool
	ConnectTimeout  time.Duration
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
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
	result   *configbind.LoadResult
	options  configbind.LoadOptions
	db       *sql.DB
	cleanups []*runtimeCleanup
}{
	entries: make(map[reflect.Type]configEntry),
	options: configbind.LoadOptions{
		Vendor:               "popcornwave",
		FileName:             "config.toml",
		ExtraConfigReadPaths: []string{"config.toml"},
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
	options := configState.options
	if options.Tool == "" {
		options.Tool = executableName()
	}
	var actionErr error
	options.Args, actionErr = parseFrameworkAction(commandArgs(options.Args))
	if actionErr != nil {
		configState.parseErr = actionErr
		return actionErr
	}
	result, err := configbind.Load(options)
	configState.result, configState.parseErr = result, err
	if err != nil {
		return err
	}
	logConfigSources(result)
	return nil
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
	configState.RLock()
	defer configState.RUnlock()
	configs := make(map[reflect.Type]any, len(configState.entries))
	for typ, entry := range configState.entries {
		value := reflect.ValueOf(entry.ptr)
		if value.Kind() == reflect.Pointer && !value.IsNil() {
			configs[typ] = value.Elem().Interface()
		}
	}
	return pwruntime.Resources{Configs: configs, Logger: logger, DB: configState.db}
}

func logConfigSources(result *configbind.LoadResult) {
	if result == nil || result.Overlay == nil {
		return
	}
	keys := result.Overlay.Keys()
	sort.Strings(keys)
	for _, key := range keys {
		entry, ok := result.Overlay.Get(key)
		if !ok {
			continue
		}
		value := entry.Raw
		if isSecretKey(key) {
			value = "[REDACTED]"
		}
		slog.Info("config loaded", "key", key, "value", value, "source", entry.Place)
	}
}

func isSecretKey(key string) bool {
	key = strings.ToLower(key)
	for _, fragment := range []string{"secret", "password", "token", "credential", "dsn", "private_key"} {
		if strings.Contains(key, fragment) {
			return true
		}
	}
	return false
}

func init() {
	registerBuiltinConfigs()
	RegisterConfig[ServerConfig]("server")
	RegisterConfig[SecurityConfig]("security")
	RegisterConfig[SessionConfig]("session")
	RegisterConfig[ObservabilityConfig]("observability")
	RegisterConfig[MiddlewareConfig]("middleware")
}

func registerBuiltinConfigs() {
	registerServerConfig()
	registerSecurityConfig()
	registerSessionConfig()
	registerObservabilityConfig()
	registerMiddlewareConfig()
}

func registerServerConfig() {
	const typeName = "github.com/shibukawa/popcornwave/pw.ServerConfig"
	defaults := map[string]string{
		"server.port":                "8080",
		"server.read_header_timeout": "5s",
		"server.read_timeout":        "30s",
		"server.write_timeout":       "0s",
		"server.idle_timeout":        "2m",
		"server.shutdown_timeout":    "10s",
		"server.max_request_body":    "10485760",
		"server.health.enabled":      "true",
		"server.health.path":         "/healthz",
		"server.readiness.enabled":   "true",
		"server.readiness.path":      "/readyz",
		"server.openapi.enabled":     "true",
		"server.openapi.path":        "/openapi.json",
		"server.public.enabled":      "true",
		"server.public.mount":        "/public",
		"server.public.read_local":   "false",
	}
	keys := []string{
		"server.port", "server.read_header_timeout", "server.read_timeout",
		"server.write_timeout", "server.idle_timeout", "server.shutdown_timeout",
		"server.max_request_body", "server.trusted_proxies",
		"server.health.enabled", "server.health.path",
		"server.readiness.enabled", "server.readiness.path",
		"server.openapi.enabled", "server.openapi.path",
		"server.public.enabled", "server.public.mount", "server.public.read_local",
	}
	configbind.Register[ServerConfig](configbind.Definition{
		TypeName:  typeName,
		Prefix:    "server",
		KnownKeys: keys,
		Defaults:  defaults,
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "server", Key: "port", Opt: "port", Env: "PORT", Help: "HTTP listen port"},
			{Prefix: "server", Key: "read_header_timeout", Help: "request header read timeout"},
			{Prefix: "server", Key: "read_timeout", Help: "request read timeout"},
			{Prefix: "server", Key: "write_timeout", Help: "response write timeout"},
			{Prefix: "server", Key: "idle_timeout", Help: "keep-alive idle timeout"},
			{Prefix: "server", Key: "shutdown_timeout", Help: "graceful shutdown timeout"},
			{Prefix: "server", Key: "max_request_body", Help: "maximum request body in bytes"},
			{Prefix: "server", Key: "trusted_proxies", Kind: cliparser.KindArray, Help: "trusted proxy IP or CIDR"},
			{Prefix: "server", Key: "health.enabled", Kind: cliparser.KindBool},
			{Prefix: "server", Key: "health.path"},
			{Prefix: "server", Key: "readiness.enabled", Kind: cliparser.KindBool},
			{Prefix: "server", Key: "readiness.path"},
			{Prefix: "server", Key: "openapi.enabled", Kind: cliparser.KindBool},
			{Prefix: "server", Key: "openapi.path"},
			{Prefix: "server", Key: "public.enabled", Kind: cliparser.KindBool},
			{Prefix: "server", Key: "public.mount"},
			{Prefix: "server", Key: "public.read_local", Kind: cliparser.KindBool},
		},
		Apply: func(dst any, overlay *configbind.Overlay) error {
			p, ok := dst.(*ServerConfig)
			if !ok || p == nil {
				return fmt.Errorf("configbind: bad ServerConfig destination")
			}
			var err error
			p.Port, err = parseConfigInt(overlay, "server.port")
			if err != nil {
				return err
			}
			for key, target := range map[string]*time.Duration{
				"read_header_timeout": &p.ReadHeaderTimeout,
				"read_timeout":        &p.ReadTimeout,
				"write_timeout":       &p.WriteTimeout,
				"idle_timeout":        &p.IdleTimeout,
				"shutdown_timeout":    &p.ShutdownTimeout,
			} {
				*target, err = parseConfigDuration(overlay, "server."+key)
				if err != nil {
					return err
				}
			}
			p.MaxRequestBody, err = parseConfigInt64(overlay, "server.max_request_body")
			if err != nil {
				return err
			}
			p.TrustedProxies, _ = overlay.GetMulti("server.trusted_proxies")
			if err := applyEndpointConfig(overlay, "server.health", &p.Health); err != nil {
				return err
			}
			if err := applyEndpointConfig(overlay, "server.readiness", &p.Readiness); err != nil {
				return err
			}
			if err := applyEndpointConfig(overlay, "server.openapi", &p.OpenAPI); err != nil {
				return err
			}
			p.Public.Enabled = configBool(overlay, "server.public.enabled")
			p.Public.Mount = valueOf(overlay, "server.public.mount")
			p.Public.ReadLocal = configBool(overlay, "server.public.read_local")
			return nil
		},
		Scaffold: []configbind.ScaffoldField{
			{Key: "port", Kind: configbind.ScaffoldInt, Default: "8080", Opt: "port", Env: "PORT", Help: "HTTP listen port"},
			{Key: "read_header_timeout", Kind: configbind.ScaffoldString, Default: "5s", Help: "request header read timeout"},
			{Key: "read_timeout", Kind: configbind.ScaffoldString, Default: "30s", Help: "request read timeout"},
			{Key: "write_timeout", Kind: configbind.ScaffoldString, Default: "0s", Help: "response write timeout; zero permits long-lived streams"},
			{Key: "idle_timeout", Kind: configbind.ScaffoldString, Default: "2m", Help: "keep-alive idle timeout"},
			{Key: "shutdown_timeout", Kind: configbind.ScaffoldString, Default: "10s", Help: "graceful shutdown timeout"},
			{Key: "max_request_body", Kind: configbind.ScaffoldInt, Default: "10485760", Help: "maximum request body in bytes"},
			{Key: "trusted_proxies", Kind: configbind.ScaffoldStringSlice, Help: "trusted proxy IP or CIDR"},
			{Key: "health.enabled", Kind: configbind.ScaffoldBool, Default: "true"},
			{Key: "health.path", Kind: configbind.ScaffoldString, Default: "/healthz"},
			{Key: "readiness.enabled", Kind: configbind.ScaffoldBool, Default: "true"},
			{Key: "readiness.path", Kind: configbind.ScaffoldString, Default: "/readyz"},
			{Key: "openapi.enabled", Kind: configbind.ScaffoldBool, Default: "true"},
			{Key: "openapi.path", Kind: configbind.ScaffoldString, Default: "/openapi.json"},
			{Key: "public.enabled", Kind: configbind.ScaffoldBool, Default: "true"},
			{Key: "public.mount", Kind: configbind.ScaffoldString, Default: "/public"},
			{Key: "public.read_local", Kind: configbind.ScaffoldBool, Default: "false"},
		},
	})
}

func registerSecurityConfig() {
	const typeName = "github.com/shibukawa/popcornwave/pw.SecurityConfig"
	defaults := map[string]string{
		"security.headers.enabled":                             "true",
		"security.headers.content_type_options":                "true",
		"security.headers.frame_options":                       "deny",
		"security.headers.referrer_policy":                     "strict-origin-when-cross-origin",
		"security.headers.content_security_policy":             "",
		"security.headers.content_security_policy_report_only": "",
		"security.headers.permissions_policy":                  "",
		"security.headers.hsts.enabled":                        "false",
		"security.headers.hsts.max_age":                        "0s",
		"security.headers.hsts.include_subdomains":             "false",
		"security.headers.hsts.preload":                        "false",
	}
	keys := make([]string, 0, len(defaults))
	for key := range defaults {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	configbind.Register[SecurityConfig](configbind.Definition{
		TypeName:  typeName,
		Prefix:    "security",
		KnownKeys: keys,
		Defaults:  defaults,
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "security", Key: "headers.enabled", Kind: cliparser.KindBool},
			{Prefix: "security", Key: "headers.content_type_options", Kind: cliparser.KindBool},
			{Prefix: "security", Key: "headers.frame_options"},
			{Prefix: "security", Key: "headers.referrer_policy"},
			{Prefix: "security", Key: "headers.content_security_policy", Env: "-"},
			{Prefix: "security", Key: "headers.content_security_policy_report_only", Env: "-"},
			{Prefix: "security", Key: "headers.permissions_policy", Env: "-"},
			{Prefix: "security", Key: "headers.hsts.enabled", Kind: cliparser.KindBool},
			{Prefix: "security", Key: "headers.hsts.max_age"},
			{Prefix: "security", Key: "headers.hsts.include_subdomains", Kind: cliparser.KindBool},
			{Prefix: "security", Key: "headers.hsts.preload", Kind: cliparser.KindBool},
		},
		Apply: func(dst any, overlay *configbind.Overlay) error {
			p, ok := dst.(*SecurityConfig)
			if !ok || p == nil {
				return fmt.Errorf("configbind: bad SecurityConfig destination")
			}
			p.Headers.Enabled = configBool(overlay, "security.headers.enabled")
			p.Headers.ContentTypeOptions = configBool(overlay, "security.headers.content_type_options")
			p.Headers.FrameOptions = valueOf(overlay, "security.headers.frame_options")
			p.Headers.ReferrerPolicy = valueOf(overlay, "security.headers.referrer_policy")
			p.Headers.ContentSecurityPolicy = valueOf(overlay, "security.headers.content_security_policy")
			p.Headers.ContentSecurityPolicyReportOnly = valueOf(overlay, "security.headers.content_security_policy_report_only")
			p.Headers.PermissionsPolicy = valueOf(overlay, "security.headers.permissions_policy")
			p.Headers.HSTS.Enabled = configBool(overlay, "security.headers.hsts.enabled")
			p.Headers.HSTS.IncludeSubdomains = configBool(overlay, "security.headers.hsts.include_subdomains")
			p.Headers.HSTS.Preload = configBool(overlay, "security.headers.hsts.preload")
			duration, err := parseConfigDuration(overlay, "security.headers.hsts.max_age")
			if err != nil {
				return err
			}
			p.Headers.HSTS.MaxAge = duration
			return nil
		},
		Scaffold: []configbind.ScaffoldField{
			{Key: "headers.enabled", Kind: configbind.ScaffoldBool, Default: "true"},
			{Key: "headers.content_type_options", Kind: configbind.ScaffoldBool, Default: "true"},
			{Key: "headers.frame_options", Kind: configbind.ScaffoldString, Default: "deny"},
			{Key: "headers.referrer_policy", Kind: configbind.ScaffoldString, Default: "strict-origin-when-cross-origin"},
			{Key: "headers.content_security_policy", Kind: configbind.ScaffoldString, Env: "-"},
			{Key: "headers.content_security_policy_report_only", Kind: configbind.ScaffoldString, Env: "-"},
			{Key: "headers.permissions_policy", Kind: configbind.ScaffoldString, Env: "-"},
			{Key: "headers.hsts.enabled", Kind: configbind.ScaffoldBool, Default: "false"},
			{Key: "headers.hsts.max_age", Kind: configbind.ScaffoldString, Default: "0s"},
			{Key: "headers.hsts.include_subdomains", Kind: configbind.ScaffoldBool, Default: "false"},
			{Key: "headers.hsts.preload", Kind: configbind.ScaffoldBool, Default: "false"},
		},
	})
}

func registerSessionConfig() {
	const typeName = "github.com/shibukawa/popcornwave/pw.SessionConfig"
	configbind.Register[SessionConfig](configbind.Definition{
		TypeName:  typeName,
		Prefix:    "session",
		KnownKeys: []string{"session.enabled", "session.ttl", "session.secret"},
		Defaults:  map[string]string{"session.enabled": "false", "session.ttl": "24h", "session.secret": ""},
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "session", Key: "enabled", Kind: cliparser.KindBool},
			{Prefix: "session", Key: "ttl"},
			{Prefix: "session", Key: "secret", Env: "SESSION_SECRET"},
		},
		Apply: func(dst any, overlay *configbind.Overlay) error {
			p := dst.(*SessionConfig)
			raw, _ := overlay.GetString("session.enabled")
			p.Enabled, _ = strconv.ParseBool(raw)
			raw, _ = overlay.GetString("session.ttl")
			ttl, err := time.ParseDuration(raw)
			if err != nil {
				return err
			}
			p.TTL = ttl
			p.Secret, _ = overlay.GetString("session.secret")
			return nil
		},
		Scaffold: []configbind.ScaffoldField{
			{Key: "enabled", Kind: configbind.ScaffoldBool, Default: "false"},
			{Key: "ttl", Kind: configbind.ScaffoldString, Default: "24h"},
			{Key: "secret", Kind: configbind.ScaffoldString, Default: "", Env: "SESSION_SECRET"},
		},
	})
}

func registerObservabilityConfig() {
	const typeName = "github.com/shibukawa/popcornwave/pw.ObservabilityConfig"
	configbind.Register[ObservabilityConfig](configbind.Definition{
		TypeName:  typeName,
		Prefix:    "observability",
		KnownKeys: []string{"observability.minimum_level", "observability.service_name"},
		Defaults:  map[string]string{"observability.minimum_level": "info", "observability.service_name": ""},
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "observability", Key: "minimum_level"},
			{Prefix: "observability", Key: "service_name", Env: "OTEL_SERVICE_NAME"},
		},
		Apply: func(dst any, overlay *configbind.Overlay) error {
			p := dst.(*ObservabilityConfig)
			p.MinimumLevel, _ = overlay.GetString("observability.minimum_level")
			p.ServiceName, _ = overlay.GetString("observability.service_name")
			return nil
		},
		Scaffold: []configbind.ScaffoldField{
			{Key: "minimum_level", Kind: configbind.ScaffoldString, Default: "info"},
			{Key: "service_name", Kind: configbind.ScaffoldString, Default: "", Env: "OTEL_SERVICE_NAME"},
		},
	})
}

func registerMiddlewareConfig() {
	const typeName = "github.com/shibukawa/popcornwave/pw.MiddlewareConfig"
	configbind.Register[MiddlewareConfig](configbind.Definition{
		TypeName: typeName,
		Prefix:   "middleware",
		KnownKeys: []string{
			"middleware.recovery", "middleware.request_id", "middleware.access_log",
			"middleware.compression", "middleware.request_timeout",
			"middleware.rdb.enabled", "middleware.rdb.dsn",
			"middleware.rdb.auto_transaction", "middleware.rdb.connect_timeout",
			"middleware.rdb.max_open_conns", "middleware.rdb.max_idle_conns",
			"middleware.rdb.conn_max_lifetime", "middleware.rdb.conn_max_idle_time",
		},
		Defaults: map[string]string{
			"middleware.recovery": "true", "middleware.request_id": "true",
			"middleware.access_log": "true", "middleware.compression": "false",
			"middleware.request_timeout": "0s",
			"middleware.rdb.enabled":     "false", "middleware.rdb.dsn": "",
			"middleware.rdb.auto_transaction": "true",
			"middleware.rdb.connect_timeout":  "5s",
			"middleware.rdb.max_open_conns":   "0", "middleware.rdb.max_idle_conns": "0",
			"middleware.rdb.conn_max_lifetime": "0s", "middleware.rdb.conn_max_idle_time": "0s",
		},
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "middleware", Key: "recovery", Kind: cliparser.KindBool},
			{Prefix: "middleware", Key: "request_id", Kind: cliparser.KindBool},
			{Prefix: "middleware", Key: "access_log", Kind: cliparser.KindBool},
			{Prefix: "middleware", Key: "compression", Kind: cliparser.KindBool},
			{Prefix: "middleware", Key: "request_timeout"},
			{Prefix: "middleware.rdb", Key: "enabled", Kind: cliparser.KindBool},
			{Prefix: "middleware.rdb", Key: "dsn"},
			{Prefix: "middleware.rdb", Key: "auto_transaction", Kind: cliparser.KindBool},
			{Prefix: "middleware.rdb", Key: "connect_timeout"},
			{Prefix: "middleware.rdb", Key: "max_open_conns"},
			{Prefix: "middleware.rdb", Key: "max_idle_conns"},
			{Prefix: "middleware.rdb", Key: "conn_max_lifetime"},
			{Prefix: "middleware.rdb", Key: "conn_max_idle_time"},
		},
		Apply: func(dst any, overlay *configbind.Overlay) error {
			p, ok := dst.(*MiddlewareConfig)
			if !ok || p == nil {
				return fmt.Errorf("configbind: bad MiddlewareConfig destination")
			}
			p.Recovery = configBool(overlay, "middleware.recovery")
			p.RequestID = configBool(overlay, "middleware.request_id")
			p.AccessLog = configBool(overlay, "middleware.access_log")
			p.Compression = configBool(overlay, "middleware.compression")
			var err error
			p.RequestTimeout, err = parseConfigDuration(overlay, "middleware.request_timeout")
			if err != nil {
				return err
			}
			p.RDB.Enabled = configBool(overlay, "middleware.rdb.enabled")
			p.RDB.DSN = valueOf(overlay, "middleware.rdb.dsn")
			p.RDB.AutoTransaction = configBool(overlay, "middleware.rdb.auto_transaction")
			p.RDB.ConnectTimeout, err = parseConfigDuration(overlay, "middleware.rdb.connect_timeout")
			if err != nil {
				return err
			}
			p.RDB.MaxOpenConns, err = parseConfigInt(overlay, "middleware.rdb.max_open_conns")
			if err != nil {
				return err
			}
			p.RDB.MaxIdleConns, err = parseConfigInt(overlay, "middleware.rdb.max_idle_conns")
			if err != nil {
				return err
			}
			p.RDB.ConnMaxLifetime, err = parseConfigDuration(overlay, "middleware.rdb.conn_max_lifetime")
			if err != nil {
				return err
			}
			p.RDB.ConnMaxIdleTime, err = parseConfigDuration(overlay, "middleware.rdb.conn_max_idle_time")
			return err
		},
		Scaffold: []configbind.ScaffoldField{
			{Key: "recovery", Kind: configbind.ScaffoldBool, Default: "true"},
			{Key: "request_id", Kind: configbind.ScaffoldBool, Default: "true"},
			{Key: "access_log", Kind: configbind.ScaffoldBool, Default: "true"},
			{Key: "compression", Kind: configbind.ScaffoldBool, Default: "false"},
			{Key: "request_timeout", Kind: configbind.ScaffoldString, Default: "0s"},
			{Key: "rdb.enabled", Kind: configbind.ScaffoldBool, Default: "false"},
			{Key: "rdb.dsn", Kind: configbind.ScaffoldString, Default: ""},
			{Key: "rdb.auto_transaction", Kind: configbind.ScaffoldBool, Default: "true"},
			{Key: "rdb.connect_timeout", Kind: configbind.ScaffoldString, Default: "5s"},
			{Key: "rdb.max_open_conns", Kind: configbind.ScaffoldInt, Default: "0"},
			{Key: "rdb.max_idle_conns", Kind: configbind.ScaffoldInt, Default: "0"},
			{Key: "rdb.conn_max_lifetime", Kind: configbind.ScaffoldString, Default: "0s"},
			{Key: "rdb.conn_max_idle_time", Kind: configbind.ScaffoldString, Default: "0s"},
		},
	})
}

func applyEndpointConfig(overlay *configbind.Overlay, prefix string, target *EndpointConfig) error {
	target.Enabled = configBool(overlay, prefix+".enabled")
	target.Path = valueOf(overlay, prefix+".path")
	return nil
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
