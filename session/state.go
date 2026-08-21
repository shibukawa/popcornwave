package session

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"
)

// promotedMarker records, inside the server record, that this session has been
// promoted and its Private slots therefore live on the server. Slot keys are
// restricted to cookie-name characters, so this key can never collide with one.
const promotedMarker = "!p"

// stateKey identifies the request-scoped session state.
type stateKey struct{}

// state is the session of one request: the token, where its records live, and
// the decoded value of every registered slot.
type state struct {
	manager *Manager
	carrier Carrier
	token   string
	record  Record[slotMap]
	// values and present are indexed by slot.index and allocated on the first
	// value, so a request that carries no session state allocates neither.
	values  []any
	present []bool
	// wire caches each record-placed slot's encoded payload, before any
	// expiry stamp, indexed like values. collect re-encodes only the slots
	// whose entry is nil: without it, one Slot.Set re-marshalled every
	// present slot in the bucket, and a handler writing two slots marshalled
	// the first one twice. A loaded slot's cached bytes also mean the bytes
	// written back are the bytes that were read, rather than a re-encoding
	// of the decoded value.
	wire [][]byte
	// detached carries the values of a WithValue state, which has no manager
	// and therefore no frozen slot order to index.
	detached map[reflect.Type]any
	promoted bool

	// hash caches keyHash(token), which several record operations need in one
	// request.
	hash string

	dirtyAnon    bool
	dirtyServer  bool
	failed       error
	recordLoaded bool
	recordErr    error
}

// slotValue returns the decoded value of one slot and whether the request
// carries it.
func (s *state) slotValue(entry *slot) (any, bool) {
	if s.manager == nil {
		value, ok := s.detached[entry.typ]
		return value, ok
	}
	if s.present == nil || !s.present[entry.index] {
		return nil, false
	}
	return s.values[entry.index], true
}

func (s *state) setSlotValue(entry *slot, value any) {
	if s.values == nil {
		s.values = make([]any, len(s.manager.slots))
		s.present = make([]bool, len(s.manager.slots))
		s.wire = make([][]byte, len(s.manager.slots))
	}
	s.values[entry.index] = value
	s.present[entry.index] = true
	// The cached encoding described the previous value; the next collect
	// re-encodes this one.
	s.wire[entry.index] = nil
}

func (s *state) clearSlotValue(entry *slot) {
	if s.present == nil {
		return
	}
	s.values[entry.index] = nil
	s.present[entry.index] = false
	s.wire[entry.index] = nil
}

// setSlotWire records the payload the current value is known to encode to —
// the bytes it was loaded from, or the bytes the last collect produced.
func (s *state) setSlotWire(entry *slot, encoded []byte) {
	if s.wire == nil {
		return
	}
	s.wire[entry.index] = encoded
}

func (s *state) slotWire(entry *slot) []byte {
	if s.wire == nil {
		return nil
	}
	return s.wire[entry.index]
}

// cookie returns the value of one request cookie. An empty value reads as
// absent, exactly as r.Cookie treats a cookie the request does not carry.
//
// The carrier parses the Cookie header once, which is what keeps the token and
// every jar-backed slot from re-parsing it per lookup.
func (s *state) cookie(name string) (string, bool) {
	if found := lookupCookie(s.carrier.Cookies(), name); found != nil && found.Value != "" {
		return found.Value, true
	}
	return "", false
}

// tokenHash returns keyHash(token), computed once per token.
func (s *state) tokenHash() string {
	if s.hash == "" && s.token != "" {
		s.hash = keyHash(s.token)
	}
	return s.hash
}

// bucket is where one slot's bytes go right now. A cookie-placed slot has its
// own cookie, and a RequestScope slot exists only in this request's memory, so
// neither is in a record.
type bucket int

const (
	bucketNone bucket = iota
	bucketAnon
	bucketServer
)

func (s *state) bucketOf(entry *slot) bucket {
	switch {
	case !entry.placement.recordPlaced():
		return bucketNone
	case s.manager.options.ServerSideAnonymous:
		return bucketServer
	case entry.placement == ServerOnly:
		return bucketServer
	case s.promoted:
		return bucketServer
	default:
		return bucketAnon
	}
}

