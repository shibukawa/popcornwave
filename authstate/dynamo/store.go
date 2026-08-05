// Package dynamo stores single-use authentication ceremony state in DynamoDB.
//
// It exists so a deployment with no relational database can run the passkey,
// OAuth, and OIDC ceremonies. Importing it registers the ceremony table; the
// client itself belongs to database/dynamo, which this package reads from the
// request context:
//
//	import _ "github.com/shibukawa/popcornwave/database/dynamo"
//	import _ "github.com/shibukawa/popcornwave/authstate/dynamo"
//
// Nothing here sweeps expired records. Take decides expiry from the stored
// deadline, so correctness never waits for a deletion; removing the bytes is
// DynamoDB TTL on the expires_at attribute, which a deployment enables on the
// table it owns. The sqlite adapter publishes Prune because a bounded DELETE is
// cheap there. Here the equivalent is a Scan over every ceremony record ever
// written, which costs more than the records it would remove.
package dynamo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/authstate"
	"github.com/shibukawa/popcornwave/database/dynamo"
	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// DeclaredTable is the name source uses. A deployment maps it onto its own
// through middleware.dynamo, like any other table.
const DeclaredTable = "popcornwave_authstate"

// Attribute names. keyAttribute is named once and used by both the table
// definition and the item, so the two cannot drift.
const (
	keyAttribute       = "state_key"
	payloadAttribute   = "payload"
	expiresAtAttribute = "expires_at"
	deadlineAttribute  = "expires_at_ms"
)

const (
	defaultMaxKeyBytes   = 256
	defaultMaxValueBytes = 64 << 10
	hardMaxKeyBytes      = 4096
	hardMaxValueBytes    = 1 << 20
	maxNamespaceBytes    = 128
)

// maxItemBytes is the DynamoDB item limit. A record over it is refused here,
// with the limit named, rather than through a service validation error that
// does not say what the limit was.
const maxItemBytes = 400 * 1024

func init() {
	dynamo.RegisterTable(DeclaredTable, Table)
}

// Table is the definition of the ceremony table. It is handwritten rather than
// generated: this package is the only reader and writer of the item, so the
// drift a generated codec closes cannot occur.
//
// The partition key holds the namespace and the correlation key joined, rather
// than the namespace alone with the key as a sort key. Every ceremony of one
// protocol shares a namespace, so that shape would put a login spike on a
// single partition. What it gives up is a cheap namespace-scoped listing, which
// nothing needs once there is no prune.
func Table(name string) dynamodb.TableDefinition {
	return dynamodb.TableDefinition{
		Name:         name,
		PartitionKey: dynamodb.KeyAttribute{Name: keyAttribute, Type: dynamodb.TypeString},
	}
}

// Options controls key isolation and resource bounds.
type Options struct {
	// Table is the declared table name. Empty means DeclaredTable.
	Table string
	// Namespace separates the ceremonies of one protocol from another's within
	// the one table. It is required.
	Namespace string
	// Now is injectable for tests.
	Now           func() time.Time
	MaxKeyBytes   int
	MaxValueBytes int
}

// Store persists expiring, single-use authentication state in DynamoDB.
//
// It is a raw store: it works on already encoded payloads, so plugin/auth can
// open one for a ceremony type this package cannot name. NewStore puts the
// codec back on for a caller that has one.
type Store struct {
	table         string
	namespace     string
	now           func() time.Time
	maxKeyBytes   int
	maxValueBytes int
}

var _ authstate.RawStore = (*Store)(nil)

// NewStore builds a typed store over NewRawStore, for a caller that owns a
// codec and wants the ordinary contract.
func NewStore[T any](codec authstate.Codec[T], options Options) (authstate.Store[T], error) {
	if codec == nil {
		return nil, authstate.ErrInvalidOptions
	}
	raw, err := NewRawStore(options)
	if err != nil {
		return nil, err
	}
	return authstate.Typed[T](raw, codec), nil
}

