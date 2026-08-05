package auth

import (
	"context"
	"errors"
	"sync"
	"time"
)

// isUnknownIdentity separates an account that is gone from a store that could
// not answer. Only the first ends a session.
func isUnknownIdentity(err error) bool {
	return errors.Is(err, ErrUnknownIdentity)
}

// Account suspension is checked where a session is created. A request that
// already carries one creates nothing, so a check placed only at admission
// never runs again for the sessions that matter: the ones a compromised or
// dismissed account is already holding.
//
// This is the other half. An authenticated request re-reads its account through
// the installed AccountLookup, and a session whose account has been suspended
// or removed ends there rather than at its own expiry.
//
// It is a re-read rather than a stamp compared against the session, because a
// stamp still has to be fetched from somewhere to be compared against, and the
// application already publishes exactly one way to ask about an account.
const (
	// accountRevalidationInterval bounds how long this process may go on
	// trusting an account it has not re-read.
	//
	// This is the honest answer to how long a suspension takes to reach a live
	// session, and there is no other. ForgetAccount makes it immediate for the
	// process that performed the suspension.
	accountRevalidationInterval = 30 * time.Second
	// maxAccountStateEntries bounds the cache. The keys are account
	// identifiers of sessions that exist, so this is bounded by the number of
	// signed-in accounts rather than by anything a caller chooses; the cap is
	// here so that a large one cannot grow the map without limit.
	maxAccountStateEntries = 4096
)

// accountAdmission is what a re-read said about the account behind a session.
type accountAdmission int

const (
	// accountLive means the account exists and may act.
	accountLive accountAdmission = iota
	// accountEnded means the account is suspended or gone. The session is
	// destroyed and the request continues unauthenticated.
	accountEnded
	// accountUnknown means the account could not be read at all. The request is
	// refused rather than judged: the credential was not found wanting, and
	// admitting on an outage would make every suspension conditional on the
	// account store being up.
	accountUnknown
)

// accountGate re-reads accounts behind live sessions, no more often than the
// revalidation interval.
type accountGate struct {
	mu      sync.Mutex
	entries map[string]accountEntry
	// now is injectable so tests move the clock without sleeping.
	now func() time.Time
}

type accountEntry struct {
	live      bool
	checkedAt time.Time
}

func newAccountGate() *accountGate {
	return &accountGate{entries: make(map[string]accountEntry), now: time.Now}
}

// admit reports whether the account behind an authenticated request may still
// act, reading it again when the cached answer has aged out.
func (g *accountGate) admit(ctx context.Context, accountID string) accountAdmission {
	if accountID == "" {
		return accountEnded
	}
	if live, cached := g.cached(accountID); cached {
		if live {
			return accountLive
		}
		return accountEnded
	}
	account, err := lookupAccount(ctx, accountID)
	if err != nil {
		// An unknown identifier and an unreachable store are different answers
		// and are kept different: ErrUnknownIdentity ends the session, and
		// anything else refuses the request without deciding.
		if isUnknownIdentity(err) {
			g.remember(accountID, false)
			return accountEnded
		}
		return accountUnknown
	}
	live := !account.Suspended
	g.remember(accountID, live)
	if live {
		return accountLive
	}
	return accountEnded
}

func (g *accountGate) cached(accountID string) (bool, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	entry, ok := g.entries[accountID]
	if !ok {
		return false, false
	}
	if g.now().Sub(entry.checkedAt) > accountRevalidationInterval {
		delete(g.entries, accountID)
		return false, false
	}
	return entry.live, true
}

func (g *accountGate) remember(accountID string, live bool) {
	now := g.now()
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.entries) >= maxAccountStateEntries {
		g.evictLocked(now)
	}
	g.entries[accountID] = accountEntry{live: live, checkedAt: now}
}

// evictLocked drops what the interval has already retired, and everything if
// that was not enough. Losing an entry costs one account read, so discarding
// the map is cheaper than the bookkeeping an eviction order would need.
func (g *accountGate) evictLocked(now time.Time) {
	for id, entry := range g.entries {
		if now.Sub(entry.checkedAt) > accountRevalidationInterval {
			delete(g.entries, id)
		}
	}
	if len(g.entries) >= maxAccountStateEntries {
		clear(g.entries)
	}
}

// forget drops the cached answer for one account, so the next request that
// carries it reads the account again.
func (g *accountGate) forget(accountID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, accountID)
}

// ForgetAccount makes a suspension, deletion, or credential change take effect
// immediately in this process rather than within the revalidation interval.
//
// Call it from whatever suspends or removes an account. It is not required for
// correctness: without it the change still lands, within the interval. It is
// also process-local, so a deployment running several instances still waits the
// interval on the others, and that interval is what the deployment can promise.
//
// It never grants: forgetting an account can only cause it to be read again.
func ForgetAccount(accountID string) {
	if rt := activeRuntime(); rt != nil && rt.accounts != nil {
		rt.accounts.forget(accountID)
	}
}
