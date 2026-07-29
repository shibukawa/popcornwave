package middlewares

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SecurityHeadersConfig contains browser security response headers.
//
// Enabled records whether a runtime selects this middleware at all; SecurityHeaders
// itself always applies the configured headers.
type SecurityHeadersConfig struct {
	Enabled                         bool       `default:"true"`
	ContentTypeOptions              bool       `default:"true" dependon:".enabled"`
	FrameOptions                    string     `default:"deny" dependon:".enabled"`
	ReferrerPolicy                  string     `default:"strict-origin-when-cross-origin" dependon:".enabled"`
	ContentSecurityPolicy           string     `env:"-" dependon:".enabled"`
	ContentSecurityPolicyReportOnly string     `env:"-" dependon:".enabled"`
	PermissionsPolicy               string     `env:"-" dependon:".enabled"`
	HSTS                            HSTSConfig `dependon:".enabled"`
}

// HSTSConfig controls Strict-Transport-Security on verified HTTPS requests.
// The dependon on the HSTS field itself carries the headers switch down over
// this whole block, so nothing here names an absolute key.
type HSTSConfig struct {
	Enabled           bool          `default:"false"`
	MaxAge            time.Duration `default:"0s" dependon:".enabled"`
	IncludeSubdomains bool          `default:"false" dependon:".enabled"`
	Preload           bool          `default:"false" dependon:".enabled"`
}

// DefaultSecurityHeaders returns the classic mode defaults.
func DefaultSecurityHeaders() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		Enabled:            true,
		ContentTypeOptions: true,
		FrameOptions:       "DENY",
		ReferrerPolicy:     "strict-origin-when-cross-origin",
	}
}

// Validate rejects response splitting and unsupported fixed-value policies.
func (c SecurityHeadersConfig) Validate() error {
	for name, value := range map[string]string{
		"frame_options": c.FrameOptions, "referrer_policy": c.ReferrerPolicy,
		"content_security_policy":             c.ContentSecurityPolicy,
		"content_security_policy_report_only": c.ContentSecurityPolicyReportOnly,
		"permissions_policy":                  c.PermissionsPolicy,
	} {
		if !validHeaderValue(value) {
			return fmt.Errorf("popcornwave: %s contains an invalid header value", name)
		}
	}
	frame := strings.ToUpper(c.FrameOptions)
	if frame != "" && frame != "OFF" && frame != "DENY" && frame != "SAMEORIGIN" {
		return fmt.Errorf("popcornwave: unsupported frame_options %q", c.FrameOptions)
	}
	if c.ReferrerPolicy != "" {
		switch strings.ToLower(c.ReferrerPolicy) {
		case "no-referrer", "same-origin", "strict-origin", "strict-origin-when-cross-origin":
		default:
			return fmt.Errorf("popcornwave: unsupported referrer_policy %q", c.ReferrerPolicy)
		}
	}
	if c.HSTS.Enabled {
		if c.HSTS.MaxAge <= 0 {
			return fmt.Errorf("popcornwave: HSTS max age must be positive")
		}
		if c.HSTS.Preload && (!c.HSTS.IncludeSubdomains || c.HSTS.MaxAge < 365*24*time.Hour) {
			return fmt.Errorf("popcornwave: HSTS preload requires include_subdomains and a max age of at least one year")
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

type securityHeadersOptions struct {
	trustedProxies []*net.IPNet
}

// SecurityHeadersOption configures SecurityHeaders.
type SecurityHeadersOption func(*securityHeadersOptions)

// WithTrustedProxies accepts X-Forwarded-Proto from the listed networks when
// deciding whether a request arrived over HTTPS. Without it only a direct TLS
// connection counts as HTTPS.
func WithTrustedProxies(networks []*net.IPNet) SecurityHeadersOption {
	return func(o *securityHeadersOptions) { o.trustedProxies = networks }
}

// SecurityHeaders sets policy headers before downstream response commitment.
func SecurityHeaders(config SecurityHeadersConfig, option ...SecurityHeadersOption) (Middleware, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	options := securityHeadersOptions{}
	for _, apply := range option {
		apply(&options)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := w.Header()
			if config.ContentTypeOptions {
				header.Set("X-Content-Type-Options", "nosniff")
			}
			if frame := strings.ToUpper(config.FrameOptions); frame == "DENY" || frame == "SAMEORIGIN" {
				header.Set("X-Frame-Options", frame)
			}
			setOptionalHeader(header, "Referrer-Policy", config.ReferrerPolicy)
			setOptionalHeader(header, "Content-Security-Policy", config.ContentSecurityPolicy)
			setOptionalHeader(header, "Content-Security-Policy-Report-Only", config.ContentSecurityPolicyReportOnly)
			setOptionalHeader(header, "Permissions-Policy", config.PermissionsPolicy)
			if config.HSTS.Enabled && requestIsHTTPS(r, options.trustedProxies) {
				value := "max-age=" + strconv.FormatInt(int64(config.HSTS.MaxAge/time.Second), 10)
				if config.HSTS.IncludeSubdomains {
					value += "; includeSubDomains"
				}
				if config.HSTS.Preload {
					value += "; preload"
				}
				header.Set("Strict-Transport-Security", value)
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

func setOptionalHeader(header http.Header, name, value string) {
	if value != "" {
		header.Set(name, value)
	}
}

func requestIsHTTPS(r *http.Request, trusted []*net.IPNet) bool {
	if r.TLS != nil {
		return true
	}
	if !trustedRemote(r.RemoteAddr, trusted) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]), "https")
}

func trustedRemote(remote string, trusted []*net.IPNet) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil {
		return false
	}
	for _, network := range trusted {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
