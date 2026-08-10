package middlewares

import (
	"net"
	"net/http"
	"strings"

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
func SecurityHeaders(config SecurityHeadersConfig, option ...SecurityHeadersOption) (Middleware, error) {
	resolved, err := pwruntime.ResolveSecurityHeaders(config)
	if err != nil {
		return nil, err
	}
	options := securityHeadersOptions{}
	for _, apply := range option {
		apply(&options)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := w.Header()
			for _, entry := range resolved.Always {
				header.Set(entry.Name, entry.Value)
			}
			if resolved.HSTS != "" && requestIsHTTPS(r, options.trustedProxies) {
				header.Set("Strict-Transport-Security", resolved.HSTS)
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

// requestIsHTTPS reports whether the client's own hop was HTTPS.
//
// A direct TLS connection is proof. A forwarded header is only evidence, and
// only from an address the deployment said it forwards through — anybody can
// send the header, so believing it from an untrusted peer would let a client
// turn HSTS on for a host it does not control.
func requestIsHTTPS(r *http.Request, trusted []*net.IPNet) bool {
	if r.TLS != nil {
		return true
	}
	if !pwruntime.TrustedProxy(remoteIP(r.RemoteAddr), trusted) {
		return false
	}
	return pwruntime.ForwardedProtoIsHTTPS(r.Header.Get("X-Forwarded-Proto"))
}

// remoteIP parses the address net/http keeps as host:port.
func remoteIP(remote string) net.IP {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	return net.ParseIP(strings.Trim(host, "[]"))
}
