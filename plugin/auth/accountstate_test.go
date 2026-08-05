package auth

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// installLookup installs an account lookup for one test and restores whatever
// was there, since the seam is process-wide.
func installLookup(t *testing.T, lookup AccountLookup) {
	t.Helper()
	lookupState.RLock()
	previous := lookupState.lookup
	lookupState.RUnlock()
	SetAccountLookup(lookup)
	t.Cleanup(func() { SetAccountLookup(previous) })
}

func TestAccountGateEndsASessionWhoseAccountEnded(t *testing.T) {
	var suspended atomic.Bool
	installLookup(t, func(_ context.Context, accountID string) (Account, error) {
		if accountID == "gone" {
			return Account{}, ErrUnknownIdentity
		}
		return Account{ID: accountID, Suspended: suspended.Load()}, nil
	})
	gate := newAccountGate()
	ctx := context.Background()

	if got := gate.admit(ctx, "account-1"); got != accountLive {
		t.Fatalf("a live account = %v, want accountLive", got)
	}

	// Suspension has to reach the session that already exists, which is the
	// whole point: a check placed only where a session is created never runs
	// again for the session a compromise is holding.
	suspended.Store(true)
	gate.forget("account-1")
	if got := gate.admit(ctx, "account-1"); got != accountEnded {
		t.Fatalf("a suspended account = %v, want accountEnded", got)
	}
	if got := gate.admit(ctx, "gone"); got != accountEnded {
		t.Fatalf("a removed account = %v, want accountEnded", got)
	}
	if got := gate.admit(ctx, ""); got != accountEnded {
		t.Fatalf("an empty account = %v, want accountEnded", got)
	}
}

func TestAccountGateRefusesRatherThanJudgesWhenTheStoreIsDown(t *testing.T) {
	// An unreachable store is not an answer. Admitting on it would make every
	// suspension conditional on the account store being up; denying on it would
	// sign everyone out over a blip.
	installLookup(t, func(context.Context, string) (Account, error) {
		return Account{}, errors.New("account store unreachable")
	})
	gate := newAccountGate()
	if got := gate.admit(context.Background(), "account-1"); got != accountUnknown {
		t.Fatalf("an unreachable store = %v, want accountUnknown", got)
	}
	// A failed read is not cached, so recovery needs no expiry to pass.
	if _, cached := gate.cached("account-1"); cached {
		t.Fatal("a failed read was cached")
	}
}

func TestAccountGateReReadsOnlyOnTheInterval(t *testing.T) {
	var reads atomic.Int64
	installLookup(t, func(_ context.Context, accountID string) (Account, error) {
		reads.Add(1)
		return Account{ID: accountID}, nil
	})
	now := time.Unix(1_700_000_000, 0).UTC()
	gate := newAccountGate()
	gate.now = func() time.Time { return now }
	ctx := context.Background()

	for range 10 {
		if got := gate.admit(ctx, "account-1"); got != accountLive {
			t.Fatalf("admit = %v", got)
		}
	}
	if got := reads.Load(); got != 1 {
		t.Fatalf("%d account reads inside the interval, want 1", got)
	}

	now = now.Add(accountRevalidationInterval + time.Second)
	if got := gate.admit(ctx, "account-1"); got != accountLive {
		t.Fatalf("admit after the interval = %v", got)
	}
	if got := reads.Load(); got != 2 {
		t.Fatalf("%d account reads after the interval, want 2", got)
	}

	// Forgetting is what makes a suspension immediate in the process that
	// performed it, rather than waiting out the interval.
	gate.forget("account-1")
	if got := gate.admit(ctx, "account-1"); got != accountLive {
		t.Fatalf("admit after forget = %v", got)
	}
	if got := reads.Load(); got != 3 {
		t.Fatalf("%d account reads after forget, want 3", got)
	}
}

func TestAccountGateCacheIsBounded(t *testing.T) {
	installLookup(t, func(_ context.Context, accountID string) (Account, error) {
		return Account{ID: accountID}, nil
	})
	gate := newAccountGate()
	ctx := context.Background()
	for i := range maxAccountStateEntries * 2 {
		gate.admit(ctx, "account-"+strconv.Itoa(i))
		if len(gate.entries) > maxAccountStateEntries {
			t.Fatalf("the cache grew to %d entries, past the %d cap", len(gate.entries), maxAccountStateEntries)
		}
	}
}

func TestAccountGateAdmitsWhenNoLookupIsInstalled(t *testing.T) {
	// A deployment that keeps no local account table has nothing to re-read,
	// and this must not become a refusal for it.
	installLookup(t, nil)
	gate := newAccountGate()
	if got := gate.admit(context.Background(), "account-1"); got != accountLive {
		t.Fatalf("admit with no lookup installed = %v, want accountLive", got)
	}
}
