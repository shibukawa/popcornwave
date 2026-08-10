package pwfast

import (
	"fmt"
	"net"

	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// SecurityHeadersConfig and its parts are the shared leaf's, so a project
// configures one set of headers and both builds send it.
type (
	SecurityHeadersConfig = pwruntime.SecurityHeadersConfig
	HSTSConfig            = pwruntime.HSTSConfig
)

// DefaultSecurityHeaders returns the classic mode defaults.
func DefaultSecurityHeaders() SecurityHeadersConfig { return pwruntime.DefaultSecurityHeaders() }

// SecurityHeadersOption configures SecurityHeaders.
type SecurityHeadersOption func(*securityHeadersOptions)

type securityHeadersOptions struct {
	trustedProxies []*net.IPNet
}

// WithTrustedProxies accepts X-Forwarded-Proto from the listed networks when
// deciding whether a request arrived over HTTPS. Without it only a direct TLS
// connection counts as HTTPS.
func WithTrustedProxies(networks []*net.IPNet) SecurityHeadersOption {
	return func(o *securityHeadersOptions) { o.trustedProxies = networks }
}

// SecurityHeaders sets the policy headers on every response.
//
// The configuration is validated and reduced to a header set by the shared
// leaf, at construction, so a misconfiguration is an error before the port is
// bound and the two transports send the same headers rather than two
// computations that agree.
func SecurityHeaders(config SecurityHeadersConfig, option ...SecurityHeadersOption) (Middleware, error) {
	resolved, err := pwruntime.ResolveSecurityHeaders(config)
	if err != nil {
		return nil, err
	}
	options := securityHeadersOptions{}
	for _, apply := range option {
		apply(&options)
	}
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(r *fasthttp.RequestCtx) {
			for _, entry := range resolved.Always {
				r.Response.Header.Set(entry.Name, entry.Value)
			}
			if resolved.HSTS != "" && requestIsHTTPS(r, options.trustedProxies) {
				r.Response.Header.Set("Strict-Transport-Security", resolved.HSTS)
			}
			next(r)
		}
	}, nil
}

// requestIsHTTPS reports whether the client's own hop was HTTPS.
//
// A direct TLS connection is proof. A forwarded header is only evidence, and
// only from an address the deployment said it forwards through — anybody can
// send the header, so believing it from an untrusted peer would let a client
// turn HSTS on for a host it does not control.
//
// The address arrives already parsed on this transport, which is the only
// difference from the other half.
func requestIsHTTPS(r *fasthttp.RequestCtx, trusted []*net.IPNet) bool {
	if r.IsTLS() {
		return true
	}
	if !pwruntime.TrustedProxy(r.RemoteIP(), trusted) {
		return false
	}
	return pwruntime.ForwardedProtoIsHTTPS(string(r.Request.Header.Peek("X-Forwarded-Proto")))
}

// PanicHandler writes the response for a panic recovered by Recover.
type PanicHandler func(r *fasthttp.RequestCtx, err error)

// Recover converts a panic into a response written by handler. A nil handler
// logs the panic and writes a plain 500.
//
// It recovers more completely than the other half can, and the difference is
// the transport rather than the code: this one buffers a response until the
// handler returns, so whatever a failed handler had written is still discardable
// and the 500 is the only thing the client sees. On net/http a response that has
// already committed can only be stopped, not taken back.
//
// What it does not reach is a panic inside a body stream writer, because that
// callback runs after the handler has returned and this frame with it.
func Recover(handler PanicHandler) Middleware {
	if handler == nil {
		handler = writePanicStatus
	}
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(r *fasthttp.RequestCtx) {
			defer func() {
				if recovered := recover(); recovered != nil {
					handler(r, fmt.Errorf("panic: %v", recovered))
				}
			}()
			next(r)
		}
	}
}

func writePanicStatus(r *fasthttp.RequestCtx, err error) {
	pwruntime.ReadLogger(r).Log(r, pwruntime.LevelError, "recovered panic",
		pwruntime.String("error", err.Error()))
	// The partial body of a failed handler is discarded rather than sent under a
	// 500, which would be a response describing one thing and carrying another.
	r.Response.ResetBody()
	r.Error(fasthttp.StatusMessage(fasthttp.StatusInternalServerError), fasthttp.StatusInternalServerError)
}

// MaxRequestBody refuses a request whose body is larger than limit.
//
// A non-positive limit disables the check, which is the same reading the other
// half gives it.
//
// The check is on the declared length rather than on a wrapped reader, because
// this transport has already read the body by the time a handler runs: the
// server's own MaxRequestBodySize is what stops an oversized one from being read
// at all, and this frame is the per-chain expression of the same limit.
func MaxRequestBody(limit int64) Middleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		if limit <= 0 {
			return next
		}
		return func(r *fasthttp.RequestCtx) {
			if int64(r.Request.Header.ContentLength()) > limit || int64(len(r.Request.Body())) > limit {
				WriteProblem(r, PayloadTooLarge("request body is too large"))
				return
			}
			next(r)
		}
	}
}
