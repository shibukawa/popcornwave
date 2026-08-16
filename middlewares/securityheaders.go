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
	CORSConfig            = pwruntime.CORSConfig
)

// DefaultCORS returns the shipped cross-origin defaults: off, and admitting the
// reads a bearer API needs once turned on.
func DefaultCORS() CORSConfig { return pwruntime.DefaultCORS() }

// DefaultContentSecurityPolicy is the policy a project gets without naming one.
const DefaultContentSecurityPolicy = pwruntime.DefaultContentSecurityPolicy

// DefaultSecurityHeaders returns the classic mode defaults.
func DefaultSecurityHeaders() SecurityHeadersConfig { return pwruntime.DefaultSecurityHeaders() }

type securityHeadersOptions struct {
	trustedProxies []*net.IPNet
	cors           pwruntime.CORSConfig
	csrfHeader     string
}

// SecurityHeadersOption configures SecurityHeaders.
type SecurityHeadersOption func(*securityHeadersOptions)

// WithTrustedProxies accepts X-Forwarded-Proto from the listed networks when
// deciding whether a request arrived over HTTPS. Without it only a direct TLS
// connection counts as HTTPS.
func WithTrustedProxies(networks []*net.IPNet) SecurityHeadersOption {
	return func(o *securityHeadersOptions) { o.trustedProxies = networks }
}

// WithCORS gives the frame the cross-origin policy to answer as well.
//
// It is one frame rather than two because both halves are browser policy
// resolved from configuration and written before commitment, and because both
// wanted the same position: the marking has to be on the response before any
// frame below writes a refusal, and the headers were owed to those refusals
// too. csrfHeader is admitted in a preflight while credentials are on.
func WithCORS(config CORSConfig, csrfHeader string) SecurityHeadersOption {
	return func(o *securityHeadersOptions) {
		o.cors = config
		o.csrfHeader = csrfHeader
	}
}

// SecurityHeaders sets policy headers before downstream response commitment,
// and answers the cross-origin policy beside them.
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
	// The header half is validated either way and sends nothing while it is
	// off. That mattered to nobody while Enabled was also what installed the
	// frame; it matters now that the cross-origin half can install it alone,
	// because a deployment that turned the headers off would otherwise get them
	// back by admitting an origin.
	if !config.Enabled {
		resolved = pwruntime.ResolvedSecurityHeaders{}
	}
	cors, err := pwruntime.ResolveCORS(options.cors, options.csrfHeader)
	if err != nil {
		return nil, err
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
			if cors.Enabled() {
				raw := ""
				if r.URL != nil {
					raw = r.URL.RawPath
				}
				path := ""
				if r.URL != nil {
					path = r.URL.Path
				}
				decision := cors.Decide(path, raw, r.Method,
					r.Header.Get("Origin"),
					r.Header.Get("Access-Control-Request-Method"),
					r.Header.Get("Access-Control-Request-Headers"))
				applyCORS(header, decision)
				cors.RecordCORSDecline(r.Context(), decision, path)
				if decision.Preflight {
					// Answered here and not below: a preflight carries no
					// cookie, no Authorization and no token, so the session,
					// the authentication and the guard would each read it as a
					// caller asking for something it may not have.
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

// applyCORS writes a decision onto a response.
//
// Vary is added rather than set, because compression and the client
// classification put their own values there and a Set would drop them.
func applyCORS(header http.Header, decision pwruntime.CORSDecision) {
	for _, entry := range decision.Headers {
		header.Set(entry.Name, entry.Value)
	}
	for _, value := range decision.Vary {
		header.Add("Vary", value)
	}
}
