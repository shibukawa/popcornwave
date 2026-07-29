package pw

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/middlewares"
)

func validateRuntimeConfig(server ServerConfig, security SecurityConfig, middleware MiddlewareConfig, observability ObservabilityConfig) error {
	if err := validateServerConfig(server); err != nil {
		return err
	}
	if err := validateSecurityConfig(security); err != nil {
		return err
	}
	if middleware.RequestTimeout < 0 {
		return fmt.Errorf("middleware.request_timeout must not be negative")
	}
	if err := validateRDBConfig(middleware.RDB); err != nil {
		return err
	}
	switch strings.ToLower(observability.MinimumLevel) {
	case "trace", "debug", "info", "warn", "error", "off":
	default:
		return fmt.Errorf("observability.minimum_level must be trace, debug, info, warn, error, or off")
	}
	switch strings.ToLower(strings.TrimSpace(observability.BootLog)) {
	case "", BootLogAuto, BootLogTree, BootLogRecord, BootLogOff:
	default:
		return fmt.Errorf("observability.boot_log must be %s, %s, %s, or %s", BootLogAuto, BootLogTree, BootLogRecord, BootLogOff)
	}
	return validateQueryLogConfig(observability.Query)
}

func validateQueryLogConfig(config QueryLogConfig) error {
	for key, value := range map[string]string{
		"observability.query.enabled":     config.Enabled,
		"observability.query.bind_values": config.BindValues,
	} {
		if _, err := resolveQueryToggle(value, false); err != nil {
			return fmt.Errorf("%s %w", key, err)
		}
	}
	for key, value := range map[string]string{
		"observability.query.level":      config.Level,
		"observability.query.slow_level": config.SlowLevel,
	} {
		if _, err := parseQueryLevel(value); err != nil {
			return fmt.Errorf("%s %w", key, err)
		}
	}
	if config.SlowThreshold < 0 {
		return fmt.Errorf("observability.query.slow_threshold must not be negative")
	}
	// Zero means unset, so a hand-written configuration may omit a bound and
	// take the default. Only a negative bound is meaningless.
	if config.MaxSQLLength < 0 {
		return fmt.Errorf("observability.query.max_sql_length must not be negative")
	}
	if config.MaxValueLength < 0 {
		return fmt.Errorf("observability.query.max_value_length must not be negative")
	}
	return nil
}

func validateRDBConfig(config RDBConfig) error {
	if !config.Enabled {
		return nil
	}
	connections, err := resolveRDBConnections(config)
	if err != nil {
		return err
	}
	members := make(map[string]int, len(connections))
	drivers := make(map[string]string, len(connections))
	for index, connection := range connections {
		key := connectionKey(config, index)
		if err := validateGroupName(connection.Group, key); err != nil {
			return err
		}
		driver, _, err := databaseTarget(connection.DSN)
		if err != nil {
			return fmt.Errorf("%s.dsn: %w", key, err)
		}
		// One group is one logical database, so a mixed-driver group would make
		// dialect and savepoint support depend on which replica answered.
		if previous, seen := drivers[connection.Group]; seen && previous != driver {
			return fmt.Errorf("connection group %q mixes the %s and %s drivers", connection.Group, previous, driver)
		}
		drivers[connection.Group] = driver
		if err := validateConnectionPool(connection, key); err != nil {
			return err
		}
		members[connection.Group]++
	}
	for index, connection := range connections {
		if connection.DSN == "sqlite://:memory:" && members[connection.Group] > 1 {
			return fmt.Errorf("%s: sqlite://:memory: cannot share a group, because each such DSN is a separate database", connectionKey(config, index))
		}
	}
	if _, err := resolveDefaultGroup(config, connections); err != nil {
		return err
	}
	if _, err := resolveWriteGroup(config, connections); err != nil {
		return err
	}
	if _, err := resolveMigrationGroup(config, connections); err != nil {
		return err
	}
	return nil
}

// connectionKey names one connection the way it appears in the file, so an
// error points at the element the operator has to edit.
func connectionKey(config RDBConfig, index int) string {
	if len(config.Connections) == 0 {
		return "middleware.rdb"
	}
	return fmt.Sprintf("middleware.rdb.connections[%d]", index)
}

