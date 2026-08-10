// Package pathpattern is the path-matching grammar the framework's
// path-scoped policies share.
//
// policy:csrf-protection is specified as using the same segment grammar and
// exclude precedence as policy:authenticated-path-protection, so the two read
// one implementation rather than two that can drift.
package pathpattern

import (
	"fmt"
	"net/http"
	"strings"
)

// pattern is one compiled path-protection pattern.
//
// The grammar is deliberately small: a literal segment matches itself, a "*"
// segment matches exactly one non-empty segment, and a trailing "**" segment
// matches the prefix itself and every descendant. Regular expressions, query
// matching, and mid-segment wildcards are not part of it.
type Pattern struct {
	source   []string
	subtree  bool
	original string
}

// Compile compiles a pattern list, rejecting the first malformed entry.
func Compile(values []string) ([]Pattern, error) {
	if len(values) == 0 {
		return nil, nil
	}
	compiled := make([]Pattern, 0, len(values))
	for _, value := range values {
		p, err := compileOne(value)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, p)
	}
	return compiled, nil
}

func compileOne(value string) (Pattern, error) {
	if !strings.HasPrefix(value, "/") {
		return Pattern{}, fmt.Errorf("pattern %q must start with a slash", value)
	}
	if strings.Contains(value, "?") || strings.Contains(value, "#") {
		return Pattern{}, fmt.Errorf("pattern %q must not contain a query or fragment", value)
	}
	trimmed := strings.TrimPrefix(value, "/")
	var segments []string
	if trimmed != "" {
		segments = strings.Split(trimmed, "/")
	}
	subtree := false
	for index, segment := range segments {
		switch {
		case segment == "**":
			if index != len(segments)-1 {
				return Pattern{}, fmt.Errorf("pattern %q may use ** only as the last segment", value)
			}
			subtree = true
		case segment == "*":
		case segment == "", segment == ".", segment == "..":
			return Pattern{}, fmt.Errorf("pattern %q has an empty or dot segment", value)
		case strings.Contains(segment, "*"):
			return Pattern{}, fmt.Errorf("pattern %q may not use a mid-segment wildcard", value)
		}
	}
	if subtree {
		segments = segments[:len(segments)-1]
	}
	return Pattern{source: segments, subtree: subtree, original: value}, nil
}

// Match reports whether path matches. path must already be canonical.
//
// The path is walked segment by segment rather than split into a slice: a
// policy runs this over every include and exclude pattern on every unsafe
// request, so the match must not allocate.
func (p Pattern) Match(path string) bool {
	remainder := strings.TrimPrefix(path, "/")
	exhausted := remainder == ""
	if !exhausted {
		// A trailing slash names the same thing as its absence for the purpose
		// of a policy decision, so it is dropped rather than counted. Counting
		// it made /admin/delete/ a different length from the pattern
		// /admin/delete, so the pattern did not match and an include failed
		// open — the request went through unprotected instead of being refused.
		remainder = strings.TrimSuffix(remainder, "/")
	}
	for _, expected := range p.source {
		if exhausted {
			return false
		}
		var actual string
		var more bool
		actual, remainder, more = strings.Cut(remainder, "/")
		exhausted = !more
		if actual == "" {
			return false
		}
		if expected != "*" && expected != actual {
			return false
		}
	}
	return p.subtree || exhausted
}

// MatchAny reports whether any pattern matches path.
func MatchAny(patterns []Pattern, path string) bool {
	for _, p := range patterns {
		if p.Match(path) {
			return true
		}
	}
	return false
}

// canonicalPath rejects a request whose path cannot be matched unambiguously.
// Dot segments and encoded separators could otherwise select a different routed
// target than the one the guard decided about.
// CanonicalPath returns the request path a policy may match against, and
// reports false for one that cannot be matched unambiguously.
func CanonicalPath(r *http.Request) (string, bool) {
	if r == nil || r.URL == nil {
		return "", false
	}
	// net/http percent-decodes URL.Path, so an encoded separator is only
	// visible in the raw form.
	return CanonicalPathOf(r.URL.Path, r.URL.RawPath)
}

// CanonicalPathOf is CanonicalPath over the decoded path and the raw one, for a
// caller whose request is not a *http.Request.
//
// Both are wanted because the refusals need both: an encoded separator is
// invisible once decoded, and dot segments are what the decoded form shows. A
// transport that keeps only one of the two passes it as decoded and the other
// empty, which loses the encoded-separator refusal and is why this takes the
// pair rather than guessing.
func CanonicalPathOf(path, raw string) (string, bool) {
	if path == "" || !strings.HasPrefix(path, "/") {
		return "", false
	}
	if raw != "" && (strings.Contains(raw, "%2f") || strings.Contains(raw, "%2F")) {
		return "", false
	}
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for index, segment := range segments {
		if segment == "." || segment == ".." {
			return "", false
		}
		// An empty segment means the path carried "//" somewhere, and routers
		// disagree about whether that is the same resource as the single-slash
		// form. A policy cannot decide about a path whose target depends on who
		// resolves it, so this is a refusal — the same answer dot segments and
		// encoded separators get, and for the same reason.
		//
		// The last segment is exempt: it is empty for the ordinary directory
		// form "/admin/", which Match normalizes rather than refusing.
		if segment == "" && index != len(segments)-1 {
			return "", false
		}
	}
	return path, true
}

// Protected reports whether a path falls inside a policy's scope.
//
// Exclude wins over include, and that precedence is the reason this is one
// function rather than two calls at each site: reversing it would silently
// widen a policy an operator wrote to narrow one, and the two orders look
// identical in a diff.
func Protected(include, exclude []Pattern, path string) bool {
	if MatchAny(exclude, path) {
		return false
	}
	return MatchAny(include, path)
}
