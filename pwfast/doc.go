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
// # What is not here yet
//
// The partial-update surface — WantsUpdate, WriteUpdate, WriteUpdateNavigate,
// Redraw, RedrawComponents — and live boundary delivery. These are not omitted
// by choice: the upstream htmlupdate runtime holds an http.Flusher and reads
// *http.Request throughout, and its port is deferred upstream. A handler
// calling one of them is refused by the transform with a message naming it,
// which is the honest outcome; a stub here that compiled and did nothing would
// not be.
//
// WriteHTML and WriteHTMLPage are also absent, for a different and smaller
// reason: both apply the registered document shell, and that registry is
// private to pw. Moving it to a leaf both packages can read is the next step,
// and it is the same move upstream made for the error types. WriteHTMLChain
// takes its wrappers explicitly and needs none of it, so it is here.
package pwfast
