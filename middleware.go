package petitweb

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RequestID validates or creates a request ID and exposes it through context.
func RequestID(header string, logger *slog.Logger) Middleware {
	if header == "" {
		header = "X-Request-ID"
	}
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(header)
			if !validRequestID(id) {
				id = newRequestID()
			}
			w.Header().Set(header, id)
			bound := logger.With("request_id", id)
			ctx := withRequestValues(r.Context(), requestValues{requestID: id, logger: bound})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func validRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if r < 0x21 || r > 0x7e {
			return false
		}
	}
	return true
}

func newRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "petitweb-request"
	}
	return hex.EncodeToString(bytes[:])
}

// Recover converts a panic into a safe negotiated error response.
func Recover(handler ErrorHandler) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					handler.WriteError(w, r, fmt.Errorf("panic: %v", recovered))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// MaxRequestBody limits downstream reads from the request body.
func MaxRequestBody(bytes int64) Middleware {
	if bytes <= 0 {
		panic("petitweb: request body limit must be positive")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, bytes)
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeadersConfig contains browser security response headers.
type SecurityHeadersConfig struct {
	ContentTypeOptions        bool
	FrameOptions              string
	ReferrerPolicy            string
	ContentSecurityPolicy     string
	ContentSecurityReportOnly string
	PermissionsPolicy         string
	HSTS                      HSTSConfig
}

// HSTSConfig controls Strict-Transport-Security on direct HTTPS requests.
type HSTSConfig struct {
	Enabled           bool
	MaxAge            time.Duration
	IncludeSubDomains bool
	Preload           bool
}

// DefaultSecurityHeaders returns the classic mode defaults.
func DefaultSecurityHeaders() SecurityHeadersConfig {
	return SecurityHeadersConfig{ContentTypeOptions: true, FrameOptions: "DENY", ReferrerPolicy: "strict-origin-when-cross-origin"}
}

// Validate rejects response splitting and unsupported fixed-value policies.
func (c SecurityHeadersConfig) Validate() error {
	for name, value := range map[string]string{
		"frame_options": c.FrameOptions, "referrer_policy": c.ReferrerPolicy,
		"content_security_policy":             c.ContentSecurityPolicy,
		"content_security_policy_report_only": c.ContentSecurityReportOnly,
		"permissions_policy":                  c.PermissionsPolicy,
	} {
		if !validHeaderValue(value) {
			return fmt.Errorf("petitweb: %s contains an invalid header value", name)
		}
	}
	frame := strings.ToUpper(c.FrameOptions)
	if frame != "" && frame != "OFF" && frame != "DENY" && frame != "SAMEORIGIN" {
		return fmt.Errorf("petitweb: unsupported frame_options %q", c.FrameOptions)
	}
	if c.ReferrerPolicy != "" {
		switch strings.ToLower(c.ReferrerPolicy) {
		case "no-referrer", "same-origin", "strict-origin", "strict-origin-when-cross-origin":
		default:
			return fmt.Errorf("petitweb: unsupported referrer_policy %q", c.ReferrerPolicy)
		}
	}
	if c.HSTS.Enabled {
		if c.HSTS.MaxAge <= 0 {
			return fmt.Errorf("petitweb: HSTS max age must be positive")
		}
		if c.HSTS.Preload && (!c.HSTS.IncludeSubDomains || c.HSTS.MaxAge < 365*24*time.Hour) {
			return fmt.Errorf("petitweb: HSTS preload requires includeSubDomains and a max age of at least one year")
		}
	}
	return nil
}

func validHeaderValue(value string) bool {
	for _, r := range value {
		if (r < 0x20 && r != '\t') || r == 0x7f {
			return false
		}
	}
	return true
}

// SecurityHeaders sets policy headers before downstream response commitment.
func SecurityHeaders(config SecurityHeadersConfig) (Middleware, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if config.ContentTypeOptions {
				w.Header().Set("X-Content-Type-Options", "nosniff")
			}
			if frame := strings.ToUpper(config.FrameOptions); frame != "" && frame != "OFF" {
				w.Header().Set("X-Frame-Options", frame)
			}
			if config.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", config.ReferrerPolicy)
			}
			if config.ContentSecurityPolicy != "" {
				w.Header().Set("Content-Security-Policy", config.ContentSecurityPolicy)
			}
			if config.ContentSecurityReportOnly != "" {
				w.Header().Set("Content-Security-Policy-Report-Only", config.ContentSecurityReportOnly)
			}
			if config.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", config.PermissionsPolicy)
			}
			if config.HSTS.Enabled && r.TLS != nil {
				value := "max-age=" + strconv.FormatInt(int64(config.HSTS.MaxAge/time.Second), 10)
				if config.HSTS.IncludeSubDomains {
					value += "; includeSubDomains"
				}
				if config.HSTS.Preload {
					value += "; preload"
				}
				w.Header().Set("Strict-Transport-Security", value)
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}
