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

	"github.com/shibukawa/popcornwave/internal/pwenv"
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
	APIDoc            string
	APIDocPath        string
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

// SessionConfig selects login-session behavior, cookie policy, and storage.
// Sessions are opaque and server-side, so no signing secret is configured.
type SessionConfig struct {
	Enabled bool
	// Backend selects the storage plugin. Only rdb is implemented.
	Backend         string
	TTL             time.Duration
	IdleTimeout     time.Duration
	RenewalInterval time.Duration
	Cookie          SessionCookieConfig
	RDB             SessionRDBConfig
}

// SessionCookieConfig is the browser cookie policy of the session middleware.
type SessionCookieConfig struct {
	Name     string
	Path     string
	Domain   string
	Secure   bool
	HTTPOnly bool
	SameSite string
}

// SessionRDBConfig configures the database-backed session store. The middleware
// source reuses the pool owned by middleware.rdb; the dedicated source opens
// its own pool from DSN.
type SessionRDBConfig struct {
	Source string
	DSN    string
	Table  string
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
	dbDriver string
	cleanups []*runtimeCleanup
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
	configState.result, configState.parseErr = result, err
	if err != nil {
		return err
	}
	logConfigSources(result)
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
	configState.RLock()
	defer configState.RUnlock()
	configs := make(map[reflect.Type]any, len(configState.entries))
	for typ, entry := range configState.entries {
		value := reflect.ValueOf(entry.ptr)
		if value.Kind() == reflect.Pointer && !value.IsNil() {
			configs[typ] = value.Elem().Interface()
		}
	}
	return pwruntime.Resources{Configs: configs, Logger: logger, DB: configState.db, DBDriver: configState.dbDriver}
}

