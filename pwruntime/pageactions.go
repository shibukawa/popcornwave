package pwruntime

import (
	"sync"
	"sync/atomic"
)

// The server functions a page's own scripts may call by name.
//
// A component script that wants to mutate without a gesture has no element to
// read an address off, and it cannot compute one either: the address holds a
// digest of the declaring directory. So the set travels with the document, and
// what decides which set is the route that matched.
//
// It is per route rather than per project because the name is what a script
// writes. Two route packages may both export Rename, and one namespace cannot
// hold both — where a page's own route package is exactly the surface
// generation already publishes.

// PageAction is one server function reachable from a page's scripts.
type PageAction struct {
	// Name is the exported Go function, which is what a script names.
	Name string
	// Path is its direct endpoint, which holds no path parameter and is
	// therefore a constant rather than something to build per request.
	Path string
}

// The table is written from generated init and read on every document render,
// so readers load a frozen snapshot and only writers take the mutex: each
// registration replaces the map rather than mutating the one readers hold.
var pageActionState struct {
	sync.Mutex
	byPattern atomic.Pointer[map[string][]PageAction]
}

// RegisterPageActions publishes the actions reachable from one route.
//
// The key is the route's own registered pattern, because that is what a request
// reports having matched, so nothing here re-matches a path the router already
// resolved.
//
// The ordinary caller is a generated init beside the registry it reads, so a
// repeated pattern replaces rather than failing: regenerating is what produces
// the second call, and the two carry the same set.
func RegisterPageActions(pattern string, actions ...PageAction) {
	if pattern == "" || len(actions) == 0 {
		return
	}
	pageActionState.Lock()
	defer pageActionState.Unlock()
	next := map[string][]PageAction{pattern: actions}
	if current := pageActionState.byPattern.Load(); current != nil {
		for key, value := range *current {
			if key != pattern {
				next[key] = value
			}
		}
	}
	pageActionState.byPattern.Store(&next)
}

// PageActionsFor returns what the route that matched publishes, or nothing.
//
// Nothing is the ordinary answer: a project with no page tree registers none, a
// route whose package exports no handler has none, and both should cost a
// render exactly one map lookup.
func PageActionsFor(pattern string) []PageAction {
	if pattern == "" {
		return nil
	}
	table := pageActionState.byPattern.Load()
	if table == nil {
		return nil
	}
	return (*table)[pattern]
}

// ResetPageActions clears the registry, for a test that publishes its own.
func ResetPageActions() {
	pageActionState.Lock()
	defer pageActionState.Unlock()
	pageActionState.byPattern.Store(nil)
}
