package pwruntime

import (
	"sync/atomic"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// The document shell and the error page are registered from generated init,
// and they live here rather than in a transport runtime for a reason that is
// about correctness rather than tidiness.
//
// Generated registration calls RegisterHTMLDocument on whichever package it
// imports, and a second backend's generated file imports the other one. Two
// runtime packages each holding this state would mean two registries: the build
// that read the empty one would render every page with no document around it,
// and nothing would report that.
//
// Both values are transport-free — a wrapper chain and a function from a
// problem to a fragment — so nothing is given up by putting them here.

var documentState atomic.Pointer[[]htmlbind.Wrapper]

// RegisterHTMLDocument installs the document shell every rendered page is
// wrapped in. It is called once, from generated code, and a second call is a
// programming error rather than a replacement.
func RegisterHTMLDocument(wrapper htmlbind.Wrapper) {
	chain := []htmlbind.Wrapper{wrapper}
	if !documentState.CompareAndSwap(nil, &chain) {
		panic("popcornweb: HTML document is already registered")
	}
}

// RegisteredHTMLDocument returns the shell chain, or nil where none was
// registered. The slice is freshly allocated per call, so a caller appending
// its own wrappers cannot reach the chain another request is rendering.
func RegisteredHTMLDocument() []htmlbind.Wrapper {
	chain := documentState.Load()
	if chain == nil {
		return nil
	}
	return append([]htmlbind.Wrapper(nil), *chain...)
}

// RegisteredHTMLDocumentWith returns the shell chain with wrappers appended
// after it, built in one allocation. It exists for the page render path: the
// plain accessor's copy carries no spare capacity, so appending a route's own
// wrappers to it re-allocated and re-copied on every rendered page. The result
// is freshly allocated for the same isolation reason as above.
func RegisteredHTMLDocumentWith(wrappers []htmlbind.Wrapper) []htmlbind.Wrapper {
	chain := documentState.Load()
	if chain == nil {
		return append([]htmlbind.Wrapper(nil), wrappers...)
	}
	combined := make([]htmlbind.Wrapper, 0, len(*chain)+len(wrappers))
	combined = append(combined, *chain...)
	return append(combined, wrappers...)
}

// SwapHTMLDocument installs a shell chain and returns what was there.
//
// It exists for tests, which install a document and must put back what they
// found; the compare-and-swap above admits one registration and no undo, which
// is right for an init and unusable for a test. Passing nil clears it.
func SwapHTMLDocument(chain []htmlbind.Wrapper) []htmlbind.Wrapper {
	if chain == nil {
		previous := documentState.Swap(nil)
		if previous == nil {
			return nil
		}
		return *previous
	}
	previous := documentState.Swap(&chain)
	if previous == nil {
		return nil
	}
	return *previous
}

// HTMLErrorPage renders an application error page from a problem. It names no
// transport, which is what lets one registration serve both runtimes.
type HTMLErrorPage func(Problem) HTMLFragment

// HTMLFragment is a bound component ready to render. It is named here as well
// as on each runtime because an application file that registers an error page
// belongs to both builds, and a file naming a runtime does not.
type HTMLFragment = htmlbind.Fragment

// The resolver is installed once from generated init and read on every error
// response, so it is published the way documentState is: an RLock is still an
// atomic write on a shared cache line, and this read must cost a load.
var errorPageState atomic.Pointer[HTMLErrorPage]

// RegisterHTMLErrorPage installs the application's error page resolver.
func RegisterHTMLErrorPage(resolve HTMLErrorPage) {
	if resolve == nil {
		errorPageState.Store(nil)
		return
	}
	errorPageState.Store(&resolve)
}

// RegisteredHTMLErrorPage returns the resolver, or nil where none was
// registered and a problem therefore takes its document form.
func RegisteredHTMLErrorPage() HTMLErrorPage {
	resolve := errorPageState.Load()
	if resolve == nil {
		return nil
	}
	return *resolve
}
