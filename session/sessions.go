package session

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"time"
)

// Options configures a Manager.
//
// Every duration here is supplied by whatever owns the session lifetime, which
// is normally popcornwave/plugin/auth: an expiry states how long a proof of
// identity stays good, and the store holding the bytes has no basis to make
// that statement. A zero TTL is allowed and means the session is bounded by the
// browser alone.
type Options struct {
	// TTL is the absolute session lifetime. Zero writes a browser-session
	// cookie and stamps no absolute deadline.
	TTL time.Duration
	// IdleTimeout optionally expires a session that stops being used. It must
	// not exceed TTL.
	IdleTimeout time.Duration
	// RenewalInterval bounds how often an active request renews idle expiry.
	// It defaults to one tenth of IdleTimeout.
	RenewalInterval time.Duration
	// Cookie is the policy of the token cookie.
	Cookie CookieOptions
	// RecordCookie is the policy of the sealed cookie that carries the record
	// of an anonymous session. Its name defaults to DefaultDataCookieName and
	// its remaining fields default to Cookie, so both expire under one policy.
	RecordCookie CookieOptions
	// Keys protects every slot that is not Shared. It is required unless every
	// registered slot is Shared, which is the only placement protecting
	// nothing.
	Keys *Keyring
	// ServerSideAnonymous places Private records in the configured backend
	// before authentication as well as after it. It is used by the development
	// memory backend to avoid persisting an unstable sealed-cookie format.
	ServerSideAnonymous bool
	// Version invalidates records written before an incompatible change.
	Version int
	// MaxBytes bounds a cookie name and encoded value together. It defaults to
	// DefaultMaxCookieBytes.
	MaxBytes int
	Now      func() time.Time
	Random   io.Reader
}

// Manager owns the opaque token that names one browser and applies the lifetime
// it is handed, without knowing what any slot means.
//
// Handlers read and write state through Load and Value and never call the
// manager. Rotate and Destroy are called by whatever owns the login, which is
// normally popcornwave/plugin/auth.
type Manager struct {
	registry *Registry
	slots    []*slot
	codec    slotMapCodec

	// server is the configured backend. anon is the sealed cookie a session
	// uses before it is promoted. They are the same store when the deployment
	// selected the cookie backend, which is what makes promotion a no-op there.
	server  Store[slotMap]
	anon    Store[slotMap]
	anonRaw *CookieStore
	// serverIsAnon records that the deployment selected the cookie backend, so
	// the two stores are one and promotion has nowhere to go. It is a flag
	// rather than a comparison because a Store carries an uncomparable codec.
	serverIsAnon bool
	// lazyRecord defers decoding a browser-held record until a record-backed
	// operation observes it. Remote backends remain eager so availability
	// failures are still answered before the application handler runs.
	lazyRecord bool

	jars map[reflect.Type]cookieSlot

	options  Options
	cookie   CookieOptions
	sameSite http.SameSite
	renewal  time.Duration
	now      func() time.Time
	random   io.Reader
}

