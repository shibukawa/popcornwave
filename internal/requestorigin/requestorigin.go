// Package requestorigin answers one question for every caller that asks it:
// did this request come from an origin this deployment serves.
//
// It exists because the question was being answered twice. The CSRF middleware
// compared a whole origin, scheme included, while the authentication endpoints
// compared host names alone, which admits an http caller to an https
// deployment. Two answers to one question is one answer that drifts, so both
// callers now share this.
package requestorigin

import (
	"net/http"
	"net/url"
	"strings"
)

// Of reconstructs the origin of the request itself, as scheme and host.
//
// A deployment behind a proxy that terminates TLS sees no r.TLS and would
// reconstruct an http origin for an https browser. Nothing here reads a
// forwarded header to repair that, because a caller can assert one and the
// value is what the comparison is made against. Such a deployment names its
// own origin to Matches instead.
//
// It returns the empty string for a request carrying no Host, which never
// matches anything.
func Of(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		return ""
	}
	return scheme + "://" + strings.TrimSuffix(host, ":")
}

// Matches reports whether the request came from this deployment's own origin or
// one the caller named in trusted, which may be nil.
//
// Origin is preferred, because a browser sets it on exactly the state-changing
// requests this protects. A literal null Origin is not one: it is what an
// opaque origin sends, and treating it as absent would fall through to the
// weaker check below.
//
// Referer is the fallback for a proxy that stripped Origin, and it is read
// strictly. A missing one is a refusal rather than a pass, since treating
// absence as trust would make the whole check optional for anything able to
// omit a header.
func Matches(r *http.Request, trusted map[string]bool) bool {
	self := Of(r)
	if origin := r.Header.Get("Origin"); origin != "" && origin != "null" {
		return origin == self || trusted[origin]
	}
	referer := r.Header.Get("Referer")
	if referer == "" {
		return false
	}
	parsed, err := url.Parse(referer)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	origin := parsed.Scheme + "://" + parsed.Host
	return origin == self || trusted[origin]
}

// Set turns configured origin strings into the map Matches takes.
//
// Each value is normalized to scheme and host, so a trailing slash or a path
// someone pasted from a browser bar does not silently fail to match. A value
// that names no scheme or no host is dropped rather than stored, because it
// cannot match an origin and keeping it would suggest it could.
func Set(origins ...string) map[string]bool {
	trusted := make(map[string]bool, len(origins))
	for _, value := range origins {
		parsed, err := url.Parse(strings.TrimSpace(value))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			continue
		}
		trusted[parsed.Scheme+"://"+parsed.Host] = true
	}
	return trusted
}
