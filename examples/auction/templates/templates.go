package templates

import (
	"net/url"

	"github.com/shibukawa/popcornwave/pw"
)

// RuntimeScriptURL backs the external declaration in document.pw.html.
//
// The runtime it names applies the sections of a page that arrive after the
// rest of it, which is what a template declaring an `async` parameter needs.
// A page without one loads it and finds nothing to do.
//
// The template calls this rather than writing a literal path, because the URL
// carries a revision derived from the script's own bytes: an upgrade that
// changes the runtime changes the URL, and a literal would go on pointing at
// bytes the server no longer serves.
func RuntimeScriptURL() *url.URL { return &url.URL{Path: pw.RuntimeScriptURL()} }

// AssetURL backs the external declaration in document.pw.html.
//
// It takes the path of a file inside the served tree — "app.css", not
// "/public/app.css" — and returns the URL this build serves it under. A build
// gives that URL a revision derived from the file's own bytes, so a deployment
// answers it immutably: a browser holding it never asks again, and an edit
// changes the URL rather than the answer behind it.
//
// The development loop has no manifest and hands back the plain URL, which
// revalidates — which is what a loop wants, since the point of an edit there is
// to see it.
func AssetURL(name string) *url.URL { return &url.URL{Path: pw.PublicAssetURL(name)} }
