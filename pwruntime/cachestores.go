package pwruntime

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// The set of configured stores, and how a call site reaches one.
//
// The set is keyed on the configuration rather than built once, so an operator
// who resizes or disables caching takes effect without a restart — and dropping
// the previous set is the eviction that change implies. Two goroutines reaching
// a cold set can each build one, and one is then dropped along with whatever it
// had collected. That costs a few repeated fetches once per process and needs
// no lock on a path every cached read takes.

type cacheStoreSet struct {
	config CacheConfig
	stores map[string]*CacheStore
	names  []string
	err    error
}

var cacheStoreState atomic.Pointer[cacheStoreSet]

// MemoStore resolves a configured store by name.
//
// A disabled section returns no store and no error, so removing caching from a
// deployment edits no call site: every operation on a nil store falls through
// to its fetch. A name the set does not hold is an error naming what is
// configured, because the alternative is a project that believes it caches.
//
// The set is immutable once built, so an application whose store name is static
// may resolve once during setup and hold the handle, which turns an unknown
// name into a startup failure rather than a first-request one.
func MemoStore(ctx context.Context, name string) (*CacheStore, error) {
	set := cacheStores()
	if set.err != nil {
		return nil, set.err
	}
	if len(set.stores) == 0 {
		return nil, nil
	}
	store, ok := set.stores[name]
	if !ok {
		return nil, fmt.Errorf("cache: no store named %q; configured: %s", name, strings.Join(set.names, ", "))
	}
	return store, nil
}

// cacheStores returns the set this configuration describes.
func cacheStores() *cacheStoreSet {
	config, _ := RegisteredConfig[CacheConfig]()
	if cached := cacheStoreState.Load(); cached != nil && sameCacheConfig(cached.config, config) {
		return cached
	}
	set := buildCacheStores(config)
	set.config = config
	// The store list is cloned because the registered configuration shares its
	// backing array with every copy handed out: a field edited in place would
	// otherwise change the cached copy too, and read as "unchanged".
	set.config.Stores = slices.Clone(config.Stores)
	cacheStoreState.Store(set)
	return set
}

// sameCacheConfig decides whether the published configuration is the one the
// current set was built from. It compares fields directly — every element is
// comparable — where a deterministic fingerprint string used to be built and
// thrown away on every MemoStore call just to say "unchanged".
func sameCacheConfig(a, b CacheConfig) bool {
	return a.Enabled == b.Enabled && slices.Equal(a.Stores, b.Stores)
}

func buildCacheStores(config CacheConfig) *cacheStoreSet {
	set := &cacheStoreSet{stores: map[string]*CacheStore{}}
	if !config.Enabled {
		return set
	}
	if len(config.Stores) == 0 {
		set.err = fmt.Errorf("cache: enabled with no [[cache.stores]] element")
		return set
	}
	for index, element := range config.Stores {
		store, err := newCacheStore(element)
		if err != nil {
			set.err = fmt.Errorf("cache.stores[%d]: %w", index, err)
			return set
		}
		if _, exists := set.stores[store.name]; exists {
			set.err = fmt.Errorf("cache.stores[%d]: duplicate store name %q, which a call site could not address", index, store.name)
			return set
		}
		set.stores[store.name] = store
		set.names = append(set.names, store.name)
	}
	sort.Strings(set.names)
	return set
}

func newCacheStore(config CacheStoreConfig) (*CacheStore, error) {
	if err := validCacheStoreName(config.Name); err != nil {
		return nil, err
	}
	// Only memory is implemented. A store that quietly fell back to it would be
	// a shared cache that is not shared, which fails in production and passes
	// every local test.
	if config.Backend != "" && config.Backend != "memory" {
		return nil, fmt.Errorf("backend %q is not implemented; memory is the only one", config.Backend)
	}
	scoped, err := cacheScoped(config.Scope)
	if err != nil {
		return nil, err
	}
	if config.TTL <= 0 {
		return nil, fmt.Errorf("ttl must be positive; a store that holds nothing is a store nobody should configure")
	}
	if config.Stale < 0 {
		return nil, fmt.Errorf("stale cannot be negative")
	}
	if config.FetchTimeout < 0 {
		return nil, fmt.Errorf("fetch_timeout cannot be negative")
	}
	return &CacheStore{
		name:    config.Name,
		ttl:     config.TTL,
		stale:   config.Stale,
		scoped:  scoped,
		timeout: config.FetchTimeout,
		entries: map[string]cacheEntry{},
		tagged:  map[string]map[string]struct{}{},
		max:     config.MaxEntries,
		now:     time.Now,
	}, nil
}

// cacheScoped reads the scope declaration. An empty value is private, which is
// the same default the render cache's annotation takes and for the same reason:
// a public entry left private costs hits, and a private entry made public
// discloses one reader's data to another.
func cacheScoped(scope string) (bool, error) {
	switch scope {
	case "", "private":
		return true, nil
	case "public":
		return false, nil
	default:
		return false, fmt.Errorf("scope %q is neither private nor public", scope)
	}
}

func validCacheStoreName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required, because a call site addresses this store by it")
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return fmt.Errorf("name %q must be ASCII lower-case letters, digits, underscore, or hyphen", name)
		}
	}
	return nil
}
