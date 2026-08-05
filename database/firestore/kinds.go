package firestore

import (
	"sort"
	"sync"

	"github.com/shibukawa/tinybind-go/firestorebind"
)

// KindInfo is one framework-owned kind and, when its records expire, the
// property a TTL policy has to be pointed at.
//
// A deployment needs both. Nothing creates a kind, so there is no schema to
// print; what deployment tooling still has to be told is which kinds exist and
// which property expires, and only the linked code knows that.
type KindInfo struct {
	// Kind is the entity kind, which is the declared name unchanged.
	Kind string
	// ExpiryProperty is the timestamp property a TTL policy expires on. It is
	// empty for a kind whose records do not expire.
	ExpiryProperty string
}

// registry holds the kinds of the linked framework stores.
var registry struct {
	sync.Mutex
	kinds map[string]KindInfo
}

// RegisterKind records a framework-owned kind from the package that writes it.
//
// The value is the store's own record type. Its Kind method names the kind, and
// its ExpiryProperty method, when it has one, names the property a TTL policy
// expires on — so the published list is derived from the same declaration the
// codec reads rather than maintained beside it. A property renamed in one place
// and not the other is the drift this exists to prevent.
//
// Registering twice for one kind keeps the first entry, since re-initialization
// in tests re-runs the same registrations.
func RegisterKind(record firestorebind.Kinder) {
	if record == nil {
		return
	}
	info := KindInfo{Kind: record.Kind()}
	if expirer, ok := record.(firestorebind.Expirer); ok {
		if property, expires := expirer.ExpiryProperty(); expires {
			info.ExpiryProperty = property
		}
	}
	registry.Lock()
	defer registry.Unlock()
	if registry.kinds == nil {
		registry.kinds = map[string]KindInfo{}
	}
	if _, held := registry.kinds[info.Kind]; held {
		return
	}
	registry.kinds[info.Kind] = info
}

// Kinds reports the framework-owned kinds this binary links, sorted by kind so
// the output of a tool that prints them is stable.
//
// It is read by documentation and by deployment tooling, and by nothing on the
// request path: no code looks a kind up here, because a kind is intrinsic to
// the type that owns it.
func Kinds() []KindInfo {
	registry.Lock()
	out := make([]KindInfo, 0, len(registry.kinds))
	for _, info := range registry.kinds {
		out = append(out, info)
	}
	registry.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out
}
