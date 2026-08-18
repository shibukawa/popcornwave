package id_

import (
	"context"
	"net/http"

	"github.com/shibukawa/popcornweb/pw"
)

type renameRequest struct {
	Name string `json:"name" check:"required"`
}

type renameResponse struct {
	Name string `json:"name"`
}

// Rename is a server action: an exported handler in a route package, reachable
// at a generated address, owning its whole response.
//
// It reads a typed request, which only works because generation runs over the
// packages of a page tree. The binder it calls is generated from this file.
//
// It answers by caller, which is what owning the whole response is for. A script
// called this and is holding the answer, so it gets a value; anyone else gets
// the page again, because a gesture has a document to update and nowhere to put
// one. A handler with nothing to return asks neither question and writes one
// response for everybody.
func Rename(w http.ResponseWriter, r *http.Request) {
	request, err := pw.Parse[renameRequest](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	if pw.WantsValue(r) {
		pw.WriteAPI(w, r, renameResponse{Name: request.Name})
		return
	}
	// A fixed destination rather than the page this came from, because it cannot
	// be reconstructed here: a bare button posts to the direct endpoint, whose
	// address is a constant carrying none of the route's path parameters. A
	// handler that needs to send somebody back to their own page belongs on a
	// form, which posts to the page URL.
	pw.RedirectSeeOther(w, r, "/")
}

// Retire is the form half of the same surface: a handler a form names rather
// than a button, so generation registers POST on the page's own pattern beside
// its GET and the lowered form reaches it with no browser runtime.
//
// It writes nothing, which is what makes the generated dispatcher answer 303
// back to the page. That is the post-redirect-get default, and a handler that
// wrote a status, a header, or a body would keep exactly that response instead.
func Retire(w http.ResponseWriter, r *http.Request) {}

// Profile is the typed shape of the same surface: an ordinary Go function of
// its own signature, admitted by the declaration above it rather than by
// looking like a handler.
//
// It is unexported, which the raw shape could not be — a declaration is what
// publishes it, so the export rule stops meaning anything here. A script calls
// it as actions.profile, its argument arrives from the call's payload by name,
// and its value comes back encoded rather than as markup.
//
// The leading context is the request's, which is what a function reading the
// database handle or the signed-in reader needs. Both live there.
var _ = pw.ServerAction(profile)

type Profile struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

func profile(ctx context.Context, id string) (Profile, error) {
	_ = ctx
	return Profile{Id: id, Name: "user " + id}, nil
}
