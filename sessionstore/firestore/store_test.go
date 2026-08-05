package firestore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/internal/firestoretest"
	"github.com/shibukawa/popcornwave/session"
	"github.com/shibukawa/tinybind-go/firestorebind"
	"github.com/shibukawa/tinygodriver/cloud/google"
	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

// newStore starts a fake, installs a client for it, and returns both.
func newStore(t *testing.T, options ...func(*Options)) (context.Context, *Store, *firestoretest.Server) {
	t.Helper()
	fake := firestoretest.New(t)
	client, err := datastore.New("demo",
		datastore.WithEndpoint(fake.Endpoint()),
		datastore.WithTokenSource(google.StaticTokenSource(google.Token{Value: "test"})),
		datastore.WithRetry(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	settings := Options{}
	for _, option := range options {
		option(&settings)
	}
	return firestorebind.WithClient(context.Background(), client), NewStore(settings), fake
}

func record(now time.Time) session.RawRecord {
	return session.RawRecord{
		Payload:       []byte(`{"cart":3}`),
		CreatedAt:     now,
		LastSeenAt:    now,
		ExpiresAt:     now.Add(time.Hour),
		IdleExpiresAt: now.Add(30 * time.Minute),
		Method:        "passkey",
		Version:       1,
	}
}

func TestPutGetRoundTrip(t *testing.T) {
	ctx, store, _ := newStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	if err := store.Put(ctx, "hash", record(now)); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, "hash")
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Payload) != `{"cart":3}` {
		t.Errorf("payload: got %q", got.Payload)
	}
	if got.Method != "passkey" || got.Version != 1 {
		t.Errorf("method/version: got %q, %d", got.Method, got.Version)
	}
	// Datastore stores microseconds, so the comparison is at that resolution
	// rather than the nanosecond one the record was built with.
	if !got.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Errorf("expires_at: got %s, want %s", got.ExpiresAt, now.Add(time.Hour))
	}
	if !got.IdleExpiresAt.Equal(now.Add(30 * time.Minute)) {
		t.Errorf("idle_expires_at: got %s", got.IdleExpiresAt)
	}
}

// The payload must never be indexed. An indexed value over 1500 bytes is stored
// without an index rather than refused, so a property that is sometimes indexed
// would be worse than one that never is.
func TestThePayloadIsStoredUnindexed(t *testing.T) {
	stored := entity{keyHash: "hash", record: record(time.Now())}.EncodeEntity()
	value, held := stored.Get(dataProperty)
	if !held {
		t.Fatal("no payload property")
	}
	if !value.ExcludeFromIndexes {
		t.Error("the payload is indexed; it must be excluded")
	}
}

func TestGetReportsAMissAsNotFound(t *testing.T) {
	ctx, store, _ := newStore(t)
	if _, err := store.Get(ctx, "absent"); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

// Expiry is decided on read. A TTL policy removes an entity within about a day
// of its deadline, so a store that waited for the deletion would serve expired
// sessions for that long.
func TestGetRefusesAnExpiredRecordBeforeAnythingDeletesIt(t *testing.T) {
	now := time.Now().UTC()
	clock := now
	ctx, store, _ := newStore(t, func(o *Options) { o.Now = func() time.Time { return clock } })

	if err := store.Put(ctx, "hash", record(now)); err != nil {
		t.Fatal(err)
	}
	clock = now.Add(2 * time.Hour)
	if _, err := store.Get(ctx, "hash"); !errors.Is(err, session.ErrExpired) {
		t.Fatalf("got %v, want ErrExpired", err)
	}
}

func TestTouchRenewsTheIdleDeadline(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	ctx, store, _ := newStore(t, func(o *Options) { o.Now = func() time.Time { return now } })

	if err := store.Put(ctx, "hash", record(now)); err != nil {
		t.Fatal(err)
	}
	renewed := now.Add(45 * time.Minute)
	if err := store.Touch(ctx, "hash", now.Add(time.Minute), renewed); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, "hash")
	if err != nil {
		t.Fatal(err)
	}
	if !got.IdleExpiresAt.Equal(renewed) {
		t.Errorf("idle_expires_at: got %s, want %s", got.IdleExpiresAt, renewed)
	}
	if !got.LastSeenAt.Equal(now.Add(time.Minute)) {
		t.Errorf("last_seen_at: got %s", got.LastSeenAt)
	}
	// The renewal must not disturb the payload it had to rewrite.
	if string(got.Payload) != `{"cart":3}` {
		t.Errorf("payload changed under a renewal: %q", got.Payload)
	}
}

func TestTouchNeverRevivesAnExpiredRecord(t *testing.T) {
	now := time.Now().UTC()
	clock := now
	ctx, store, _ := newStore(t, func(o *Options) { o.Now = func() time.Time { return clock } })

	if err := store.Put(ctx, "hash", record(now)); err != nil {
		t.Fatal(err)
	}
	clock = now.Add(2 * time.Hour)
	if err := store.Touch(ctx, "hash", clock, clock.Add(time.Hour)); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestTouchOnAMissingRecordDoesNotCreateOne(t *testing.T) {
	ctx, store, fake := newStore(t)
	now := time.Now().UTC()
	if err := store.Touch(ctx, "absent", now, now.Add(time.Hour)); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
	if count := fake.Len(DeclaredKind); count != 0 {
		t.Errorf("a renewal created %d entities", count)
	}
}

// The version precondition is what a condition expression would be elsewhere: a
// record rotated or deleted between the read and the write is at a different
// version, so the renewal is refused rather than recreating it.
func TestTouchLosingARaceReportsNotFoundInsteadOfRewriting(t *testing.T) {
	now := time.Now().UTC()
	ctx, store, fake := newStore(t, func(o *Options) { o.Now = func() time.Time { return now } })

	if err := store.Put(ctx, "hash", record(now)); err != nil {
		t.Fatal(err)
	}
	// Between this renewal's read and its write, another request rotates the
	// same session.
	fake.BeforeCommit(func() {
		rotated := record(now)
		rotated.Payload = []byte(`{"cart":99}`)
		if err := store.Put(ctx, "hash", rotated); err != nil {
			t.Errorf("racing write: %v", err)
		}
	})
	err := store.Touch(ctx, "hash", now.Add(time.Minute), now.Add(45*time.Minute))
	if !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
	got, err := store.Get(ctx, "hash")
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Payload) != `{"cart":99}` {
		t.Errorf("the losing renewal overwrote the winner: %q", got.Payload)
	}
}

