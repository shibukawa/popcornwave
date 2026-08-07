package session

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// UnavailableHandler responds to a request whose session backend could not be
// reached. The lifecycle fails closed here instead of silently downgrading the
// request to unauthenticated.
type UnavailableHandler func(http.ResponseWriter, *http.Request, error)

// Middleware resolves the session token into the registered slots.
//
// A missing token is a browser with no session yet, not a failure. A malformed
// or expired one is cleared and the request continues with no session. A
// backend failure is answered by unavailable without calling the next handler.
func (m *Manager) Middleware(unavailable UnavailableHandler) func(http.Handler) http.Handler {
	if unavailable == nil {
		unavailable = defaultUnavailable
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			current := &state{manager: m, writer: w, request: r}
			if !m.lazyRecord {
				switch err := current.resolveRecord(); {
				case err == nil:
				case staleSessionError(err):
					// Stale or unreadable browser state: clear it and continue with
					// no session.
					m.clearCookie(w)
				default:
					unavailable(w, r, err)
					return
				}
			}
			current.loadCookieSlots()

			// The session is storage. Whether the browser holding it is a
			// signed-in account is settled at SlotAuthentication by whatever
			// owns the login, which reads its own slot.
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), stateKey{}, current)))
		})
	}
}

func defaultUnavailable(w http.ResponseWriter, _ *http.Request, _ error) {
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
}

// Attach resolves the session of r and returns the request carrying it, for a
// caller that reaches the manager outside its middleware.
//
// Middleware is the normal path and does this for every request. Attach exists
// for the callers that legitimately have no middleware above them, such as a
// test seam that establishes a session against a bare request. Attaching twice
// is a no-op: the request keeps the session it already has.
func (m *Manager) Attach(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
	if w == nil || r == nil {
		return r, fmt.Errorf("%w: nil request", ErrInvalidOptions)
	}
	if _, ok := currentState(r.Context()); ok {
		return r, nil
	}
	current := &state{manager: m, writer: w, request: r}
	if !m.lazyRecord {
		switch err := current.resolveRecord(); {
		case err == nil:
		case staleSessionError(err):
			m.clearCookie(w)
		default:
			return r, err
		}
	}
	current.loadCookieSlots()
	return r.WithContext(context.WithValue(r.Context(), stateKey{}, current)), nil
}

// Rotate revokes the record the request carries and issues a replacement under
// a new token, keeping every slot value.
//
// It is the fixation defense, so it changes the token and never the state,
// which is what lets a login keep what the anonymous browser accumulated. It is
// also the promotion: the replacement is written to the configured server
// backend, so a Private slot that rode a sealed cookie while the session was
// anonymous lands on the server and its cookie is expired in the same response.
//
// A deployment running the cookie backend promotes nothing, because the
// destination is where the value already is.
func (m *Manager) Rotate(w http.ResponseWriter, r *http.Request) error {
	current, ok := currentState(r.Context())
	if !ok {
		return fmt.Errorf("%w: no session middleware on request", ErrInvalidOptions)
	}
	if m.server == nil {
		return errNoRecordStore
	}
	if err := current.resolveRecord(); err != nil && !staleSessionError(err) {
		return err
	}
	previous := current.token
	previousHash := current.tokenHash()
	// A slot that declared ResetOnRotate is dropped rather than carried, so the
	// replacement session mints its own.
	for _, entry := range m.slots {
		if entry.resetOnRotate {
			current.clearSlotValue(entry)
		}
	}

	// Revoke first, so no window leaves two live tokens for one browser. This
	// also expires the record cookie, which is the marker of an unpromoted
	// session: clearing it is what makes the next request read from the server.
	if previous != "" {
		for _, store := range m.stores() {
			if err := store.Delete(current.bindTo(store), previousHash); err != nil {
				return err
			}
		}
	}

	current.token, current.hash = "", ""
	current.promoted = true
	current.dirtyAnon, current.dirtyServer = false, false
	if err := current.ensureToken(); err != nil {
		return err
	}
	current.dirtyServer = true
	if err := current.flush(); err != nil {
		// The replacement did not land. The previous record is already revoked,
		// so report rather than leave the caller believing the login finished.
		return err
	}
	return nil
}

// Destroy ends the whole session: every record is revoked and every cookie the
// session owns is expired, whatever placement each slot carries.
//
// A cookie a deployment wants to survive this is a Jar cookie and not a
// registered slot, which is why a sign-in hint lives outside the session.
func (m *Manager) Destroy(w http.ResponseWriter, r *http.Request) error {
	current, ok := currentState(r.Context())
	if !ok {
		m.clearCookie(w)
		return nil
	}
	if err := current.resolveRecord(); err != nil && !staleSessionError(err) {
		return err
	}
	return current.destroy()
}

// currentState returns the request-scoped session installed by Middleware.
func currentState(ctx context.Context) (*state, bool) {
	if ctx == nil {
		return nil, false
	}
	current, ok := ctx.Value(stateKey{}).(*state)
	return current, ok && current != nil
}

// Present reports whether the request carries a session token at all. It is
// what a logout endpoint checks before doing work, and it is not an
// authentication claim: an anonymous browser holding a cart has a session.
func Present(ctx context.Context) bool {
	current, ok := currentState(ctx)
	if !ok {
		return false
	}
	_ = current.resolveRecord()
	return current.token != ""
}

func staleSessionError(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, ErrExpired) ||
		errors.Is(err, ErrCodec) || errors.Is(err, ErrInvalidKey)
}
