package pw

import (
	"net/http"
	"net/url"

	"github.com/shibukawa/popcornwave/internal/safeurl"
	tinybind "github.com/shibukawa/tinybind-go"
)

// Redirect sends the browser to another location.
//
// It exists rather than leaving handlers on http.Redirect for two reasons, and
// only one of them is that a stdlib helper taking the writer and the request is
// a call no second transport can follow.
//
// The other is the branch. An update request is a fetch, so a 303 is followed
// by the fetch and its target applied as a region set for the wrong page; the
// answer there is the navigate directive instead. That branch belongs in one
// place rather than in every action handler that redirects.
//
// The target is refused unless a browser can follow it without running script.
// A redirect target is commonly a return path taken from the request, and the
// update runtime hands it to location.assign, which executes a javascript: URL
// rather than navigating to it. Refusing here means an application cannot turn
// its own redirect into script execution by forwarding a parameter it did not
// check — and the check is on both branches, because a handler should not have
// to know which one it took.
func Redirect(w http.ResponseWriter, r *http.Request, url string, status int) {
	if !safeurl.Navigable(url) {
		WriteProblem(w, r, InternalServerError(errUnsafeNavigation))
		return
	}
	if WantsUpdate(r) {
		WriteUpdateNavigate(w, r, url)
		return
	}
	http.Redirect(w, r, url, status)
}

// RedirectSeeOther is the form an action handler wants: 303, so a reload does
// not repost what the handler just applied.
func RedirectSeeOther(w http.ResponseWriter, r *http.Request, url string) {
	Redirect(w, r, url, http.StatusSeeOther)
}

// QueryValue reads one query parameter, and reports whether it was present.
//
// A handler binding its whole input takes api:request-binding instead. This is
// for the one-value case, where declaring a type costs more than it explains —
// and it exists so that case does not have to reach into the request itself,
// which is the read no second transport can follow.
func QueryValue(r *http.Request, key string) (string, bool) {
	return tinybind.QueryValue(r, key)
}

// Queries is the request's parsed query string, read once so that a decoder
// binding several parameters parses it once rather than per parameter.
//
// It is a function, like PathValue and for the same reason: the other transport
// parses a query into a type of its own, and a generated decoder that reached
// into the request itself would compile against only one of them. Pair it with
// [QueryLookup]; a handler wanting a single value takes [QueryValue] instead.
func Queries(r *http.Request) url.Values {
	return tinybind.Queries(r)
}

// QueryLookup reads one parameter out of what Queries returned, and reports
// whether it was present. An absent parameter and an empty one are different
// answers, which is what a decoder needs to tell a missing value from a blank.
func QueryLookup(q url.Values, key string) (string, bool) {
	return tinybind.QueryLookup(q, key)
}

// PathValue reads one route segment, and is empty for a segment the pattern
// does not name.
//
// It is here rather than left to the request's own method for the reason
// QueryValue is: a second transport has no *http.Request to call a method on,
// and route values reach it by a different route entirely. A generated route
// decoder calls this, so the same decoder compiles against either.
func PathValue(r *http.Request, key string) string {
	return tinybind.PathValue(r, key)
}

// FormValue reads one submitted form field, on the same terms as QueryValue.
func FormValue(r *http.Request, key string) string {
	values, err := tinybind.ParseFormMap(r)
	if err != nil {
		return ""
	}
	return values[key]
}