// Slot is the request-scoped handle to one registered piece of state.
type Slot[T any] struct {
	state *state
	entry *slot
}

// Get returns the current value and whether the request carried a usable one.
func (s *Slot[T]) Get() (T, bool) {
	var zero T
	if s == nil || s.state == nil {
		return zero, false
	}
	if s.state.manager != nil && s.entry.placement.recordPlaced() {
		_ = s.state.resolveRecord()
	}
	held, ok := s.state.slotValue(s.entry)
	if !ok {
		return zero, false
	}
	value, ok := held.(T)
	if !ok {
		return zero, false
	}
	return value, true
}

// Set writes value and makes it what later handlers in this request read.
//
// A cookie-placed slot writes Set-Cookie immediately and therefore precedes
// response commitment. A record-placed slot is flushed once, before the
// response is committed. The first Set on any slot issues the session token, so
// a visitor who writes nothing receives no cookie and occupies no storage.
func (s *Slot[T]) Set(value T) error {
	if s == nil || s.state == nil {
		return fmt.Errorf("%w: no session on request", ErrInvalidOptions)
	}
	return s.state.set(s.entry, value)
}

// Clear removes the value and makes the request look like it carried none.
func (s *Slot[T]) Clear() error {
	if s == nil || s.state == nil {
		return fmt.Errorf("%w: no session on request", ErrInvalidOptions)
	}
	return s.state.clear(s.entry)
}

// Value returns the request-scoped handle for T. It reports false when the
// session middleware did not run or T is not registered.
func Value[T any](ctx context.Context) (*Slot[T], bool) {
	if ctx == nil {
		return nil, false
	}
	current, ok := ctx.Value(stateKey{}).(*state)
	if !ok || current == nil {
		return nil, false
	}
	typ := reflect.TypeFor[T]()
	if current.manager == nil {
		// A detached state installed by WithValue: readable, and refusing every
		// write because there is no browser and no store behind it.
		return &Slot[T]{state: current, entry: &slot{key: typ.String(), typ: typ}}, true
	}
	entry, ok := current.manager.registry.lookup(typ)
	if !ok {
		return nil, false
	}
	return &Slot[T]{state: current, entry: entry}, true
}

// WithValue returns ctx carrying value as the slot for T, exactly as the
// middleware would have resolved it, without a manager behind it.
//
// It is the seam a test uses to run a handler against a session it did not have
// to establish. A slot installed this way is read-only: Set and Clear refuse,
// because nothing here reaches a browser or a store.
func WithValue[T any](ctx context.Context, value T) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	next := &state{detached: map[reflect.Type]any{}}
	if current, ok := ctx.Value(stateKey{}).(*state); ok && current != nil && current.manager == nil {
		// Extend a detached state rather than shadowing it, so several values
		// installed in sequence are all readable.
		for typ, held := range current.detached {
			next.detached[typ] = held
		}
	}
	next.detached[reflect.TypeFor[T]()] = value
	return context.WithValue(ctx, stateKey{}, next)
}

// Load returns the value of the registered slot for T and whether the request
// carried one.
func Load[T any](ctx context.Context) (T, bool) {
	handle, ok := Value[T](ctx)
	if !ok {
		var zero T
		return zero, false
	}
	return handle.Get()
}

func (s *state) set(entry *slot, value any) error {
	if s.failed != nil {
		return s.failed
	}
	if s.manager == nil {
		return fmt.Errorf("%w: this session was installed by WithValue and is read-only", ErrInvalidOptions)
	}
	if entry.placement == RequestScope {
		// Nothing leaves this request: no cookie, no token, no record. The
		// value dies when the request does, which is the placement's promise.
		s.setSlotValue(entry, value)
		return nil
	}
	if entry.placement.cookiePlaced() {
		jar, ok := s.manager.jars[entry.typ]
		if !ok {
			return fmt.Errorf("%w: slot %q has no cookie", ErrInvalidOptions, entry.key)
		}
		if err := jar.save(s.carrier, value); err != nil {
			return err
		}
		s.setSlotValue(entry, value)
		return nil
	}
	if err := s.resolveRecord(); err != nil && !staleSessionError(err) {
		return err
	}
	if err := s.ensureToken(); err != nil {
		return err
	}
	s.setSlotValue(entry, value)
	s.markDirty(s.bucketOf(entry))
	// Flushing here rather than at commit keeps a refused oversized write an
	// error the caller sees, instead of a silent loss after the handler ran.
	return s.flush()
}

