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
			resolved, err := m.Resolve(HTTPCarrier(w, r))
			if err != nil {
				unavailable(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(resolved.Attach(r.Context())))
		})
	}
}

// Resolved is a session resolved against one request, ready to be published
// wherever that transport publishes request state.
type Resolved struct{ state *state }

// ValueStore is a request value that carries its own state instead of being
// replaced by a derived copy, which is how the second transport publishes
// request state.
type ValueStore interface {
	SetUserValue(key, value any)
}

// StoreOn records this session on a request value that carries its own state.
//
// It exists because the key is this package's and should stay that way: a
// transport publishing the session would otherwise need the key exported, and
// an exported key is one any code can write, which for a session means any code
// can present one.
func (r Resolved) StoreOn(store ValueStore) {
	if r.state == nil || store == nil {
		return
	}
	store.SetUserValue(stateKey{}, r.state)
}

// Attach returns ctx carrying this session.
func (r Resolved) Attach(ctx context.Context) context.Context {
	if r.state == nil {
		return ctx
	}
	return context.WithValue(ctx, stateKey{}, r.state)
}

// Resolve reads the session a carrier presents, and reports the failure that
// should be answered rather than continued through.
//
// It is the whole of what Middleware does apart from wiring, and it exists
// separately so a second transport carries a session without a second copy of
// any of this. The rules a copy would have had to reproduce are the ones worth
// not reproducing: when a token rotates, which cookie is cleared on a stale
// record, and the difference between browser state that is merely stale and a
// backend that is down.
//
// Stale or unreadable browser state is cleared and the request continues with
// no session, because a client holding an expired cookie has not done anything
// wrong. A backend failure returns an error, because continuing would serve the
// request as anonymous and a reader could not tell that apart from a signed-out
// visitor.
func (m *Manager) Resolve(carrier Carrier) (Resolved, error) {
	current := &state{manager: m, carrier: carrier}
	if !m.lazyRecord {
		switch err := current.resolveRecord(); {
		case err == nil:
		case staleSessionError(err):
			m.clearCookie(current.carrier)
		default:
			return Resolved{}, err
		}
	}
	current.loadCookieSlots()
	// The session is storage. Whether the browser holding it is a signed-in
	// account is settled at SlotAuthentication by whatever owns the login,
	// which reads its own slot.
	return Resolved{state: current}, nil
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
	resolved, err := m.AttachTo(HTTPCarrier(w, r))
	if err != nil {
		return r, err
	}
	return r.WithContext(resolved.Attach(r.Context())), nil
}

// AttachTo is Attach over a carrier: it returns the session already attached to
// the carrier's context, and otherwise resolves one.
//
// The caller publishes the result, through Attach for a transport that derives
// a context and through StoreOn for one that writes into its request value.
// Which of those it is, is the only part that differs.
func (m *Manager) AttachTo(carrier Carrier) (Resolved, error) {
	if !readable(carrier) || !writable(carrier) {
		return Resolved{}, fmt.Errorf("%w: nil request", ErrInvalidOptions)
	}
	if existing, ok := currentState(carrier.Context()); ok {
		return Resolved{state: existing}, nil
	}
	current := &state{manager: m, carrier: carrier}
	if !m.lazyRecord {
		switch err := current.resolveRecord(); {
		case err == nil:
		case staleSessionError(err):
			m.clearCookie(current.carrier)
		default:
			return Resolved{}, err
		}
	}
	current.loadCookieSlots()
	return Resolved{state: current}, nil
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
	return m.RotateOn(r.Context())
}

// RotateOn is Rotate over the context the session was attached to.
//
// The response writer Rotate takes is not read: the cookies of a rotation go to
// the carrier the session was resolved with, which is fixed when the session is
// attached rather than when it turns. So the context is the whole input, and a
// transport whose request value is its context passes that.
func (m *Manager) RotateOn(ctx context.Context) error {
	current, ok := currentState(ctx)
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
	return m.DestroyOn(r.Context())
}

// DestroyOn is Destroy over the context the session was attached to, on the
// same terms as RotateOn.
func (m *Manager) DestroyOn(ctx context.Context) error {
	current, ok := currentState(ctx)
	if !ok {
		// No session was attached, so there is no record to revoke and no
		// carrier to expire a cookie on. This used to read current.carrier for
		// the clear, which dereferences the nil the lookup just reported.
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
