package pwruntime

import (
	"sync/atomic"

	"github.com/shibukawa/popcornweb/internal/requestorigin"
)

// A WebSocket handshake is a GET, so it passes the CSRF frame untouched, and it
// carries the session cookie like any other request. An upgrade admitted from
// another site is therefore cross-site request forgery with a connection left
// open behind it, and the module's own default — comparing the Origin's host
// against the Host header — cannot see the scheme, a forwarding header, or the
// origins this deployment declared acceptable.
//
// This is where that judgement is made instead, once, for both transports. Each
// half reads the four facts below its own way and calls in, which is the shape
// requestorigin already uses for the scheme and the client address.

// SocketHandshake is what an upgrade request tells the origin check.
type SocketHandshake struct {
	// TLS reports a direct TLS connection, which outranks any header.
	TLS bool
	// Host is the host the request was addressed to.
	Host string
	// RemoteAddress is the peer, which decides whether the forwarded header
	// below is evidence or noise.
	RemoteAddress string
	// ForwardedProto is X-Forwarded-Proto, read only from a declared proxy.
	ForwardedProto string
}

var socketOriginPolicy atomic.Pointer[func(origin, host string) bool]

// SetSocketOriginPolicy records the check an application installed with the
// socket defaults. Both runtimes call it, so a policy installed once covers
// both, and passing nil restores this framework's own resolution.
func SetSocketOriginPolicy(check func(origin, host string) bool) {
	if check == nil {
		socketOriginPolicy.Store(nil)
		return
	}
	socketOriginPolicy.Store(&check)
}

// SocketOriginCheck returns the check one handshake is judged by, or nil when
// the module's own default should stand.
//
// The resolution runs per handshake rather than per message, which is once for
// the life of a connection, so it compiles the declared proxies and the trusted
// origin set there rather than caching a value a republished configuration
// would leave stale.
func SocketOriginCheck(handshake SocketHandshake, development bool) func(origin, host string) bool {
	if installed := socketOriginPolicy.Load(); installed != nil {
		return *installed
	}
	settings, published := ResolvedChainSettings()
	if !published {
		// Nothing read a configuration, so there are no declared proxies and no
		// declared origins to judge against — a handler test, or a mux built
		// without a parse. Anywhere a deployment can actually be, a parse has
		// published, so refusing here costs a served application nothing.
		if development {
			return nil
		}
		return admitNonBrowserOnly
	}
	proxies, err := requestorigin.Compile(settings.TrustedProxies)
	if err != nil {
		// A value this deployment could not compile is one nobody declared, and
		// trusting nothing is what the zero value already means. Reporting it
		// belongs to the parse, which refuses to start on the same value.
		proxies = requestorigin.Proxies{}
	}
	self := proxies.OriginOf(handshake.Host, proxies.SchemeOf(
		handshake.TLS, handshake.RemoteAddress, handshake.ForwardedProto))
	trusted := requestorigin.Set(settings.CSRF.TrustedOrigins...)
	return func(origin, _ string) bool {
		if origin == "" {
			return true
		}
		// The host the upgrader offers is the raw header; the one compared here
		// is the resolved origin, which is the whole point of resolving it. The
		// referer argument is empty because a handshake has no navigation
		// behind it: the fallback that exists for a stripped Origin has nothing
		// to read, and leaving it empty is what keeps a literal "null" origin —
		// an opaque one, and therefore a browser — refused.
		return requestorigin.MatchesOrigin(self, origin, "", trusted)
	}
}

// admitNonBrowserOnly refuses every origin a browser could send and admits a
// request carrying none.
//
// RFC 6455 requires a browser client to send Origin and permits only a
// non-browser client to omit it, so an absent header cannot be a page on
// another site. Refusing it instead — which is what the CSRF rule does, because
// a form post can reach the server with Origin stripped and Referer intact —
// would refuse every command-line and service client on a deployment that has
// no cross-site exposure at all.
func admitNonBrowserOnly(origin, _ string) bool { return origin == "" }
