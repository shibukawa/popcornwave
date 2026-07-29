package petitweb

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// ServerConfig controls the classic HTTP server and operational endpoints.
type ServerConfig struct {
	Address           string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxRequestBody    int64
	TrustedProxies    []string
	// Health, Readiness, and OpenAPI are the absolute paths their endpoints
	// serve, and an unset path serves nothing. There is no default path: an
	// application that answers on /healthz should say so where a reader of its
	// setup can see it, rather than inherit it from here.
	Health    string
	Readiness string
	OpenAPI   string
}

// DefaultServerConfig returns conservative production-oriented defaults.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Address:           ":8080",
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
		ShutdownTimeout:   10 * time.Second,
		MaxRequestBody:    8 << 20,
	}
}

// Validate checks startup invariants before a listener accepts requests.
func (c ServerConfig) Validate() error {
	var result error
	if c.Address == "" {
		result = errors.Join(result, errors.New("petitweb: server address is empty"))
	} else if _, _, err := net.SplitHostPort(c.Address); err != nil {
		result = errors.Join(result, fmt.Errorf("petitweb: invalid server address: %w", err))
	}
	for name, duration := range map[string]time.Duration{
		"read_header_timeout": c.ReadHeaderTimeout, "read_timeout": c.ReadTimeout,
		"write_timeout": c.WriteTimeout, "idle_timeout": c.IdleTimeout,
		"shutdown_timeout": c.ShutdownTimeout,
	} {
		if duration <= 0 {
			result = errors.Join(result, fmt.Errorf("petitweb: %s must be positive", name))
		}
	}
	if c.MaxRequestBody <= 0 {
		result = errors.Join(result, errors.New("petitweb: max_request_body must be positive"))
	}
	paths := make(map[string]string)
	for name, endpoint := range map[string]string{"health": c.Health, "readiness": c.Readiness, "openapi": c.OpenAPI} {
		if endpoint == "" {
			continue
		}
		if !validAbsolutePath(endpoint) {
			result = errors.Join(result, fmt.Errorf("petitweb: invalid %s path %q", name, endpoint))
		}
		if previous, exists := paths[endpoint]; exists {
			result = errors.Join(result, fmt.Errorf("petitweb: %s and %s endpoints use the same path %q", previous, name, endpoint))
		}
		paths[endpoint] = name
	}
	for _, proxy := range c.TrustedProxies {
		if net.ParseIP(proxy) == nil {
			if _, _, err := net.ParseCIDR(proxy); err != nil {
				result = errors.Join(result, fmt.Errorf("petitweb: invalid trusted proxy %q", proxy))
			}
		}
	}
	return result
}

func validAbsolutePath(path string) bool {
	return strings.HasPrefix(path, "/") && path != "" && !strings.ContainsAny(path, "\r\n?#")
}
