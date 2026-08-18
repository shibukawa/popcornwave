//go:build !fasthttp

package fastfixture

import "github.com/shibukawa/popcornweb/pw"

// The route table, registered the ordinary way.
//
// It carries the tag because every declaration in it is transport-typed: the
// mux is one transport's, and so is the handler it names. What the second build
// gets instead is a generated RegisterRoutes, emitted from this same table onto
// pwfast.RouteInstaller — which is why the addresses cannot drift apart.
var mux = pw.NewServeMux()

func init() { mux.HandleFunc("GET /greet", Greet) }

// Handlers is what an entry point mounts.
func Handlers() *pw.ServeMux { return mux }
