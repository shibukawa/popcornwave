package templates

import (
	"net/url"

	"github.com/shibukawa/popcornwave/pwbrowser"
)

// RuntimeScriptURL backs the external declaration in document.pw.html.
//
// The template names the runtime through this call rather than as a literal
// path, because the URL carries a revision derived from the script itself: an
// upgrade that changes the runtime changes the URL, and a template holding a
// literal would go on pointing at bytes the server no longer serves.
//
// A url attribute takes a url.URL rather than a string, so a path can never be
// assembled out of unvalidated text by accident.
func RuntimeScriptURL() *url.URL { return &url.URL{Path: pwbrowser.RuntimeScriptURL()} }