// NewManager validates options and returns a Manager over registry.
//
// backend is the configured server store. A nil backend selects the cookie
// backend, where nothing is kept on the server and promotion has nowhere to go.
// Registering further slots after this call is refused.
func NewManager(registry *Registry, backend RawStore, options Options) (*Manager, error) {
	if registry == nil {
		return nil, fmt.Errorf("%w: nil registry", ErrInvalidOptions)
	}
	if options.TTL < 0 || options.TTL > maxTTL {
		return nil, fmt.Errorf("%w: ttl", ErrInvalidOptions)
	}
	if options.IdleTimeout < 0 || (options.TTL > 0 && options.IdleTimeout > options.TTL) {
		return nil, fmt.Errorf("%w: idle timeout", ErrInvalidOptions)
	}
	if options.RenewalInterval < 0 {
		return nil, fmt.Errorf("%w: renewal interval", ErrInvalidOptions)
	}
	if options.Version < 0 {
		return nil, fmt.Errorf("%w: version", ErrInvalidOptions)
	}
	if registry.needsKeyring(options.ServerSideAnonymous) && options.Keys == nil {
		return nil, fmt.Errorf("%w: a browser-protected slot is registered, so a keyring is required", ErrInvalidOptions)
	}
	if backend == nil {
		if key, ok := registry.hasServerOnly(); ok {
			// The slot asked for revocation, and a record the browser holds
			// cannot be taken back.
			return nil, fmt.Errorf("%w: slot %q is ServerOnly and the cookie backend cannot revoke", ErrInvalidOptions, key)
		}
	}
	cookie, sameSite, err := normalizeCookie(options.Cookie, DefaultCookieName)
	if err != nil {
		return nil, err
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	random := options.Random
	if random == nil {
		random = rand.Reader
	}
	renewal := options.RenewalInterval
	if renewal == 0 && options.IdleTimeout > 0 {
		renewal = options.IdleTimeout / 10
	}

	slots := registry.freeze()
	// The promotion marker is encoded like a slot, so it survives the round
	// trip that tells the next request where its Private slots live.
	order := []string{promotedMarker}
	for _, entry := range slots {
		if !entry.placement.cookiePlaced() {
			order = append(order, entry.key)
		}
	}
	codec := slotMapCodec{order: order}

	manager := &Manager{
		registry: registry,
		slots:    slots,
		codec:    codec,
		jars:     map[reflect.Type]cookieSlot{},
		options:  options,
		cookie:   cookie,
		sameSite: sameSite,
		renewal:  renewal,
		now:      now,
		random:   random,
	}

	// The record cookie repeats the token cookie policy unless it was given one
	// of its own, so both expire under the same rules.
	recordCookie := options.RecordCookie
	if recordCookie.Name == "" {
		recordCookie.Name = DefaultDataCookieName
	}
	if recordCookie.Path == "" {
		recordCookie.Path = cookie.Path
		recordCookie.Domain = cookie.Domain
		recordCookie.Secure = cookie.Secure
		recordCookie.HTTPOnly = cookie.HTTPOnly
		recordCookie.SameSite = cookie.SameSite
	}
	if options.ServerSideAnonymous && backend != nil {
		manager.server = Typed[slotMap](backend, codec)
		manager.anon = manager.server
		manager.serverIsAnon = true
	} else if options.Keys != nil {
		anon, err := NewCookieStore(CookieStoreOptions{
			Keys:     options.Keys,
			Cookie:   recordCookie,
			MaxBytes: options.MaxBytes,
			Now:      now,
			Random:   random,
		})
		if err != nil {
			return nil, err
		}
		manager.anonRaw = anon
		manager.anon = Typed[slotMap](anon, codec)
	}
	switch {
	case manager.server != nil:
		// ServerSideAnonymous already selected the one store for both phases.
	case backend != nil:
		manager.server = Typed[slotMap](backend, codec)
	case manager.anon != nil:
		manager.server = manager.anon
		manager.serverIsAnon = true
	default:
		// No keyring and no backend: only Shared slots are registered, and they
		// never touch a record.
		manager.server = nil
	}
	_, memoryBackend := backend.(*MemoryStore)
	manager.lazyRecord = manager.serverIsAnon && (manager.anonRaw != nil || memoryBackend)

	for _, entry := range slots {
		if entry.newCookie == nil {
			continue
		}
		// A slot that stated no lifetime tracks the session, which is what
		// makes it die with it rather than at the next browser close.
		maxAge := options.TTL
		if entry.expiry > 0 {
			maxAge = entry.expiry
		}
		jar, err := entry.newCookie(JarOptions{
			Mode:     entry.placement.mode(),
			Keys:     options.Keys,
			Cookie:   slotCookie(cookie, entry),
			MaxAge:   maxAge,
			MaxBytes: options.MaxBytes,
			Now:      now,
			Random:   random,
		})
		if err != nil {
			return nil, fmt.Errorf("slot %q: %w", entry.key, err)
		}
		manager.jars[entry.typ] = jar
	}
	return manager, nil
}

// slotCookie gives a cookie-placed slot its own name under the session cookie
// policy. A Shared slot is readable by the front end, so it is never HttpOnly.
func slotCookie(base CookieOptions, entry *slot) CookieOptions {
	cookie := base
	cookie.Name = entry.key
	if entry.placement == Shared || entry.placement == ReadOnly {
		// The front end reads these, which an HttpOnly cookie forbids.
		cookie.HTTPOnly = false
	}
	return cookie
}

// CookieName reports the configured token cookie name.
func (m *Manager) CookieName() string { return m.cookie.Name }

// bind hands the request and response to a store that keeps its records in the
// browser. A backend store implements no binder and receives the context
// unchanged.
func (m *Manager) bind(ctx context.Context, store Store[slotMap], w http.ResponseWriter, r *http.Request) context.Context {
	binder, ok := store.(RequestBinder)
	if !ok {
		return ctx
	}
	return binder.BindRequest(ctx, w, r)
}

func (m *Manager) requestToken(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(m.cookie.Name)
	if err != nil || cookie == nil || !validToken(cookie.Value) {
		return "", false
	}
	return cookie.Value, true
}

// recordStore picks where the record of this request lives. The presence of the
// sealed record cookie is the marker: an anonymous session carries it, and a
// promoted session does not, because the promotion expired it.
func (m *Manager) recordStore(r *http.Request) (Store[slotMap], bool) {
	if m.anonRaw != nil && r != nil {
		if cookie, err := r.Cookie(m.anonRaw.CookieName()); err == nil && cookie != nil && cookie.Value != "" {
			return m.anon, true
		}
	}
	return m.server, false
}

func (m *Manager) writeCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	cookie := &http.Cookie{
		Name:     m.cookie.Name,
		Value:    token,
		Path:     m.cookie.Path,
		Domain:   m.cookie.Domain,
		Secure:   m.cookie.Secure,
		HttpOnly: m.cookie.HTTPOnly,
		SameSite: m.sameSite,
	}
	if !expiresAt.IsZero() {
		cookie.Expires = expiresAt
		cookie.MaxAge = int(expiresAt.Sub(m.now()).Seconds())
	}
	http.SetCookie(w, cookie)
}

func (m *Manager) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookie.Name,
		Value:    "",
		Path:     m.cookie.Path,
		Domain:   m.cookie.Domain,
		MaxAge:   -1,
		Secure:   m.cookie.Secure,
		HttpOnly: m.cookie.HTTPOnly,
		SameSite: m.sameSite,
	})
}

// newRecord stamps a record under the lifetime this manager was handed.
func (m *Manager) newRecord(values slotMap, now time.Time) Record[slotMap] {
	record := Record[slotMap]{
		Data:       values,
		CreatedAt:  now,
		LastSeenAt: now,
		Version:    m.options.Version,
	}
	if m.options.TTL > 0 {
		record.ExpiresAt = now.Add(m.options.TTL)
		if m.options.IdleTimeout > 0 {
			record.IdleExpiresAt = now.Add(m.options.IdleTimeout)
		}
	}
	return record
}

// deadline of a record with no lifetime source is zero, which writes a
// browser-session cookie.
func (m *Manager) deadlineOf(record Record[slotMap]) time.Time {
	if m.options.TTL <= 0 {
		return time.Time{}
	}
	return record.deadline()
}

var errNoRecordStore = errors.New("session: no record store; only Shared slots are registered")
