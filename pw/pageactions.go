package pw

import (
	"encoding/json"
	"net/http"

	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinybind-go/htmlbind"
)

// pageActionMetaName is where a document carries the server functions its own
// scripts may call. It is a contract between this file and boundary.js.
const pageActionMetaName = "pw-actions"

// PageAction is one server function a page's component scripts may call.
type PageAction = pwruntime.PageAction

// RegisterPageActions publishes the actions reachable from one route.
//
// The caller is a generated init beside the registry it reads, derived by pw
// generate the way the reloadable registration already is, so an application
// writes nothing and cannot leave the two out of step.
func RegisterPageActions(pattern string, actions ...PageAction) {
	pwruntime.RegisterPageActions(pattern, actions...)
}

// ActionCallHeader says a server function was called by name from a script
// rather than reached by a gesture on an element.
//
// It is a second axis rather than a second mode, because the mode already says
// what it says: this is an action request, and an update response applies to it
// exactly as it does to a submitted form. What this adds is who is holding the
// answer. A gesture has a document to update and nowhere to put a value; a
// script called this and can read one.
//
// A header of this framework's own rather than the mode's parameter field: the
// module documents that field as the caller's wire version, and giving it a
// second meaning would collide the day a caller versions its wire.
const ActionCallHeader = "Pw-Call"

// WantsValue reports a server function called from a script.
//
// It is the third branch of an action handler, beside WantsUpdate and the
// ordinary response:
//
//	switch {
//	case pw.WantsValue(r):   // a script called it, so answer with a value
//	case pw.WantsUpdate(r):  // an intercepted gesture, so answer with regions
//	default:                 // a native submit, so redirect
//	}
//
// A handler that never asks is unchanged: it writes one response and every
// caller gets it, which is what a handler answering only regions already does.
func WantsValue(r *http.Request) bool {
	return r != nil && r.Header.Get(ActionCallHeader) != ""
}

// pageActionHeadNodes carries this route's actions into the document.
//
// The route is read from the pattern the router reports having matched, so
// nothing here re-resolves a path that was already resolved. A request that
// matched no pattern — a handler registered without one, or a call from a test
// — carries none, which is the same answer as a route with no actions.
func pageActionHeadNodes(r *http.Request) []htmlbind.HeadNode {
	if r == nil {
		return nil
	}
	actions := pwruntime.PageActionsFor(r.Pattern)
	if len(actions) == 0 {
		return nil
	}
	// A name-to-address object, which is the shape a script reads: it names the
	// Go function and gets back something it can call.
	addresses := make(map[string]string, len(actions))
	for _, action := range actions {
		addresses[action.Name] = action.Path
	}
	encoded, err := json.Marshal(addresses)
	if err != nil {
		return nil
	}
	return []htmlbind.HeadNode{
		htmlbind.HeadMeta(
			htmlbind.HeadAttr{Name: "name", Value: pageActionMetaName},
			htmlbind.HeadAttr{Name: "content", Value: string(encoded)},
		),
	}
}