// NewRawStore builds the store. It opens nothing: the client comes from the
// request context, installed by the database/dynamo middleware. That is why it
// takes no client argument, unlike the sqlite adapter which takes a pool.
func NewRawStore(options Options) (*Store, error) {
	if !validNamespace(options.Namespace) ||
		options.MaxKeyBytes < 0 || options.MaxKeyBytes > hardMaxKeyBytes ||
		options.MaxValueBytes < 0 || options.MaxValueBytes > hardMaxValueBytes {
		return nil, authstate.ErrInvalidOptions
	}
	table := options.Table
	if table == "" {
		table = DeclaredTable
	}
	maxKeyBytes := options.MaxKeyBytes
	if maxKeyBytes == 0 {
		maxKeyBytes = defaultMaxKeyBytes
	}
	maxValueBytes := options.MaxValueBytes
	if maxValueBytes == 0 {
		maxValueBytes = defaultMaxValueBytes
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Store{
		table: table, namespace: options.Namespace, now: now,
		maxKeyBytes: maxKeyBytes, maxValueBytes: maxValueBytes,
	}, nil
}

// resolve returns the client and the deployed table name. Resolution happens
// inside tinybind, so this store never builds a deployed name itself.
func (s *Store) resolve(ctx context.Context) (*dynamodb.Client, string, error) {
	client, table, err := dynamobind.TableFromContext(ctx, s.table)
	if err != nil {
		return nil, "", fmt.Errorf(
			"%w: no DynamoDB client in context; import database/dynamo and enable middleware.dynamo",
			authstate.ErrUnavailable)
	}
	return client, table, nil
}

// itemKey joins the namespace and the correlation key. The separator is a
// character validNamespace forbids, so no namespace and key pair can collide
// with another.
func (s *Store) itemKey(key string) string {
	return s.namespace + ":" + key
}

// Put stores a value that has never been stored under this key, or whose
// previous record has already expired.
//
// The condition carries the whole guarantee in one request: the item is absent,
// or its stored deadline has passed. That is the sqlite adapter's rule reached
// without a read followed by a conditional upsert.
func (s *Store) Put(ctx context.Context, key string, payload []byte, expiresAt time.Time) error {
	if !s.ready() || ctx == nil {
		return authstate.ErrInvalidOptions
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if key == "" || len(key) > s.maxKeyBytes {
		return authstate.ErrInvalidKey
	}
	now := s.now()
	if expiresAt.IsZero() || !expiresAt.After(now) || expiresAt.UnixMilli() <= 0 {
		return authstate.ErrInvalidExpiry
	}
	if len(payload) == 0 {
		return fmt.Errorf("%w: encode", authstate.ErrCodec)
	}
	if len(payload) > s.maxValueBytes {
		return authstate.ErrLimitExceeded
	}
	if len(payload) > maxItemBytes {
		return fmt.Errorf("%w: ceremony payload is %d bytes, over the DynamoDB item limit of %d",
			authstate.ErrLimitExceeded, len(payload), maxItemBytes)
	}
	client, table, err := s.resolve(ctx)
	if err != nil {
		return err
	}
	item := dynamodb.Item{
		keyAttribute:     dynamodb.S(s.itemKey(key)),
		payloadAttribute: dynamodb.B(payload),
		// The contract carries a millisecond deadline and DynamoDB TTL reads
		// seconds, so both are stored: the deadline decides correctness and
		// the second-precision copy is what a deployment points TTL at.
		deadlineAttribute:  dynamodb.N(expiresAt.UnixMilli()),
		expiresAtAttribute: dynamodb.N(expiresAt.Unix()),
	}
	_, err = client.PutItem(ctx, table, item,
		dynamodb.WithCondition("attribute_not_exists(#key) OR #deadline <= :now"),
		dynamodb.WithExpressionNames(map[string]string{
			"#key":      keyAttribute,
			"#deadline": deadlineAttribute,
		}),
		dynamodb.WithExpressionValues(map[string]dynamodb.AttributeValue{
			":now": dynamodb.N(now.UnixMilli()),
		}))
	switch {
	case errors.Is(err, dynamodb.ErrConditionalCheck):
		// A live record already holds this key. An expired one would have been
		// replaced, which is what the second half of the condition is for.
		return authstate.ErrAlreadyExists
	case err != nil:
		return unavailable(ctx, err)
	}
	return nil
}

// Take removes a value and returns it, atomically.
//
// DeleteItem asking for the old item removes and returns in one request, so the
// single-use guarantee needs no read followed by a delete. This is the same
// shape as the sqlite adapter's DELETE RETURNING, and the reason both adapters
// can promise it.
//
// Everything after the removal is validation of what came back. A malformed,
// expired, or undecodable record stays consumed: it was single-use, and handing
// it back would be worse than losing it.
func (s *Store) Take(ctx context.Context, key string) ([]byte, error) {
	if !s.ready() || ctx == nil {
		return nil, authstate.ErrInvalidOptions
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if key == "" || len(key) > s.maxKeyBytes {
		return nil, authstate.ErrInvalidKey
	}
	client, table, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	result, err := client.DeleteItem(ctx, table,
		dynamodb.Key{keyAttribute: dynamodb.S(s.itemKey(key))},
		dynamodb.WithReturnValues("ALL_OLD"))
	if err != nil {
		return nil, unavailable(ctx, err)
	}
	// DeleteItem succeeds on an absent key, so an empty ALL_OLD is the miss.
	if result == nil || len(result.Attributes) == 0 {
		return nil, authstate.ErrNotFound
	}
	deadline, held := result.Attributes[deadlineAttribute].AsInt()
	if !held || deadline <= 0 {
		return nil, fmt.Errorf("%w: malformed record", authstate.ErrCodec)
	}
	if !time.UnixMilli(deadline).After(s.now()) {
		return nil, authstate.ErrExpired
	}
	payload, held := result.Attributes[payloadAttribute].AsBytes()
	if !held || len(payload) == 0 {
		return nil, fmt.Errorf("%w: malformed record", authstate.ErrCodec)
	}
	if len(payload) > s.maxValueBytes {
		return nil, authstate.ErrLimitExceeded
	}
	return payload, nil
}

func (s *Store) ready() bool {
	return s != nil && s.now != nil && s.table != "" && s.namespace != "" &&
		s.maxKeyBytes > 0 && s.maxValueBytes > 0
}

func validNamespace(value string) bool {
	if value == "" || len(value) > maxNamespaceBytes || strings.Contains(value, ":") {
		return false
	}
	for i := range len(value) {
		if value[i] < 0x21 || value[i] > 0x7e {
			return false
		}
	}
	return true
}

// unavailable maps a driver failure onto the contract error while keeping the
// driver sentinel reachable through errors.Is. A cancelled caller is reported
// as its own error rather than as a backend outage.
func unavailable(ctx context.Context, err error) error {
	if cause := ctx.Err(); cause != nil {
		return cause
	}
	return fmt.Errorf("%w: %w", authstate.ErrUnavailable, err)
}
