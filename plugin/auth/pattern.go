package auth

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
type pattern struct {
	source   []string
	subtree  bool
	original string
}

func compilePatterns(values []string) ([]pattern, error) {
	if len(values) == 0 {
		return nil, nil
	}
	compiled := make([]pattern, 0, len(values))
	for _, value := range values {
		p, err := compilePattern(value)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, p)
	}
	return compiled, nil
}

func compilePattern(value string) (pattern, error) {
	if !strings.HasPrefix(value, "/") {
		return pattern{}, fmt.Errorf("pattern %q must start with a slash", value)
	}
	if strings.Contains(value, "?") || strings.Contains(value, "#") {
		return pattern{}, fmt.Errorf("pattern %q must not contain a query or fragment", value)
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
				return pattern{}, fmt.Errorf("pattern %q may use ** only as the last segment", value)
			}
			subtree = true
		case segment == "*":
		case segment == "", segment == ".", segment == "..":
			return pattern{}, fmt.Errorf("pattern %q has an empty or dot segment", value)
		case strings.Contains(segment, "*"):
			return pattern{}, fmt.Errorf("pattern %q may not use a mid-segment wildcard", value)
		}
	}
	if subtree {
		segments = segments[:len(segments)-1]
	}
	return pattern{source: segments, subtree: subtree, original: value}, nil
}

// match reports whether path matches. path must already be the canonical
// request path.
func (p pattern) match(path string) bool {
	trimmed := strings.TrimPrefix(path, "/")
	var segments []string
	if trimmed != "" {
		segments = strings.Split(trimmed, "/")
	}
	if p.subtree {
		if len(segments) < len(p.source) {
			return false
		}
	} else if len(segments) != len(p.source) {
		return false
	}
	for index, expected := range p.source {
		actual := segments[index]
		if actual == "" {
			return false
		}
		if expected == "*" {
			continue
		}
		if expected != actual {
			return false
		}
	}
	return true
}

func matchAny(patterns []pattern, path string) bool {
	for _, p := range patterns {
		if p.match(path) {
			return true
		}
	}
	return false
}

// canonicalPath rejects a request whose path cannot be matched unambiguously.
// Dot segments and encoded separators could otherwise select a different routed
// target than the one the guard decided about.
func canonicalPath(r *http.Request) (string, bool) {
	if r == nil || r.URL == nil {
		return "", false
	}
	path := r.URL.Path
	if path == "" || !strings.HasPrefix(path, "/") {
		return "", false
	}
	// net/http percent-decodes URL.Path, so an encoded separator is only
	// visible in the raw form.
	if raw := r.URL.RawPath; raw != "" && (strings.Contains(raw, "%2f") || strings.Contains(raw, "%2F")) {
		return "", false
	}
	for _, segment := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if segment == "." || segment == ".." {
			return "", false
		}
	}
	return path, true
}
