//go:build !fasthttp

package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornweb/pw"
)

func init() {
	mux.HandleFunc("GET /{$}", dashboard)
}

// dashboard is an ordinary handler, and that is the point of the example. It
// names no source, starts no goroutine, opens no connection, and writes no
// header: the live bindings are called by generated code with the
// subscription's own context, so a page that keeps updating is written like a
// page that does not.
func dashboard(w http.ResponseWriter, r *http.Request) {
	pw.WriteHTML(w, r, Dashboard(DashboardParams{Room: "general"}))
}
