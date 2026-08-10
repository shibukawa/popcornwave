// Package pwfast is the pw surface over a fasthttp request, for the build that
// decision:transport-source-transform generates.
//
// It is the second half of a pair. An application author writes net/http and
// calls pw; generation rewrites those handlers for the other transport and
// imports this package under the name pw, so a rewritten call selector is
// unchanged and only the import line moves. Every declaration here therefore
// carries the same name as its pw counterpart, takes the request value first,
// and keeps the parameter names the net/http half uses — which is what makes a
// rewritten body the same text on both transports rather than a translation of
// it.
//
// # Not tagged
//
// This package carries no build constraint, deliberately. Only application
// files are tagged; a library behind a tag is invisible to go vet, go test and
// gopls in an untagged run, and both surfaces are worth covering in one.
//
// # Shared state, not a second copy
//
// The document shell, the error page, the reloadable components, the problem
// value and the resolved update configuration all live in pwruntime, and both
// runtimes reach that one copy. This is not tidiness: generated registration
// runs from init and calls whichever package it imports, so two registries
// would leave one build rendering pages with no document around them and
// answering no redraw at all.
//
// The update configuration travels the same way. Whichever runtime read the
// configuration file publishes what it resolved, and this half reads it — a
// settings file is not a transport concern, and this package has no reader of
// its own.
//
// # Streaming, and where the two halves differ
//
// ServeUpdate and ServeLive answer a streamed navigation and a live
// subscription. Both call the module's own entries, and the delta path is the
// same entry the net/http half calls, so the two transports send the same
// records rather than two implementations that agree.
//
// The live path is the one place that is not yet true. The net/http half
// predates the module having a live entry and keeps a loop of its own over the
// chain renderer, where this half calls the module. What a client receives is
// the same protocol either way; what differs is which code produces it, and
// two hand-written live loops would be two chances to disagree about framing,
// digests and close reasons on the one response nobody watches. Moving the
// other half onto the module entry is what closes that, and it is tracked.
//
// Nothing that is missing is stubbed. A declaration that compiled and did
// nothing would hide a gap where an absent one is a build error naming the
// symbol, which is what the refusal contract does everywhere else.
package pwfast
