package pwcli

import (
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
)

// publicAssetCompressible reports whether a served file earns a precompressed
// sibling. Eligibility is decided from the path and its media type, so an
// already-compressed format is never encoded again into something larger.
func publicAssetCompressible(name string) bool {
	mediaType := strings.ToLower(mime.TypeByExtension(filepath.Ext(name)))
	if separator := strings.IndexByte(mediaType, ';'); separator >= 0 {
		mediaType = mediaType[:separator]
	}
	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	switch mediaType {
	case "application/javascript", "application/json", "application/manifest+json",
		"application/xml", "image/svg+xml":
		return true
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html", ".css", ".js", ".mjs", ".json", ".map", ".txt", ".xml", ".svg", ".webmanifest":
		return true
	default:
		return false
	}
}

// reportDerivedAssets names every rewrite, every declined conversion, and every
// source kept anyway.
//
// An author cannot see a build-time rewrite by reading the template, and a
// source that stayed looks exactly like a conversion that never ran, so the
// build is the only place either becomes visible.
func reportDerivedAssets(stdout io.Writer, report derivedReport) {
	for _, line := range report.converted {
		fmt.Fprintf(stdout, "asset: converted %s\n", line)
	}
	for _, line := range report.skipped {
		fmt.Fprintf(stdout, "asset: kept %s\n", line)
	}
	for _, line := range report.retained {
		fmt.Fprintf(stdout, "asset: source retained %s\n", line)
	}
	for _, line := range report.unserved {
		fmt.Fprintf(stdout, "asset: not served %s (TypeScript is a build input)\n", line)
	}
}
