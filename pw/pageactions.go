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
