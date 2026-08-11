package pwruntime

import (
	"sync"
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
		panic("popcornwave: HTML document is already registered")
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

var errorPageState = struct {
	sync.RWMutex
	resolve HTMLErrorPage
}{}

// RegisterHTMLErrorPage installs the application's error page resolver.
func RegisterHTMLErrorPage(resolve HTMLErrorPage) {
	errorPageState.Lock()
	defer errorPageState.Unlock()
	errorPageState.resolve = resolve
}

// RegisteredHTMLErrorPage returns the resolver, or nil where none was
// registered and a problem therefore takes its document form.
func RegisteredHTMLErrorPage() HTMLErrorPage {
	errorPageState.RLock()
	defer errorPageState.RUnlock()
	return errorPageState.resolve
}