func (s *state) clear(entry *slot) error {
	if s.manager == nil {
		return fmt.Errorf("%w: this session was installed by WithValue and is read-only", ErrInvalidOptions)
	}
	if entry.placement == RequestScope {
		s.clearSlotValue(entry)
		return nil
	}
	if entry.placement.cookiePlaced() {
		if jar, ok := s.manager.jars[entry.typ]; ok {
			jar.clear(s.carrier)
		}
		s.clearSlotValue(entry)
		return nil
	}
	if err := s.resolveRecord(); err != nil && !staleSessionError(err) {
		return err
	}
	if _, ok := s.slotValue(entry); !ok {
		return nil
	}
	s.clearSlotValue(entry)
	if s.token == "" {
		return nil
	}
	s.markDirty(s.bucketOf(entry))
	return s.flush()
}

func (s *state) markDirty(where bucket) {
	switch where {
	case bucketAnon:
		s.dirtyAnon = true
	case bucketServer:
		s.dirtyServer = true
	}
}

// ensureToken issues the token on the first write, so a bare read costs the
// browser and the server nothing.
func (s *state) ensureToken() error {
	if s.token != "" {
		return nil
	}
	if s.manager.server == nil && s.manager.anon == nil {
		return errNoRecordStore
	}
	token, err := newToken(s.manager.random)
	if err != nil {
		return fmt.Errorf("%w: token", ErrUnavailable)
	}
	s.token = token
	s.hash = ""
	s.record = s.manager.newRecord(nil, s.manager.now())
	s.manager.writeCookie(s.carrier, token, s.manager.deadlineOf(s.record))
	return nil
}

// collect encodes the slots of one bucket.
func (s *state) collect(where bucket) (slotMap, error) {
	values := slotMap{}
	now := s.manager.now()
	for _, entry := range s.manager.slots {
		if s.bucketOf(entry) != where {
			continue
		}
		held, ok := s.slotValue(entry)
		if !ok {
			continue
		}
		encoded := s.slotWire(entry)
		if encoded == nil {
			fresh, err := entry.encode(held)
			if err != nil {
				return nil, fmt.Errorf("slot %q: %w", entry.key, err)
			}
			encoded = fresh
			s.setSlotWire(entry, fresh)
		}
		if entry.expiry > 0 {
			// A slot may die before the session that carries it, so it carries
			// its own deadline inside the record. The stamp is applied per
			// collect, over the cached payload, so the deadline keeps sliding
			// exactly as it did when the slot was re-encoded each time.
			encoded = stampSlot(now.Add(entry.expiry), encoded)
		}
		values[entry.key] = encoded
	}
	if where == bucketServer && s.promoted {
		values[promotedMarker] = []byte{1}
	}
	return values, nil
}

// flush writes every dirty record. It replaces each placement atomically; a
// write spanning both fails the request rather than committing one of them.
func (s *state) flush() error {
	if s.token == "" || (!s.dirtyAnon && !s.dirtyServer) {
		return nil
	}
	if s.dirtyAnon {
		if err := s.put(bucketAnon, s.manager.anon); err != nil {
			return err
		}
		s.dirtyAnon = false
	}
	if s.dirtyServer {
		if err := s.put(bucketServer, s.manager.server); err != nil {
			return err
		}
		s.dirtyServer = false
	}
	return nil
}

func (s *state) put(where bucket, store Store[slotMap]) error {
	if store == nil {
		return errNoRecordStore
	}
	values, err := s.collect(where)
	if err != nil {
		return err
	}
	record := s.record
	record.Data = values
	if len(values) == 0 {
		// Nothing left in this placement: drop the record rather than keeping an
		// empty one alive.
		return store.Delete(s.bindTo(store), s.tokenHash())
	}
	return store.Put(s.bindTo(store), s.tokenHash(), record)
}

func (s *state) bindTo(store Store[slotMap]) context.Context {
	return s.manager.bind(s.carrier.Context(), store, s.carrier)
}

