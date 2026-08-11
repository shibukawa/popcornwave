package pwruntime

import (
	"strconv"
	"strings"
)

// AcceptsHTML reports whether a client would rather have a page than a
// document, given its Accept header.
//
// An absent, empty, or unreadable header is not a preference, so it takes the
// API representation — which is also what a client that sent no opinion at all
// is most likely to be.
//
// It is here rather than in either runtime because it reads a header and
// decides a representation, and two builds of one application answering the
// same request differently is exactly what a shared rule prevents.
func AcceptsHTML(accept string) bool {
	htmlQuality, jsonQuality := -1.0, -1.0
	for entry := range splitAccept(accept, ',') {
		media, parameters, _ := strings.Cut(entry, ";")
		media = strings.TrimSpace(strings.ToLower(media))
		quality := 1.0
		for parameter := range splitAccept(parameters, ';') {
			name, value, found := strings.Cut(strings.TrimSpace(parameter), "=")
			if !found || strings.TrimSpace(name) != "q" {
				continue
			}
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				quality = parsed
			}
		}
		switch media {
		case "text/html", "application/xhtml+xml":
			htmlQuality = max(htmlQuality, quality)
		case "application/json", "application/problem+json":
			jsonQuality = max(jsonQuality, quality)
		}
	}
	return htmlQuality > 0 && htmlQuality >= jsonQuality
}

func splitAccept(value string, separator byte) func(func(string) bool) {
	return func(yield func(string) bool) {
		for value != "" {
			piece := value
			if index := strings.IndexByte(value, separator); index >= 0 {
				piece, value = value[:index], value[index+1:]
			} else {
				value = ""
			}
			if !yield(piece) {
				return
			}
		}
	}
}
