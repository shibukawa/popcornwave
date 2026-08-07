// Package dynamo stores login sessions in DynamoDB.
//
// It exists so a deployment with no relational database can still log a user
// in. Importing it registers the backend and the session table; the client
// itself belongs to database/dynamo, which this package reads from the request
// context:
//
//	import _ "github.com/shibukawa/popcornwave/database/dynamo"
//	import _ "github.com/shibukawa/popcornwave/sessionstore/dynamo"
//
// Nothing here sweeps expired records. A record is judged expired when it is
// read and cannot be renewed once dead, so correctness never waits for a
// deletion. Removing the bytes is DynamoDB TTL on the dead_at attribute, which
// a deployment enables on the table it owns.
package dynamo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shibukawa/popcornwave/database/dynamo"
	"github.com/shibukawa/popcornwave/session"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// DeclaredTable is the name source uses. A deployment maps it onto its own
// through middleware.dynamo, like any other table.
const DeclaredTable = "popcornwave_session"

// Attribute names. keyAttribute is named once and used by both the table
// definition and the item, so the two cannot drift.
const (
	keyAttribute             = "key_hash"
	dataAttribute            = "data"
	createdAtAttribute       = "created_at"
	authenticatedAtAttribute = "authenticated_at"
	lastSeenAtAttribute      = "last_seen_at"
	expiresAtAttribute       = "expires_at"
	idleExpiresAtAttribute   = "idle_expires_at"
	deadAtAttribute          = "dead_at"
	methodAttribute          = "method"
	versionAttribute         = "version"
)

// maxItemBytes is the DynamoDB item limit. A record over it is refused here,
// with the limit named, rather than through a service validation error that
// does not say what the limit was.
const maxItemBytes = 400 * 1024

// Table is the definition of the session table. It is handwritten rather than
// generated: this package is the only reader and writer of the item, so the
// drift a generated codec closes cannot occur, and generating for one internal
// type would put a code generation step in the framework's own build.
func Table(name string) dynamodb.TableDefinition {
	return dynamodb.TableDefinition{
		Name:         name,
		PartitionKey: dynamodb.KeyAttribute{Name: keyAttribute, Type: dynamodb.TypeString},
	}
}

// Options configures a store.
type Options struct {
	// Table is the declared table name. Empty means DeclaredTable.
	Table string
	// ConsistentRead makes the first read strongly consistent and removes the
	// retry described on Get. It costs twice the read capacity.
	ConsistentRead bool
	// Now is injectable for tests.
	Now func() time.Time
}

// Store is the DynamoDB session backend. It implements session.RawStore, so it
// never sees the application payload type: the host adds that back with
// session.Typed.
type Store struct {
	table   string
	strong  bool
	nowFunc func() time.Time
}

var _ session.RawStore = (*Store)(nil)

