package pw

import (
	"net/http"

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

// FormValue reads one submitted form field, on the same terms as QueryValue.
func FormValue(r *http.Request, key string) string {
	values, err := tinybind.ParseFormMap(r)
	if err != nil {
		return ""
	}
	return values[key]
}
