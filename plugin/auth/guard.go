package auth

import (
	"net/http"
	"net/url"

	"github.com/shibukawa/popcornwave/pw"
)

// guard requires an authenticated request on every protected path. Protection
// is opt-in: a path is protected when it matches at least one include pattern
// and no exclude pattern.
func (rt *runtime) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, ok := canonicalPath(r)
		if !ok {
			pw.WriteProblem(w, r, pw.BadRequest())
			return
		}
		if !rt.protected(path) || pw.Authenticated(r.Context()) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if rt.config.Protection.Unauthenticated == UnauthenticatedUnauthorized {
			pw.WriteProblem(w, r, pw.Unauthorized())
			return
		}
		http.Redirect(w, r, rt.loginURL(path), http.StatusSeeOther)
	})
}

// protected applies exclude-over-include precedence. The framework's own login,
// callback, and logout paths always stay reachable.
func (rt *runtime) protected(path string) bool {
	switch path {
	case rt.config.LoginPath, rt.config.CallbackPath, rt.config.LogoutPath:
		return false
	}
	if matchAny(rt.exclude, path) {
		return false
	}
	return matchAny(rt.include, path)
}

// loginURL sends the browser to the local login path, carrying only a validated
// local return path.
func (rt *runtime) loginURL(path string) string {
	target := rt.config.LoginPath
	if returnPath := localReturnPath(path); returnPath != "" {
		target += "?" + url.Values{"next": []string{returnPath}}.Encode()
	}
	return target
}
