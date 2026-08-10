package middlewares

import (
	"net"
	"net/http"

	"github.com/shibukawa/popcornwave/internal/requestorigin"
	"github.com/shibukawa/popcornwave/pwruntime"
)

// The configuration, its defaults and its validation are the shared leaf's.
// They are aliased rather than converted, so there is one declaration and one
// set of struct tags for the configuration binder to read, and nothing that can
// drift between the two transports.
type (
	SecurityHeadersConfig = pwruntime.SecurityHeadersConfig
	HSTSConfig            = pwruntime.HSTSConfig
)

// DefaultContentSecurityPolicy is the policy a project gets without naming one.
const DefaultContentSecurityPolicy = pwruntime.DefaultContentSecurityPolicy

// DefaultSecurityHeaders returns the classic mode defaults.
func DefaultSecurityHeaders() SecurityHeadersConfig { return pwruntime.DefaultSecurityHeaders() }

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
//
// The header set is resolved once, by the shared leaf, so a misconfiguration is
// an error before the port is bound and both transports send the same headers
// rather than two computations that agree. The scheme question is answered by
// internal/requestorigin, which is where every caller that asks it answers it.
func SecurityHeaders(config SecurityHeadersConfig, option ...SecurityHeadersOption) (Middleware, error) {
	resolved, err := pwruntime.ResolveSecurityHeaders(config)
	if err != nil {
		return nil, err
	}
	options := securityHeadersOptions{}
	for _, apply := range option {
		apply(&options)
	}
	proxies := requestorigin.FromNetworks(options.trustedProxies)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := w.Header()
			for _, entry := range resolved.Always {
				header.Set(entry.Name, entry.Value)
			}
			if resolved.HSTS != "" && proxies.IsHTTPS(r) {
				header.Set("Strict-Transport-Security", resolved.HSTS)
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}
