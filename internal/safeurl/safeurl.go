// Package safeurl decides whether a URL may be handed to a browser as a
// navigation target or written into a URL-bearing attribute.
//
// It exists because escaping is the wrong tool for the question. HTML escaping
// makes a value safe to sit inside markup; it says nothing about what the
// browser does when the value is followed. "javascript:alert(1)" contains none
// of & < > " ', so it survives every escaper unchanged and then executes. The
// standard library draws this line for the same reason, which is why
// html/template rewrites such a URL to #ZgotmplZ rather than escaping it.
//
// The rule here is an allowlist of schemes, not a denylist of dangerous ones. A
// denylist has to anticipate every scheme a browser will ever execute, and the
// set has grown before: javascript:, then data:, then vbscript:. An allowlist
// only has to name the ones a web application actually navigates to.
package safeurl

import "strings"

// navigableSchemes are the schemes a browser may be sent to. It matches what
// html/template permits, plus tel, which is ordinary in a page and cannot carry
// script.
//
// blob: and filesystem: are absent deliberately. Both can name a document that
// runs script in the page's own origin, so they belong to the same class as
// data: even though they read as inert.
var navigableSchemes = map[string]bool{
	"http":   true,
	"https":  true,
	"mailto": true,
	"tel":    true,
}

// Navigable reports whether value is a URL this framework will hand to a
// browser.
//
// A relative URL is always navigable: it cannot name a scheme, so it cannot
// name an executing one, and resolving it against the page can only produce the
// page's own scheme. That includes the protocol-relative form //host/path,
// which reaches another origin but still under the page's scheme. Whether
// reaching another origin is desirable is the caller's policy and not a
// property of the URL, so it is not decided here; what is decided here is that
// following the value cannot run script.
//
// An empty value is not navigable. Navigating to "" reloads the current page,
// which is never what a caller meant to ask for and hides the bug that produced
// the empty string.
func Navigable(value string) bool {
	scheme, rest, found := strings.Cut(value, ":")
	if !found {
		// No colon at all, so no scheme: an ordinary relative reference.
		return value != ""
	}
	// A colon that appears after a slash, a question mark, or a hash is part of
	// a path, query, or fragment rather than a scheme delimiter — "/a:b" and
	// "?x=a:b" are relative references. RFC 3986 settles this the same way: a
	// scheme cannot contain any of those characters.
	if strings.ContainsAny(scheme, "/?#") {
		return value != ""
	}
	if scheme == "" {
		// ":/path" has an empty scheme, which no browser resolves the way the
		// caller intended. Refuse rather than guess.
		return false
	}
	if !validScheme(scheme) {
		// A colon this early that is not a scheme is not a shape worth
		// admitting; the value is malformed either way.
		return false
	}
	if !navigableSchemes[strings.ToLower(scheme)] {
		return false
	}
	// mailto: and tel: carry an opaque target rather than a hierarchical path,
	// so an empty remainder is a link to nowhere.
	return rest != "" || scheme == "http" || scheme == "https"
}

// validScheme reports whether name has the syntax RFC 3986 gives a scheme.
//
// It matters because a value like "java\nscript:alert(1)" must not be treated
// as the relative reference its embedded newline would otherwise make it look
// like. Anything that is not a well-formed scheme is refused outright.
func validScheme(name string) bool {
	for index := range len(name) {
		c := name[index]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		case index > 0 && (c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'):
		default:
			return false
		}
	}
	// A scheme must begin with a letter, which an empty name does not.
	return name != ""
}
