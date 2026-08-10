package pwruntime

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type clientAddressKey struct{}

// WithClientAddress records the address of the caller rather than of the relay
// in front of it, resolved once per request against this deployment's declared
// proxies.
//
// It is recorded rather than recomputed because more than one consumer needs
// it — a rate limit bucket, a live subscription bound — and two resolutions of
// one question are one answer that drifts.
func WithClientAddress(ctx context.Context, address string) context.Context {
	if address == "" {
		return ctx
	}
	return context.WithValue(ctx, clientAddressKey{}, address)
}

// ClientAddress returns the resolved caller address for the request.
//
// It falls back to the request's peer for a chain that never resolved one,
// which is what a handler served outside the framework middleware stack sees.
func ClientAddress(ctx context.Context, r *http.Request) string {
	if ctx != nil {
		if address, ok := ctx.Value(clientAddressKey{}).(string); ok && address != "" {
			return address
		}
	}
	if r == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return strings.Trim(host, "[]")
}

// StoreClientAddress records the resolved caller on a request value that
// carries its own state, which is WithClientAddress for a transport that
// cannot derive a context.
//
// An empty address is not stored, the same as the deriving form: the absence of
// a resolved address and a resolution to the empty string would otherwise be
// indistinguishable to every reader.
func StoreClientAddress(store ValueStore, address string) {
	if address == "" {
		return
	}
	store.SetUserValue(clientAddressKey{}, address)
}
