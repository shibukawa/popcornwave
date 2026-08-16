package pwruntime

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

// publishCache installs one cache configuration for the duration of a test and
// puts back whatever was there, so these tests leak no lookup into the rest of
// the package.
//
// It takes a pointer the test keeps, which is both what the registry hands back
// and what lets a test change the configuration under a running process the way
// an operator does.
func publishCache(t *testing.T, config *CacheConfig) {
	t.Helper()
	previous := PublishConfigLookup(func(target reflect.Type) (any, bool) {
		if target == reflect.TypeFor[CacheConfig]() {
			return config, true
		}
		return nil, false
	})
	// The set is memoized, so a test must not read one built from the previous
	// test's configuration.
	cacheStoreState.Store(nil)
	t.Cleanup(func() {
		PublishConfigLookup(previous)
		cacheStoreState.Store(nil)
	})
}

func oneStore(name string) CacheConfig {
	return CacheConfig{
		Enabled: true,
		Stores: []CacheStoreConfig{{
			Name: name, Backend: "memory", TTL: time.Minute,
			Scope: "private", MaxEntries: 16, FetchTimeout: time.Minute,
		}},
	}
}

func TestAConfiguredStoreResolvesByName(t *testing.T) {
	config := oneStore("upstream")
	publishCache(t, &config)
	store, err := MemoStore(context.Background(), "upstream")
	if err != nil {
		t.Fatalf("MemoStore: %v", err)
	}
	if store == nil {
		t.Fatal("no store for a configured name")
	}
	if store.Name() != "upstream" {
		t.Errorf("Name = %q, want upstream", store.Name())
	}
}

// The same name must answer with the same handle, because a store rebuilt per
// call would hold nothing.
func TestResolvingTwiceAnswersOneStore(t *testing.T) {
	config := oneStore("upstream")
	publishCache(t, &config)
	first, err := MemoStore(context.Background(), "upstream")
	if err != nil {
		t.Fatal(err)
	}
	second, err := MemoStore(context.Background(), "upstream")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("two resolutions produced two stores")
	}
}

// A disabled section answers with no store and no error, which is what lets a
// deployment remove caching without editing a call site.
func TestADisabledSectionResolvesToNoStore(t *testing.T) {
	config := CacheConfig{Enabled: false}
	publishCache(t, &config)
	store, err := MemoStore(context.Background(), "upstream")
	if err != nil {
		t.Fatalf("MemoStore: %v", err)
	}
	if store != nil {
		t.Errorf("a disabled section produced a store")
	}
}

func TestAnUnconfiguredNameIsAnError(t *testing.T) {
	config := oneStore("upstream")
	publishCache(t, &config)
	_, err := MemoStore(context.Background(), "typo")
	if err == nil {
		t.Fatal("an unconfigured name resolved")
	}
	// The message names what is configured, because the failure is almost
	// always a spelling and the reader needs the alternatives.
	if !strings.Contains(err.Error(), "upstream") {
		t.Errorf("error %q does not name the configured stores", err)
	}
}

func TestConfigurationIsRefusedRatherThanIgnored(t *testing.T) {
	for name, config := range map[string]CacheConfig{
		"enabled with no store": {Enabled: true},
		"unimplemented backend": {Enabled: true, Stores: []CacheStoreConfig{
			{Name: "upstream", Backend: "redis", TTL: time.Minute},
		}},
		"unknown scope": {Enabled: true, Stores: []CacheStoreConfig{
			{Name: "upstream", Backend: "memory", TTL: time.Minute, Scope: "shared"},
		}},
		"non-positive ttl": {Enabled: true, Stores: []CacheStoreConfig{
			{Name: "upstream", Backend: "memory", TTL: 0},
		}},
		"negative stale": {Enabled: true, Stores: []CacheStoreConfig{
			{Name: "upstream", Backend: "memory", TTL: time.Minute, Stale: -time.Second},
		}},
		"illegal name": {Enabled: true, Stores: []CacheStoreConfig{
			{Name: "Upstream Cache", Backend: "memory", TTL: time.Minute},
		}},
		"duplicate name": {Enabled: true, Stores: []CacheStoreConfig{
			{Name: "upstream", Backend: "memory", TTL: time.Minute},
			{Name: "upstream", Backend: "memory", TTL: time.Minute},
		}},
	} {
		t.Run(name, func(t *testing.T) {
			publishCache(t, &config)
			if _, err := MemoStore(context.Background(), "upstream"); err == nil {
				t.Errorf("%s was accepted", name)
			}
		})
	}
}

// An unimplemented backend must fail rather than fall back to memory: a store
// that silently became per-process would be a shared cache that is not shared,
// which passes every local test and fails in production.
func TestAnUnimplementedBackendDoesNotFallBackToMemory(t *testing.T) {
	config := CacheConfig{Enabled: true, Stores: []CacheStoreConfig{
		{Name: "upstream", Backend: "redis", TTL: time.Minute},
	}}
	publishCache(t, &config)
	_, err := MemoStore(context.Background(), "upstream")
	if err == nil {
		t.Fatal("an unimplemented backend was accepted")
	}
	if !strings.Contains(err.Error(), "redis") {
		t.Errorf("error %q does not name the refused backend", err)
	}
}

// Resizing takes effect without a restart, and dropping the previous set is the
// eviction that change implies. The set is memoized on a fingerprint of the
// configuration, so this changes one under a process rather than resetting the
// memo by hand.
func TestAChangedConfigurationBuildsANewSet(t *testing.T) {
	config := oneStore("upstream")
	publishCache(t, &config)
	first, err := MemoStore(context.Background(), "upstream")
	if err != nil {
		t.Fatal(err)
	}
	config.Stores[0].MaxEntries = 32
	second, err := MemoStore(context.Background(), "upstream")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("a resized store was not rebuilt")
	}
	if second.max != 32 {
		t.Errorf("rebuilt store holds max %d, want 32", second.max)
	}
	// An unchanged configuration must not rebuild, or every read would land on
	// an empty store.
	third, err := MemoStore(context.Background(), "upstream")
	if err != nil {
		t.Fatal(err)
	}
	if second != third {
		t.Errorf("an unchanged configuration rebuilt the set")
	}
}

func TestScopeDefaultsToPrivate(t *testing.T) {
	store, err := newCacheStore(CacheStoreConfig{Name: "upstream", TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if !store.scoped {
		t.Errorf("an undeclared scope produced a shared store")
	}
}
