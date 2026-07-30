// Package otelui serves the Web UI half of the pw dev telemetry viewer.
//
// The receiver, store, and snapshot API are linked from
// github.com/shibukawa/localotelviewer/viewer, whose public package ships no
// assets on purpose so that embedding it costs nothing to an application that
// brings its own UI. This package is that UI: a committed production build of
// the upstream React component, so a pw release build needs no Node toolchain.
//
// Rebuild it with `npm ci && npm run build` in webui/ after taking new
// component sources from the upstream repository. See NOTICE for attribution.
package otelui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web
var assets embed.FS

// Handler serves the viewer UI. It is mounted through viewer.WithWebHandler,
// which routes only the paths the receiver and snapshot API do not claim.
func Handler() http.Handler {
	root, err := fs.Sub(assets, "web")
	if err != nil {
		// The build output is embedded at compile time, so a failure here is a
		// broken binary rather than a condition a developer loop can report.
		panic(err)
	}
	return http.FileServer(http.FS(root))
}