func validateGroupName(group, key string) error {
	if group == "" {
		return fmt.Errorf("%s.group must not be empty", key)
	}
	for _, r := range group {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return fmt.Errorf("%s.group %q must use lower-case letters, digits, underscore, or hyphen", key, group)
		}
	}
	return nil
}

func validateConnectionPool(connection RDBConnectionConfig, key string) error {
	if connection.ConnectTimeout <= 0 {
		return fmt.Errorf("%s.connect_timeout must be positive", key)
	}
	if connection.MaxOpenConns < 0 || connection.MaxIdleConns < 0 {
		return fmt.Errorf("%s pool sizes must not be negative", key)
	}
	if connection.MaxOpenConns > 0 && connection.MaxIdleConns > connection.MaxOpenConns {
		return fmt.Errorf("%s.max_idle_conns must not exceed max_open_conns", key)
	}
	if connection.ConnMaxLifetime < 0 || connection.ConnMaxIdleTime < 0 {
		return fmt.Errorf("%s connection durations must not be negative", key)
	}
	if connection.DSN == "sqlite://:memory:" && connection.MaxOpenConns != 1 {
		return fmt.Errorf("%s.max_open_conns must be 1 for sqlite://:memory:", key)
	}
	return nil
}

func validateServerConfig(config ServerConfig) error {
	if config.Port < 0 || config.Port > 65535 {
		return fmt.Errorf("server.port must be between 0 and 65535")
	}
	for key, value := range map[string]time.Duration{
		"read_header_timeout": config.ReadHeaderTimeout,
		"read_timeout":        config.ReadTimeout,
		"write_timeout":       config.WriteTimeout,
		"idle_timeout":        config.IdleTimeout,
	} {
		if value < 0 {
			return fmt.Errorf("server.%s must not be negative", key)
		}
	}
	if config.ShutdownTimeout <= 0 {
		return fmt.Errorf("server.shutdown_timeout must be positive")
	}
	if config.MaxRequestBody < 0 {
		return fmt.Errorf("server.max_request_body must not be negative")
	}
	if _, err := compileTrustedProxies(config.TrustedProxies); err != nil {
		return err
	}
	switch config.APIDoc {
	case "", APIDocScalar, APIDocSwagger:
	default:
		return fmt.Errorf("server.api_doc must be %q, %q, or empty", APIDocScalar, APIDocSwagger)
	}
	if config.APIDoc != "" && !config.OpenAPI.Enabled {
		return fmt.Errorf("server.api_doc requires server.openapi.enabled")
	}
	seen := map[string]string{}
	for key, endpoint := range operationalEndpointPaths(config) {
		if err := validateEndpointPath(key, endpoint); err != nil {
			return err
		}
		if previous, exists := seen[endpoint]; exists {
			return fmt.Errorf("%s duplicates %s: %s", key, previous, endpoint)
		}
		seen[endpoint] = key
	}
	if config.Public.Enabled {
		mount, err := middlewares.NormalizePublicMount(config.Public.Mount)
		if err != nil {
			return err
		}
		for endpoint, key := range seen {
			if strings.HasPrefix(endpoint+"/", mount) || strings.HasPrefix(mount, endpoint+"/") {
				return fmt.Errorf("server.public.mount overlaps %s: %s", key, endpoint)
			}
		}
	}
	return nil
}

// operationalEndpointPaths maps the configuration key of every enabled
// framework-owned endpoint to the path it serves.
func operationalEndpointPaths(config ServerConfig) map[string]string {
	paths := map[string]string{}
	for key, endpoint := range map[string]EndpointConfig{
		"server.health.path": config.Health, "server.readiness.path": config.Readiness,
		"server.openapi.path": config.OpenAPI,
	} {
		if endpoint.Enabled {
			paths[key] = endpoint.Path
		}
	}
	if config.APIDoc != "" {
		paths["server.api_doc_path"] = config.APIDocPath
	}
	return paths
}

func validateEndpointPath(key, value string) error {
	if value == "" || value[0] != '/' {
		return fmt.Errorf("%s must be an absolute path", key)
	}
	if path.Clean(value) != value {
		return fmt.Errorf("%s must be canonical", key)
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("%s is invalid", key)
	}
	if hasControl(value) {
		return fmt.Errorf("%s contains control characters", key)
	}
	return nil
}

