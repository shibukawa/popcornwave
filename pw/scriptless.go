package pw

import (
	"net/http"

	"github.com/shibukawa/popcornweb/session"
	"github.com/shibukawa/tinybind-go/htmlbind"
)

// Detection of a browser that runs no script, so an async page reaches it as the
// settled document rather than as fallbacks nothing arrives to replace.
//
// Such a browser is not a crawler, so isBotRequest never sees it: it sends an
// ordinary User-Agent, takes the streaming branch, and keeps every fallback
// because the boundary runtime is what replaces them. A script cannot ask it
// anything either — that is the defining property of the client. The one HTML
// feature that fires precisely when scripting is off is noscript, so that is
// what asks.
//
// The block redirects to this same page under a marker parameter, and that
// request renders buffered. The reader therefore reaches the page they asked
// for, complete, one round trip later and at the same path: what is discarded is
// the first request, never the first view.

// scriptlessMarkerParam marks a request the noscript block redirected.
//
// It is what makes the mechanism correct on its own. The marked request selects
// the buffered branch from the parameter rather than from the cookie, so a
// client that refuses cookies terminates here instead of being sent round again.
const scriptlessMarkerParam = "_pw_nojs"

// scriptlessCookieName remembers the answer so only the first page of a visit
// pays the extra round trip. It is an optimization and nothing depends on it:
// with cookies refused every page costs two requests and every page is correct.
const scriptlessCookieName = "pw_nojs"

// scriptlessCookieMaxAge expires the answer on its own, because it records a
// rendering preference rather than an identity: a reader who turns scripting
// back on stops paying for this without having to clear anything.
const scriptlessCookieMaxAge = 30 * 24 * 60 * 60

// scriptlessSafeMethod reports whether a redirect may be aimed at this request.
//
// A meta refresh re-issues a GET, so emitting the block on a non-GET response
// would discard the validation errors or the receipt that response just
// rendered. The marker is still honored on any method; only asking is bounded.
func scriptlessSafeMethod(r *http.Request) bool {
	return r.Method == http.MethodGet || r.Method == http.MethodHead
}

func scriptlessMarkedURL(r *http.Request) bool {
	return r.URL.Query().Has(scriptlessMarkerParam)
}

func scriptlessCookieSet(r *http.Request) bool {
	cookie, err := r.Cookie(scriptlessCookieName)
	return err == nil && cookie.Value == "1"
}

// resolveScriptless decides what this request already knows about the client.
//
// It runs only where the response would otherwise stream, so a page with no
// await block, a classified bot, and a deployment with streaming off all reach
// none of it.
//
// buffered says this client waits for every boundary. handled says the response
// is already written and the caller must return.
func resolveScriptless(w http.ResponseWriter, r *http.Request) (buffered, handled bool) {
	marked := scriptlessMarkedURL(r)
	known := scriptlessCookieSet(r)
	switch {
	case marked && known && scriptlessSafeMethod(r):
		// A bookmarked marker URL, or a later visit that already carries the
		// answer. Sending the reader to the clean URL keeps the parameter from
		// outliving the one page that needed it, and the cookie carries the
		// decision from there.
		redirectWithoutScriptlessMarker(w, r)
		return false, true
	case marked:
		markScriptless(w, r)
		return true, false
	case known:
		return true, false
	}
	return false, false
}

// scriptlessProbeHead is the block that asks.
//
// It reaches the document through the head contribution channel, so no
// application template carries it, no shell edit can drop it, and it appears
// only on the responses that would otherwise be wrong.
func scriptlessProbeHead(r *http.Request) htmlbind.HeadNode {
	return htmlbind.HeadNoScript(
		htmlbind.HeadMeta(
			htmlbind.HeadAttr{Name: "http-equiv", Value: "refresh"},
			// Zero delay: an immediate redirect is exempt from the accessibility
			// bound on timed refreshes, and any visible delay would show the
			// reader exactly the fallbacks this exists to replace.
			htmlbind.HeadAttr{Name: "content", Value: "0; url=" + scriptlessTarget(r, true)},
		),
	)
}

func redirectWithoutScriptlessMarker(w http.ResponseWriter, r *http.Request) {
	// See Other rather than a permanent code: the marker means something about
	// this client at this moment, and a cache that remembered the redirect would
	// apply one reader's answer to the next.
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, scriptlessTarget(r, false), http.StatusSeeOther)
}

// scriptlessTarget renders this same request's URL with the marker added or
// removed, as a path and query only.
//
// Scheme and host are dropped deliberately. The target is this same page, so
// they are the browser's own, and naming them would send a reader behind a
// TLS-terminating proxy to the scheme the application saw rather than the one
// they are on.
func scriptlessTarget(r *http.Request, marked bool) string {
	target := *r.URL
	query := target.Query()
	if marked {
		query.Set(scriptlessMarkerParam, "1")
	} else {
		query.Del(scriptlessMarkerParam)
	}
	target.RawQuery = query.Encode()
	target.Scheme, target.Host, target.User, target.Fragment = "", "", nil, ""
	return target.RequestURI()
}

// markScriptless records the answer for the rest of the visit.
//
// The cookie follows the session cookie's own policy, so one deployment decision
// covers both rather than two that can disagree — the same reasoning setupCSRF
// applies to the anonymous token cookie.
func markScriptless(w http.ResponseWriter, r *http.Request) {
	sessionConfig := Config[SessionConfig](requestContext(r))
	sameSite, err := session.ParseSameSite(sessionConfig.Cookie.SameSite)
	if err != nil {
		// An unparseable policy is a startup problem reported elsewhere. Here it
		// must not cost the reader their page, and Lax is what a navigation
		// needs.
		sameSite = http.SameSiteLaxMode
	}
	if sameSite == http.SameSiteStrictMode {
		// Strict would withhold the answer from a reader arriving by a link from
		// another site, so their first page there would pay the round trip
		// again. Nothing here is a capability, so Lax costs nothing to relax to.
		sameSite = http.SameSiteLaxMode
	}
	path := sessionConfig.Cookie.Path
	if path == "" {
		path = "/"
	}
	http.SetCookie(w, &http.Cookie{
		Name:   scriptlessCookieName,
		Value:  "1",
		Path:   path,
		Domain: sessionConfig.Cookie.Domain,
		Secure: sessionConfig.Cookie.Secure,
		// No script may read it and no script would know what to do with it.
		HttpOnly: true,
		SameSite: sameSite,
		MaxAge:   scriptlessCookieMaxAge,
	})
}
