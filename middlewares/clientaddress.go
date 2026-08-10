package middlewares

import (
	"net"
	"net/http"
	"sync"

	"github.com/shibukawa/popcornwave/internal/requestorigin"
	"github.com/shibukawa/popcornwave/pwruntime"
)

// ResolveClientAddress records the caller's own address on the request context,
// reading X-Forwarded-For only from the declared proxy networks.
//
// It exists so that everything downstream counting or bounding one client —
// a rate limit bucket, a live subscription bound — reads one resolved value
// instead of each reaching for RemoteAddr, which behind a proxy is the proxy.
//
// With no networks declared it records the peer unchanged, which is the same
// answer the callers gave before this frame existed.
func ResolveClientAddress(trustedProxies []*net.IPNet) Middleware {
	proxies := requestorigin.FromNetworks(trustedProxies)
	var undeclared sync.Once
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if proxies.Empty() && forwardedBy(r) {
				warnUndeclaredProxy(&undeclared, r)
			}
			address := proxies.ClientAddress(r)
			next.ServeHTTP(w, r.WithContext(pwruntime.WithClientAddress(r.Context(), address)))
		})
	}
}

func forwardedBy(r *http.Request) bool {
	return r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Forwarded-Proto") != ""
}

// warnUndeclaredProxy reports a deployment that sits behind a proxy its
// configuration never named.
//
// Every consequence of that is silent otherwise: HSTS stops being emitted, the
// CSRF origin comparison reconstructs http for an https browser, and every
// rate limit and live bound counts the proxy instead of the caller. None of
// those produce a message of their own, which is what makes this one worth a
// line.
//
// It fires once per process. A per-request advisory is a log flood, and the
// condition is a property of the deployment rather than of the request.
func warnUndeclaredProxy(once *sync.Once, r *http.Request) {
	once.Do(func() {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		pwruntime.ReadLogger(r.Context()).Log(r.Context(), pwruntime.LevelWarn,
			"a forwarded header arrived from a peer server.trusted_proxies does not name, and was ignored",
			pwruntime.String("peer", host),
			pwruntime.String("setting", "server.trusted_proxies"),
			pwruntime.String("consequence", "the client address, the request scheme, and HSTS are resolved as if no proxy were present"))
	})
}