// destroy revokes every record and expires every cookie this session owns.
func (s *state) destroy() error {
	var failure error
	if s.token != "" {
		hash := s.tokenHash()
		for _, store := range s.manager.stores() {
			if err := store.Delete(s.bindTo(store), hash); err != nil && failure == nil {
				failure = err
			}
		}
	}
	s.manager.clearCookie(s.carrier)
	for _, entry := range s.manager.slots {
		if entry.outlives {
			// The slot belongs to the browser rather than to whoever was signed
			// in, which is what declaring OutlivesSession states.
			continue
		}
		if jar, ok := s.manager.jars[entry.typ]; ok {
			jar.clear(s.carrier)
		}
	}
	s.token = ""
	s.hash = ""
	for _, entry := range s.manager.slots {
		if entry.placement == RequestScope {
			// The value was derived from this request rather than stored by the
			// session, so the destruction of the session has nothing of its to
			// take back.
			continue
		}
		s.clearSlotValue(entry)
	}
	s.dirtyAnon, s.dirtyServer = false, false
	s.record = Record[slotMap]{}
	return failure
}

// stores returns the distinct record stores, so a destroy reaches both
// placements without deleting twice where they are the same store.
func (m *Manager) stores() []Store[slotMap] {
	switch {
	case m.server == nil && m.anon == nil:
		return nil
	case m.anon == nil:
		return []Store[slotMap]{m.server}
	case m.server == nil, m.serverIsAnon:
		return []Store[slotMap]{m.anon}
	default:
		return []Store[slotMap]{m.server, m.anon}
	}
}

// load reads the records this request carries and decodes every slot.
//
// A session may hold two records at once: the sealed cookie holding its Private
// slots while it is anonymous, and the server record holding its ServerOnly
// slots. A promotion merges the first into the second, so a value found on the
// server wins over a record cookie the browser may still be presenting.
func (s *state) load() error {
	token, ok := s.cookie(s.manager.cookie.Name)
	if !ok || !validToken(token) {
		return nil
	}
	s.token = token
	s.hash = ""
	hash := s.tokenHash()

	server, serverOK, err := s.read(s.manager.server, hash)
	if err != nil {
		s.token, s.hash = "", ""
		return err
	}
	anon, anonOK := Record[slotMap]{}, false
	if s.manager.anon != nil && !s.manager.serverIsAnon {
		anon, anonOK, err = s.read(s.manager.anon, hash)
		if err != nil && !errors.Is(err, ErrExpired) {
			s.token, s.hash = "", ""
			return err
		}
	}
	if !serverOK && !anonOK {
		s.token, s.hash = "", ""
		return ErrNotFound
	}

	// The common session holds one record, whose map is then read directly; a
	// merged copy is built only when both placements answered.
	var values slotMap
	if anonOK {
		s.record = anon
		values = anon.Data
	}
	if serverOK {
		s.record = server
		_, s.promoted = server.Data[promotedMarker]
		values = server.Data
	}
	if anonOK && serverOK {
		values = make(slotMap, len(anon.Data)+len(server.Data))
		for key, encoded := range anon.Data {
			values[key] = encoded
		}
		// A value found on the server wins over a record cookie the browser
		// may still be presenting.
		for key, encoded := range server.Data {
			values[key] = encoded
		}
	}
	for _, entry := range s.manager.slots {
		if !entry.placement.recordPlaced() {
			// A cookie-placed slot has its own cookie, and a RequestScope slot
			// starts every request empty; neither reads from the record, so a
			// stale record key can never populate one.
			continue
		}
		encoded, ok := values[entry.key]
		if !ok {
			continue
		}
		if entry.expiry > 0 {
			// Past its own deadline a slot reads as absent, exactly as one that
			// was never written.
			if encoded, ok = unstampSlot(encoded, s.manager.now()); !ok {
				continue
			}
		}
		value, err := entry.decode(encoded)
		if err != nil {
			// One unreadable slot is cleared rather than failing the record.
			continue
		}
		s.setSlotValue(entry, value)
		// The bytes just decoded are the slot's encoding; a later flush of
		// another slot in the bucket writes them back rather than
		// re-marshalling the value.
		s.setSlotWire(entry, encoded)
	}
	return nil
}

