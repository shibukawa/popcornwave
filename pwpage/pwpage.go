// Package pwpage is the runtime that generated page tree code calls.
//
// It exists because the generation templates name their runtime symbols
// literally: a router type, a wrapper type, an option type, and one render
// entry. Most of those could come from pw itself, but Option there already
// means an application lifecycle option, and generated page code needs the name
// to mean a render option. So the page tree points at this package, and this
// package points at pw.
//
// Application code has no reason to import it.
package pwpage

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
)

// Router is what the generated Register installs on. Registration needs one
// method, so both mux types a project can carry satisfy it: pw.ServeMux, which
// is a distinct type only in a TinyGo build, and net/http.ServeMux.
type Router interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// Wrapper is one ancestor layout of a page, already bound to its arguments.
type Wrapper = pw.HTMLWrapper

// Fragment is a page component with its parameters bound.
type Fragment = pw.HTMLFragment

// Option tunes one render. The generated Register takes these and passes them
// to every page, so a per-request cache, timeout, or error hook is configured
// once for a whole tree.
type Option = pw.HTMLOption

// Render answers a request with one page, composed inside its ancestor layouts
// and the registered document shell.
//
// The error result is what the generated caller writes through pw.WriteProblem.
// It is always nil, because the pw response path answers its own failures: a
// render that fails there has already chosen a status and rendered an error page
// through the same chain, which is more than a caller holding only the error
// could do. The result exists so the generated handler shape stays the one
// system:tinybind emits, and so a future failure this package cannot answer has
// somewhere to go.
func Render(w http.ResponseWriter, r *http.Request, wrappers []Wrapper, leaf Fragment, options ...Option) error {
	pw.WriteHTMLPage(w, r, wrappers, leaf, options...)
	return nil
}