func validateOperationalEndpointCollisions(handler http.Handler, config ServerConfig) error {
	resolver, ok := handler.(interface {
		Handler(*http.Request) (http.Handler, string)
	})
	if !ok {
		return nil
	}
	for key, endpoint := range operationalEndpointPaths(config) {
		request, err := http.NewRequest(http.MethodGet, "http://popcornwave.invalid"+endpoint, nil)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		_, pattern := resolver.Handler(request)
		if exactRoutePath(pattern) == endpoint {
			return fmt.Errorf("%s collides with application route %q", key, pattern)
		}
	}
	if config.Public.Enabled {
		mount, err := middlewares.NormalizePublicMount(config.Public.Mount)
		if err != nil {
			return err
		}
		for _, requestPath := range []string{strings.TrimSuffix(mount, "/"), mount, mount + "collision"} {
			request, err := http.NewRequest(http.MethodGet, "http://popcornwave.invalid"+requestPath, nil)
			if err != nil {
				return fmt.Errorf("server.public.mount: %w", err)
			}
			_, pattern := resolver.Handler(request)
			route := exactRoutePath(pattern)
			if route != "" && route != "/" && publicPathsOverlap(route, mount) {
				return fmt.Errorf("server.public.mount collides with application route %q", pattern)
			}
		}
	}
	return nil
}

func publicPathsOverlap(route, mount string) bool {
	route = strings.TrimSuffix(route, "{$}")
	route = strings.TrimSuffix(route, "...")
	route = strings.TrimSuffix(route, "/")
	mount = strings.TrimSuffix(mount, "/")
	return route == mount || strings.HasPrefix(route+"/", mount+"/") || strings.HasPrefix(mount+"/", route+"/")
}

func exactRoutePath(pattern string) string {
	if pattern == "" {
		return ""
	}
	fields := strings.Fields(pattern)
	if len(fields) > 0 {
		pattern = fields[len(fields)-1]
	}
	if index := strings.Index(pattern, "/"); index >= 0 {
		return pattern[index:]
	}
	return ""
}

func validateSecurityConfig(config SecurityConfig) error {
	headers := config.Headers
	switch strings.ToLower(headers.FrameOptions) {
	case "deny", "sameorigin", "off":
	default:
		return fmt.Errorf("security.headers.frame_options must be deny, sameorigin, or off")
	}
	switch strings.ToLower(headers.ReferrerPolicy) {
	case "no-referrer", "same-origin", "strict-origin", "strict-origin-when-cross-origin":
	default:
		return fmt.Errorf("security.headers.referrer_policy is invalid")
	}
	for key, value := range map[string]string{
		"content_security_policy":             headers.ContentSecurityPolicy,
		"content_security_policy_report_only": headers.ContentSecurityPolicyReportOnly,
		"permissions_policy":                  headers.PermissionsPolicy,
	} {
		if hasControl(value) {
			return fmt.Errorf("security.headers.%s contains control characters", key)
		}
	}
	if headers.HSTS.MaxAge < 0 {
		return fmt.Errorf("security.headers.hsts.max_age must not be negative")
	}
	if headers.HSTS.Enabled && headers.HSTS.MaxAge <= 0 {
		return fmt.Errorf("security.headers.hsts.max_age must be positive when HSTS is enabled")
	}
	if headers.HSTS.Preload && !headers.HSTS.IncludeSubdomains {
		return fmt.Errorf("security.headers.hsts.preload requires include_subdomains")
	}
	return nil
}

func hasControl(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func compileTrustedProxies(values []string) ([]*net.IPNet, error) {
	networks := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("server.trusted_proxies contains an empty value")
		}
		if ip := net.ParseIP(value); ip != nil {
			bits := 128
			if ip.To4() != nil {
				ip, bits = ip.To4(), 32
			}
			networks = append(networks, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return nil, fmt.Errorf("server.trusted_proxies %q: %w", value, err)
		}
		networks = append(networks, network)
	}
	return networks, nil
}
