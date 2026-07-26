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

func validateRuntimeConfig(server ServerConfig, security SecurityConfig, middleware MiddlewareConfig, observability ObservabilityConfig, auth AuthConfig) error {
	if err := validateServerConfig(server); err != nil {
		return err
	}
	if err := validateAuthConfig(auth); err != nil {
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
	return nil
}

// validateAuthConfig fails startup when a login could never succeed. An empty
// issuer or client credential is the common case: the project was scaffolded
// for OIDC but the provider values were never supplied, and the application
// would otherwise fail later at the first login attempt.
func validateAuthConfig(config AuthConfig) error {
	if !config.Enabled {
		return nil
	}
	switch config.Mode {
	case AuthModeOIDC, AuthModeOIDCPasskey, AuthModePasskey:
	default:
		return fmt.Errorf("auth.mode must be %q, %q, or %q", AuthModeOIDC, AuthModeOIDCPasskey, AuthModePasskey)
	}
	endpoints := map[string]string{
		"auth.login_path":    config.LoginPath,
		"auth.callback_path": config.CallbackPath,
		"auth.logout_path":   config.LogoutPath,
	}
	seen := map[string]string{}
	for key, value := range endpoints {
		if err := validateLocalPath(key, value); err != nil {
			return err
		}
		if previous, duplicate := seen[value]; duplicate {
			return fmt.Errorf("%s and %s must not share the path %q", previous, key, value)
		}
		seen[value] = key
	}
	for key, value := range map[string]string{
		"auth.post_login_redirect":  config.PostLoginRedirect,
		"auth.post_logout_redirect": config.PostLogoutRedirect,
	} {
		if err := validateLocalPath(key, value); err != nil {
			return err
		}
	}
	if !config.UsesOIDC() {
		return nil
	}
	var missing []string
	for _, field := range []struct {
		key         string
		environment string
		value       string
	}{
		{"auth.oidc.issuer", "AUTH_OIDC_ISSUER", config.OIDC.Issuer},
		{"auth.oidc.client_id", "AUTH_OIDC_CLIENT_ID", config.OIDC.ClientID},
		{"auth.oidc.client_secret", "AUTH_OIDC_CLIENT_SECRET", config.OIDC.ClientSecret},
	} {
		if strings.TrimSpace(field.value) == "" {
			missing = append(missing, fmt.Sprintf("%s (%s)", field.key, field.environment))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"auth.mode %q needs %s; run pw dev to use the development identity provider, or supply the values in config.%s.toml or the environment",
			config.Mode, strings.Join(missing, ", "), Env())
	}
	issuer, err := url.Parse(config.OIDC.Issuer)
	if err != nil || !issuer.IsAbs() || issuer.Host == "" || (issuer.Scheme != "http" && issuer.Scheme != "https") {
		return fmt.Errorf("auth.oidc.issuer must be an absolute http or https URL")
	}
	if config.OIDC.RedirectURL != "" {
		redirect, err := url.Parse(config.OIDC.RedirectURL)
		if err != nil || !redirect.IsAbs() || redirect.Host == "" {
			return fmt.Errorf("auth.oidc.redirect_url must be an absolute URL")
		}
	}
	return nil
}

// validateLocalPath keeps a configured redirect target on this origin, so a
// configuration mistake cannot turn an endpoint into an open redirect.
func validateLocalPath(key, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", key)
	}
	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return fmt.Errorf("%s must be an absolute path on this origin", key)
	}
	if strings.ContainsAny(value, " \t\r\n") || strings.Contains(value, "://") {
		return fmt.Errorf("%s must be an absolute path on this origin", key)
	}
	return nil
}

func validateRDBConfig(config RDBConfig) error {
	if !config.Enabled {
		return nil
	}
	if _, _, err := databaseTarget(config.DSN); err != nil {
		return err
	}
	if config.ConnectTimeout <= 0 {
		return fmt.Errorf("middleware.rdb.connect_timeout must be positive")
	}
	if config.MaxOpenConns < 0 || config.MaxIdleConns < 0 {
		return fmt.Errorf("middleware.rdb pool sizes must not be negative")
	}
	if config.MaxOpenConns > 0 && config.MaxIdleConns > config.MaxOpenConns {
		return fmt.Errorf("middleware.rdb.max_idle_conns must not exceed max_open_conns")
	}
	if config.ConnMaxLifetime < 0 || config.ConnMaxIdleTime < 0 {
		return fmt.Errorf("middleware.rdb connection durations must not be negative")
	}
	if config.DSN == "sqlite://:memory:" && config.MaxOpenConns != 1 {
		return fmt.Errorf("middleware.rdb.max_open_conns must be 1 for sqlite://:memory:")
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
	seen := map[string]string{}
	for name, endpoint := range map[string]EndpointConfig{
		"health": config.Health, "readiness": config.Readiness, "openapi": config.OpenAPI,
	} {
		if !endpoint.Enabled {
			continue
		}
		if err := validateEndpointPath("server."+name+".path", endpoint.Path); err != nil {
			return err
		}
		if previous, exists := seen[endpoint.Path]; exists {
			return fmt.Errorf("server.%s.path duplicates server.%s.path: %s", name, previous, endpoint.Path)
		}
		seen[endpoint.Path] = name
	}
	if config.Public.Enabled {
		mount, err := middlewares.NormalizePublicMount(config.Public.Mount)
		if err != nil {
			return err
		}
		for endpoint, name := range seen {
			if strings.HasPrefix(endpoint+"/", mount) || strings.HasPrefix(mount, endpoint+"/") {
				return fmt.Errorf("server.public.mount overlaps server.%s.path: %s", name, endpoint)
			}
		}
	}
	return nil
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
	for name, endpoint := range map[string]EndpointConfig{
		"health": config.Health, "readiness": config.Readiness, "openapi": config.OpenAPI,
	} {
		if !endpoint.Enabled {
			continue
		}
		request, err := http.NewRequest(http.MethodGet, "http://popcornwave.invalid"+endpoint.Path, nil)
		if err != nil {
			return fmt.Errorf("server.%s.path: %w", name, err)
		}
		_, pattern := resolver.Handler(request)
		if exactRoutePath(pattern) == endpoint.Path {
			return fmt.Errorf("server.%s.path collides with application route %q", name, pattern)
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