// resolveRecord loads the record at most once. Cookie-backed deployments call
// it from their first record operation; backend deployments call it before the
// handler so an unavailable store still fails closed.
func (s *state) resolveRecord() error {
	if s.recordLoaded {
		return s.recordErr
	}
	s.recordLoaded = true
	s.recordErr = s.load()
	if staleSessionError(s.recordErr) {
		s.manager.clearCookie(s.carrier)
	}
	if s.recordErr != nil && !staleSessionError(s.recordErr) {
		s.failed = s.recordErr
	}
	return s.recordErr
}

// loadCookieSlots decodes each cookie-placed slot from its own cookie. A value
// this jar did not write, or one past its expiry, is cleared and continues as
// absent: stale client state is not an error the application has to handle.
func (s *state) loadCookieSlots() {
	for _, entry := range s.manager.slots {
		jar, ok := s.manager.jars[entry.typ]
		if !ok {
			continue
		}
		// The jar cookie is named after the slot key, and the shared parse
		// spares one Cookie-header scan per slot.
		raw, _ := s.cookie(entry.key)
		switch value, err := jar.load(raw); {
		case err == nil:
			s.setSlotValue(entry, value)
		case !errors.Is(err, ErrCookieMissing):
			jar.clear(s.carrier)
		}
	}
}

// read loads and renews one record.
func (s *state) read(store Store[slotMap], hash string) (Record[slotMap], bool, error) {
	if store == nil {
		return Record[slotMap]{}, false, nil
	}
	ctx := s.bindTo(store)
	record, err := store.Get(ctx, hash)
	switch {
	case errors.Is(err, ErrNotFound):
		return Record[slotMap]{}, false, nil
	case err != nil:
		return Record[slotMap]{}, false, err
	}
	if record.Version != s.manager.options.Version {
		_ = store.Delete(ctx, hash)
		return Record[slotMap]{}, false, ErrExpired
	}
	now := s.manager.now()
	// The record's own deadline decides, not the configured TTL. Gating on TTL
	// meant a session bounded only by an idle timeout was never checked here.
	if deadline := record.deadline(); !deadline.IsZero() && !deadline.After(now) {
		_ = store.Delete(ctx, hash)
		return Record[slotMap]{}, false, ErrExpired
	}
	if renewed, ok := s.renew(store, ctx, hash, record, now); ok {
		record = renewed
	}
	return record, true, nil
}

// renew touches the record only after RenewalInterval and never extends it
// beyond the absolute expiry.
func (s *state) renew(store Store[slotMap], ctx context.Context, hash string, record Record[slotMap], now time.Time) (Record[slotMap], bool) {
	options := s.manager.options
	if options.IdleTimeout <= 0 || s.manager.renewal <= 0 {
		return record, false
	}
	if now.Sub(record.LastSeenAt) < s.manager.renewal {
		return record, false
	}
	idleExpiresAt := now.Add(options.IdleTimeout)
	if !record.ExpiresAt.IsZero() && idleExpiresAt.After(record.ExpiresAt) {
		// Renewal never reaches past the absolute expiry. With no absolute
		// expiry there is nothing to clamp against.
		idleExpiresAt = record.ExpiresAt
	}
	if err := touchStore(store, ctx, hash, record, now, idleExpiresAt); err != nil {
		return record, false
	}
	record.LastSeenAt = now
	record.IdleExpiresAt = idleExpiresAt
	s.manager.writeCookie(s.carrier, s.token, s.manager.deadlineOf(record))
	return record, true
}

// touchStore renews through TouchRecord where the store offers it — the
// renewal follows the Get that produced record, so a backend that can write
// from it skips re-reading what this request just read — and through Touch
// otherwise.
func touchStore[T any](store Store[T], ctx context.Context, hash string, record Record[T], lastSeenAt, idleExpiresAt time.Time) error {
	if toucher, ok := store.(RecordToucher[T]); ok {
		handled, err := toucher.TouchRecord(ctx, hash, record, lastSeenAt, idleExpiresAt)
		if handled {
			return err
		}
	}
	return store.Touch(ctx, hash, lastSeenAt, idleExpiresAt)
}