// NewStore builds a store. It opens nothing: the client comes from the request
// context, installed by the database/dynamo middleware.
func NewStore(options Options) *Store {
	table := options.Table
	if table == "" {
		table = DeclaredTable
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Store{table: table, strong: options.ConsistentRead, nowFunc: now}
}

func (store *Store) now() time.Time { return store.nowFunc().UTC() }

// resolve returns the client and the deployed table name. Resolution happens
// through the process handle database/dynamo owns, so this store never builds
// a deployed name itself and reads no context value.
func (store *Store) resolve(ctx context.Context) (*dynamodb.Client, string, error) {
	handle, err := dynamo.Handle(ctx)
	if err != nil {
		return nil, "", fmt.Errorf(
			"%w: no DynamoDB client available; import database/dynamo and enable middleware.dynamo",
			session.ErrUnavailable)
	}
	client, table, err := handle.Table(ctx, store.table)
	if err != nil {
		return nil, "", fmt.Errorf(
			"%w: no DynamoDB client available; import database/dynamo and enable middleware.dynamo",
			session.ErrUnavailable)
	}
	return client, table, nil
}

// Put replaces one key. PutItem is atomic per item, so no condition is needed.
func (store *Store) Put(ctx context.Context, keyHash string, record session.RawRecord) error {
	if keyHash == "" {
		return fmt.Errorf("%w: empty key", session.ErrInvalidKey)
	}
	client, table, err := store.resolve(ctx)
	if err != nil {
		return err
	}
	// The payload arrives already encoded: session.Typed owns the codec, so a
	// backend never sees the application type.
	encoded := record.Payload
	if size := len(encoded); size > maxItemBytes {
		return fmt.Errorf("%w: session payload is %d bytes, over the DynamoDB item limit of %d",
			session.ErrCodec, size, maxItemBytes)
	}
	item := dynamodb.Item{
		keyAttribute:             dynamodb.S(keyHash),
		dataAttribute:            dynamodb.B(encoded),
		createdAtAttribute:       epoch(record.CreatedAt),
		authenticatedAtAttribute: epoch(record.AuthenticatedAt),
		lastSeenAtAttribute:      epoch(record.LastSeenAt),
		expiresAtAttribute:       epoch(record.ExpiresAt),
		deadAtAttribute:          dynamodb.N(record.Deadline().Unix()),
		methodAttribute:          dynamodb.S(record.Method),
		versionAttribute:         dynamodb.N(record.Version),
	}
	if !record.IdleExpiresAt.IsZero() {
		item[idleExpiresAtAttribute] = epoch(record.IdleExpiresAt)
	}
	if _, err := client.PutItem(ctx, table, item); err != nil {
		return unavailable(err)
	}
	return nil
}

// Get returns the stored record.
//
// The read is eventually consistent, and a miss is retried once with a
// consistent read. A hit is never re-read, so the common authenticated request
// pays the cheap read; the case the retry exists for is the false miss right
// after a login rotation, which a consistent read cannot produce.
func (store *Store) Get(ctx context.Context, keyHash string) (session.RawRecord, error) {
	if keyHash == "" {
		return session.RawRecord{}, fmt.Errorf("%w: empty key", session.ErrInvalidKey)
	}
	client, table, err := store.resolve(ctx)
	if err != nil {
		return session.RawRecord{}, err
	}
	key := dynamodb.Key{keyAttribute: dynamodb.S(keyHash)}

	item, err := client.GetItem(ctx, table, key, dynamodb.WithConsistentRead(store.strong))
	if errors.Is(err, dynamodb.ErrItemNotFound) && !store.strong {
		item, err = client.GetItem(ctx, table, key, dynamodb.WithConsistentRead(true))
	}
	switch {
	case errors.Is(err, dynamodb.ErrItemNotFound):
		return session.RawRecord{}, session.ErrNotFound
	case err != nil:
		return session.RawRecord{}, unavailable(err)
	}

	record, err := store.decode(item)
	if err != nil {
		return session.RawRecord{}, err
	}
	// Expiry is decided here rather than trusted to a deletion: DynamoDB TTL
	// is asynchronous and documented as taking up to two days, so a store that
	// waited for it would serve expired sessions for that long.
	if !store.now().Before(record.Deadline()) {
		return session.RawRecord{}, session.ErrExpired
	}
	return record, nil
}

// Touch renews an existing record in one conditional request. The condition
// carries the whole guarantee: the item must exist and still be alive, so no
// read-then-write window exists for a concurrent delete to fall into.
func (store *Store) Touch(ctx context.Context, keyHash string, lastSeenAt, idleExpiresAt time.Time) error {
	if keyHash == "" {
		return fmt.Errorf("%w: empty key", session.ErrInvalidKey)
	}
	client, table, err := store.resolve(ctx)
	if err != nil {
		return err
	}
	values := map[string]dynamodb.AttributeValue{
		":last_seen": dynamodb.N(lastSeenAt.UTC().Unix()),
		":now":       dynamodb.N(store.now().Unix()),
	}
	names := map[string]string{
		"#last_seen": lastSeenAtAttribute,
		"#dead":      deadAtAttribute,
		"#key":       keyAttribute,
	}
	update := "SET #last_seen = :last_seen"
	if !idleExpiresAt.IsZero() {
		// The caller has already clamped the renewal to the absolute expiry,
		// so dead_at is the new idle expiry outright. A DynamoDB update
		// expression has no conditional operator to clamp it with anyway.
		values[":idle"] = dynamodb.N(idleExpiresAt.UTC().Unix())
		names["#idle"] = idleExpiresAtAttribute
		update += ", #idle = :idle, #dead = :idle"
	}
	condition := "attribute_exists(#key) AND #dead > :now"

	_, err = client.UpdateItem(ctx, table, dynamodb.Key{keyAttribute: dynamodb.S(keyHash)}, update,
		dynamodb.WithCondition(condition),
		dynamodb.WithExpressionNames(names),
		dynamodb.WithExpressionValues(values))
	switch {
	case errors.Is(err, dynamodb.ErrConditionalCheck):
		// The item is gone or already dead. Either way the contract says a
		// renewal must not revive it.
		return session.ErrNotFound
	case err != nil:
		return unavailable(err)
	}
	return nil
}

// Delete removes one record. DeleteItem succeeds on an absent key, which is the
// idempotence the contract asks for.
func (store *Store) Delete(ctx context.Context, keyHash string) error {
	if keyHash == "" {
		return fmt.Errorf("%w: empty key", session.ErrInvalidKey)
	}
	client, table, err := store.resolve(ctx)
	if err != nil {
		return err
	}
	if _, err := client.DeleteItem(ctx, table, dynamodb.Key{keyAttribute: dynamodb.S(keyHash)}); err != nil {
		return unavailable(err)
	}
	return nil
}

// decode rebuilds a record. A malformed item is a codec failure rather than a
// partially filled record.
func (store *Store) decode(item dynamodb.Item) (session.RawRecord, error) {
	encoded, held := item[dataAttribute].AsBytes()
	if !held {
		return session.RawRecord{}, fmt.Errorf("%w: %s", session.ErrCodec, dataAttribute)
	}
	version, held := item[versionAttribute].AsInt()
	if !held {
		return session.RawRecord{}, fmt.Errorf("%w: %s", session.ErrCodec, versionAttribute)
	}
	method, _ := item[methodAttribute].AsString()
	record := session.RawRecord{
		Payload:         encoded,
		CreatedAt:       readEpoch(item, createdAtAttribute),
		AuthenticatedAt: readEpoch(item, authenticatedAtAttribute),
		LastSeenAt:      readEpoch(item, lastSeenAtAttribute),
		ExpiresAt:       readEpoch(item, expiresAtAttribute),
		IdleExpiresAt:   readEpoch(item, idleExpiresAtAttribute),
		Method:          method,
		Version:         int(version),
	}
	if record.ExpiresAt.IsZero() {
		return session.RawRecord{}, fmt.Errorf("%w: %s", session.ErrCodec, expiresAtAttribute)
	}
	return record, nil
}

// epoch stores a timestamp as second-precision number, the only form DynamoDB
// TTL reads and the form every timestamp here uses for consistency.
func epoch(value time.Time) dynamodb.AttributeValue {
	if value.IsZero() {
		return dynamodb.N(0)
	}
	return dynamodb.N(value.UTC().Unix())
}

func readEpoch(item dynamodb.Item, name string) time.Time {
	value, present := item[name]
	if !present {
		return time.Time{}
	}
	seconds, held := value.AsInt()
	if !held || seconds == 0 {
		return time.Time{}
	}
	return time.Unix(seconds, 0).UTC()
}

// unavailable maps a driver failure onto the contract error while keeping the
// driver sentinel reachable through errors.Is.
func unavailable(err error) error {
	return fmt.Errorf("%w: %w", session.ErrUnavailable, err)
}
