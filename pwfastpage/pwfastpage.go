// Package pwfastpage is the runtime that generated page tree code calls on the
// second transport.
//
// It is the pwpage counterpart, and exists for the same reason that one does:
// the generation templates name their runtime symbols literally, and Option in
// pwfast already means something else. So the page tree points here, and this
// points at pwfast.
//
// Application code has no reason to import it.
package pwfastpage

import (
	"github.com/shibukawa/popcornwave/pwfast"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// Router is what the generated Register installs on. Registration needs one
// method, and pwfast.ServeMux satisfies it.
type Router interface {
	HandleFunc(pattern string, handler func(*fasthttp.RequestCtx))
}

// Wrapper is one ancestor layout of a page, already bound to its arguments.
type Wrapper = pwfast.HTMLWrapper

// Fragment is a page component with its parameters bound.
type Fragment = pwfast.HTMLFragment

// Option tunes one render.
type Option = pwfast.HTMLOption

// Render answers a request with one page, composed inside its ancestor layouts
// and the registered document shell.
//
// The error result is always nil, exactly as the net/http half's is: the pwfast
// response path answers its own failures, having already chosen a status and
// rendered through the same chain, which is more than a caller holding only the
// error could do. The result exists so the generated handler shape is the one
// the emitter produces, and so a future failure this package cannot answer has
// somewhere to go.
func Render(r *fasthttp.RequestCtx, wrappers []Wrapper, leaf Fragment, options ...Option) error {
	pwfast.WriteHTMLPage(r, wrappers, leaf, options...)
	return nil
}