func TestDeleteIsIdempotent(t *testing.T) {
	ctx, store, _ := newStore(t)
	if err := store.Put(ctx, "hash", record(time.Now())); err != nil {
		t.Fatal(err)
	}
	for attempt := range 2 {
		if err := store.Delete(ctx, "hash"); err != nil {
			t.Fatalf("delete %d: %v", attempt, err)
		}
	}
	if _, err := store.Get(ctx, "hash"); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

// A record over the entity limit is refused here, with the limit named, rather
// than through a service error that does not say what the limit was.
func TestPutRefusesARecordOverTheEntityLimit(t *testing.T) {
	ctx, store, fake := newStore(t)
	oversized := record(time.Now())
	oversized.Payload = make([]byte, datastore.MaxEntityBytes+1)

	err := store.Put(ctx, "hash", oversized)
	if !errors.Is(err, session.ErrCodec) {
		t.Fatalf("got %v, want ErrCodec", err)
	}
	if !strings.Contains(err.Error(), "1048572") {
		t.Errorf("the limit is not named: %v", err)
	}
	if fake.Calls("commit") != 0 {
		t.Error("an oversized record was sent to the service")
	}
}

func TestAnEmptyKeyIsRefusedWithoutARequest(t *testing.T) {
	ctx, store, fake := newStore(t)
	now := time.Now()
	if err := store.Put(ctx, "", record(now)); !errors.Is(err, session.ErrInvalidKey) {
		t.Errorf("put: got %v", err)
	}
	if _, err := store.Get(ctx, ""); !errors.Is(err, session.ErrInvalidKey) {
		t.Errorf("get: got %v", err)
	}
	if err := store.Touch(ctx, "", now, now); !errors.Is(err, session.ErrInvalidKey) {
		t.Errorf("touch: got %v", err)
	}
	if err := store.Delete(ctx, ""); !errors.Is(err, session.ErrInvalidKey) {
		t.Errorf("delete: got %v", err)
	}
	if fake.Calls("lookup")+fake.Calls("commit") != 0 {
		t.Error("an empty key reached the service")
	}
}

// A store reached without the middleware fails as an ordinary error naming the
// import, rather than panicking.
func TestWithoutAClientTheStoreIsUnavailable(t *testing.T) {
	store := NewStore(Options{})
	_, err := store.Get(context.Background(), "hash")
	if !errors.Is(err, session.ErrUnavailable) {
		t.Fatalf("got %v, want ErrUnavailable", err)
	}
	if !strings.Contains(err.Error(), "database/firestore") {
		t.Errorf("the error does not name the import: %v", err)
	}
}

func TestABackendFailureIsUnavailable(t *testing.T) {
	ctx, store, fake := newStore(t)
	fake.FailNext("PERMISSION_DENIED")
	if _, err := store.Get(ctx, "hash"); !errors.Is(err, session.ErrUnavailable) {
		t.Fatalf("got %v, want ErrUnavailable", err)
	}
}

// The kind is published for the deployment that has to point a TTL policy at
// it, and the property comes from the same declaration the codec reads.
func TestTheExpiryPropertyIsDeclaredOnTheRecordType(t *testing.T) {
	property, expires := entity{}.ExpiryProperty()
	if !expires || property != deadAtProperty {
		t.Fatalf("ExpiryProperty: got %q, %v", property, expires)
	}
	stored := entity{keyHash: "hash", record: record(time.Now())}.EncodeEntity()
	if _, held := stored.Get(property); !held {
		t.Errorf("the declared expiry property %q is not written", property)
	}
}

// Two test servers on different namespaces never see each other's sessions,
// which is the whole of the isolation mechanism.
func TestTwoNamespacesDoNotObserveEachOther(t *testing.T) {
	fake := firestoretest.New(t)
	contexts := map[string]context.Context{}
	for _, namespace := range []string{"run-a", "run-b"} {
		client, err := datastore.New("demo",
			datastore.WithEndpoint(fake.Endpoint()),
			datastore.WithNamespace(namespace),
			datastore.WithTokenSource(google.StaticTokenSource(google.Token{Value: "test"})),
			datastore.WithRetry(1, 0))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = client.Close() })
		contexts[namespace] = firestorebind.WithClient(context.Background(), client)
	}

	store := NewStore(Options{})
	now := time.Now().UTC()
	first := record(now)
	first.Payload = []byte(`{"run":"a"}`)
	if err := store.Put(contexts["run-a"], "shared", first); err != nil {
		t.Fatal(err)
	}

	if _, err := store.Get(contexts["run-b"], "shared"); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("run-b saw run-a's session: %v", err)
	}
	got, err := store.Get(contexts["run-a"], "shared")
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Payload) != `{"run":"a"}` {
		t.Errorf("payload: got %q", got.Payload)
	}
}