func logConfigSources(result *configbind.LoadResult) {
	if result == nil || result.Overlay == nil {
		return
	}
	if result.FoundFile {
		slog.Info("config file resolved", "environment", Env(), "path", result.ConfigPath)
	} else {
		slog.Info("config file not found", "environment", Env())
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
		"server.api_doc":             "",
		"server.api_doc_path":        "/docs",
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
		"server.api_doc", "server.api_doc_path",
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
			{Prefix: "server", Key: "api_doc", Help: "API documentation UI: scalar, swagger, or empty to disable"},
			{Prefix: "server", Key: "api_doc_path", Help: "API documentation UI path"},
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
			p.APIDoc = valueOf(overlay, "server.api_doc")
			p.APIDocPath = valueOf(overlay, "server.api_doc_path")
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
			{Key: "api_doc", Kind: configbind.ScaffoldString, Help: "API documentation UI: scalar, swagger, or empty to disable"},
			{Key: "api_doc_path", Kind: configbind.ScaffoldString, Default: "/docs", Help: "API documentation UI path"},
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
	defaults := map[string]string{
		"session.enabled":          "false",
		"session.backend":          "rdb",
		"session.ttl":              "24h",
		"session.idle_timeout":     "0s",
		"session.renewal_interval": "0s",
		"session.cookie.name":      "pw_session",
		"session.cookie.path":      "/",
		"session.cookie.domain":    "",
		"session.cookie.secure":    "true",
		"session.cookie.http_only": "true",
		"session.cookie.same_site": "lax",
		"session.rdb.source":       "middleware",
		"session.rdb.dsn":          "",
		"session.rdb.table":        "popcornwave_session",
	}
	keys := make([]string, 0, len(defaults))
	for key := range defaults {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	configbind.Register[SessionConfig](configbind.Definition{
		TypeName:  typeName,
		Prefix:    "session",
		KnownKeys: keys,
		Defaults:  defaults,
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "session", Key: "enabled", Kind: cliparser.KindBool},
			{Prefix: "session", Key: "backend", Help: "session storage backend"},
			{Prefix: "session", Key: "ttl", Help: "absolute session lifetime"},
			{Prefix: "session", Key: "idle_timeout", Help: "inactivity expiry; zero disables it"},
			{Prefix: "session", Key: "renewal_interval", Help: "minimum interval between idle expiry renewals"},
			{Prefix: "session", Key: "cookie.name"},
			{Prefix: "session", Key: "cookie.path"},
			{Prefix: "session", Key: "cookie.domain"},
			{Prefix: "session", Key: "cookie.secure", Kind: cliparser.KindBool},
			{Prefix: "session", Key: "cookie.http_only", Kind: cliparser.KindBool},
			{Prefix: "session", Key: "cookie.same_site"},
			{Prefix: "session", Key: "rdb.source", Help: "middleware reuses middleware.rdb; dedicated opens rdb.dsn"},
			{Prefix: "session", Key: "rdb.dsn", Help: "dedicated session database DSN"},
			{Prefix: "session", Key: "rdb.table"},
		},
		Apply: func(dst any, overlay *configbind.Overlay) error {
			p, ok := dst.(*SessionConfig)
			if !ok || p == nil {
				return fmt.Errorf("configbind: bad SessionConfig destination")
			}
			p.Enabled = configBool(overlay, "session.enabled")
			p.Backend = valueOf(overlay, "session.backend")
			var err error
			for key, target := range map[string]*time.Duration{
				"ttl":              &p.TTL,
				"idle_timeout":     &p.IdleTimeout,
				"renewal_interval": &p.RenewalInterval,
			} {
				*target, err = parseConfigDuration(overlay, "session."+key)
				if err != nil {
					return err
				}
			}
			p.Cookie = SessionCookieConfig{
				Name:     valueOf(overlay, "session.cookie.name"),
				Path:     valueOf(overlay, "session.cookie.path"),
				Domain:   valueOf(overlay, "session.cookie.domain"),
				Secure:   configBool(overlay, "session.cookie.secure"),
				HTTPOnly: configBool(overlay, "session.cookie.http_only"),
				SameSite: valueOf(overlay, "session.cookie.same_site"),
			}
			p.RDB = SessionRDBConfig{
				Source: valueOf(overlay, "session.rdb.source"),
				DSN:    valueOf(overlay, "session.rdb.dsn"),
				Table:  valueOf(overlay, "session.rdb.table"),
			}
			return nil
		},
		Scaffold: []configbind.ScaffoldField{
			{Key: "enabled", Kind: configbind.ScaffoldBool, Default: "false"},
			{Key: "backend", Kind: configbind.ScaffoldString, Default: "rdb", Help: "session storage backend"},
			{Key: "ttl", Kind: configbind.ScaffoldString, Default: "24h", Help: "absolute session lifetime"},
			{Key: "idle_timeout", Kind: configbind.ScaffoldString, Default: "0s", Help: "inactivity expiry; zero disables it"},
			{Key: "renewal_interval", Kind: configbind.ScaffoldString, Default: "0s", Help: "minimum interval between idle expiry renewals"},
			{Key: "cookie.name", Kind: configbind.ScaffoldString, Default: "pw_session"},
			{Key: "cookie.path", Kind: configbind.ScaffoldString, Default: "/"},
			{Key: "cookie.domain", Kind: configbind.ScaffoldString, Default: ""},
			{Key: "cookie.secure", Kind: configbind.ScaffoldBool, Default: "true", Help: "disable only for loopback development"},
			{Key: "cookie.http_only", Kind: configbind.ScaffoldBool, Default: "true"},
			{Key: "cookie.same_site", Kind: configbind.ScaffoldString, Default: "lax"},
			{Key: "rdb.source", Kind: configbind.ScaffoldString, Default: "middleware", Help: "middleware reuses middleware.rdb; dedicated opens rdb.dsn"},
			{Key: "rdb.dsn", Kind: configbind.ScaffoldString, Default: "", Help: "dedicated session database DSN"},
			{Key: "rdb.table", Kind: configbind.ScaffoldString, Default: "popcornwave_session"},
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
			"middleware.rdb.connect_timeout",
			"middleware.rdb.max_open_conns", "middleware.rdb.max_idle_conns",
			"middleware.rdb.conn_max_lifetime", "middleware.rdb.conn_max_idle_time",
		},
		Defaults: map[string]string{
			"middleware.recovery": "true", "middleware.request_id": "true",
			"middleware.access_log": "true", "middleware.compression": "false",
			"middleware.request_timeout": "0s",
			"middleware.rdb.enabled":     "false", "middleware.rdb.dsn": "",
			"middleware.rdb.connect_timeout": "5s",
			"middleware.rdb.max_open_conns":  "0", "middleware.rdb.max_idle_conns": "0",
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
