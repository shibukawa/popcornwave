package middlewares

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Lifecycle describes when an HTTP resource becomes deprecated and when it
// is expected to become unavailable. Either date can be omitted.
type Lifecycle struct {
	DeprecatedAt     time.Time
	SunsetAt         time.Time
	DocumentationURL string
}

// LifecycleHeaders validates and returns RFC 9745 Deprecation and RFC 8594
// Sunset response middleware. The middleware does not change route behavior.
func LifecycleHeaders(lifecycle Lifecycle) (func(http.Handler) http.Handler, error) {
	headers, err := lifecycleHeaders(lifecycle)
	if err != nil {
		return nil, err
	}
	// Flattened once: the set is fixed at construction, and ranging the map
	// and its value slices per request paid iteration for a known answer.
	type headerLine struct{ name, value string }
	var lines []headerLine
	for name, values := range headers {
		for _, value := range values {
			lines = append(lines, headerLine{name: name, value: value})
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, line := range lines {
				w.Header().Add(line.name, line.value)
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

func lifecycleHeaders(lifecycle Lifecycle) (http.Header, error) {
	if lifecycle.DeprecatedAt.IsZero() && lifecycle.SunsetAt.IsZero() && lifecycle.DocumentationURL == "" {
		return nil, fmt.Errorf("middlewares: empty API lifecycle")
	}
	if !lifecycle.DeprecatedAt.IsZero() && !lifecycle.SunsetAt.IsZero() && lifecycle.SunsetAt.Before(lifecycle.DeprecatedAt) {
		return nil, fmt.Errorf("middlewares: sunset precedes deprecation")
	}

	headers := make(http.Header)
	if !lifecycle.DeprecatedAt.IsZero() {
		headers.Set("Deprecation", "@"+strconv.FormatInt(lifecycle.DeprecatedAt.Unix(), 10))
	}
	if !lifecycle.SunsetAt.IsZero() {
		headers.Set("Sunset", lifecycle.SunsetAt.UTC().Format(http.TimeFormat))
	}
	if lifecycle.DocumentationURL != "" {
		sunsetOnly := lifecycle.DeprecatedAt.IsZero() && !lifecycle.SunsetAt.IsZero()
		link, err := lifecycleLink(lifecycle.DocumentationURL, sunsetOnly)
		if err != nil {
			return nil, err
		}
		headers.Add("Link", link)
	}
	return headers, nil
}

func lifecycleLink(raw string, sunsetOnly bool) (string, error) {
	if strings.ContainsAny(raw, "<>\r\n") {
		return "", fmt.Errorf("middlewares: invalid lifecycle documentation URL")
	}
	target, err := url.Parse(raw)
	if err != nil || !target.IsAbs() || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") {
		return "", fmt.Errorf("middlewares: lifecycle documentation URL must be an absolute HTTP URL")
	}
	relation := "deprecation"
	if sunsetOnly {
		relation = "sunset"
	}
	return "<" + target.String() + ">; rel=\"" + relation + "\"", nil
}
