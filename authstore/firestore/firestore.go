// Package firestore holds the account-side authentication stores of
// plugin/auth in Firestore in Datastore mode.
//
// plugin/auth owns four framework kinds. The ceremony store is
// authstate/firestore; the other three are here: the admission allowlist, the
// passkey credentials, and the issued bootstrap credentials. Importing this
// package publishes all three kinds; the client itself belongs to
// database/firestore, which these stores read from the request context:
//
//	import _ "github.com/shibukawa/popcornweb/database/firestore"
//	import _ "github.com/shibukawa/popcornweb/authstore/firestore"
//
// A first passkey enrollment is one commit here. The bootstrap credential is
// consumed and the credential inserted inside one transaction, so neither can
// happen without the other; only the activation callback stays outside, because
// a contention abort re-runs the closure and an activation that ran twice is a
// side effect no transaction can bound. See Credentials.SaveFirstCredential.
//
// Nothing creates a kind. A kind exists once something writes to it, so there
// is no schema step and no startup table check; what a deployment still has to
// apply is the TTL policy on the bootstrap kind, which database/firestore Kinds
// publishes.
package firestore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shibukawa/popcornweb/database/firestore"
	"github.com/shibukawa/tinybind-go/firestorebind"
	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

// Declared kinds. They are the names plugin/auth already owns; a kind is
// intrinsic to the type, so nothing maps them onto another name.
const (
	DeclaredAllowlistKind  = "popcornweb_auth_allowlist"
	DeclaredCredentialKind = "popcornweb_passkey_credential"
	DeclaredBootstrapKind  = "popcornweb_auth_bootstrap"
)

func init() {
	firestore.RegisterKind(allowlistEntry{})
	firestore.RegisterKind(credentialEntity{})
	firestore.RegisterKind(bootstrapEntity{})
}

// txContextKey carries an open transaction to a store that has to join one.
//
// A first enrollment consumes the bootstrap credential and writes the
// credential together, and the two live in different stores, so the transaction
// reaches the second one through the context the framework already threads
// through its spend callback.
type txContextKey struct{}

func withTx(ctx context.Context, tx *firestorebind.Tx) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// txFrom returns the transaction this call has to join, if any.
func txFrom(ctx context.Context) (*firestorebind.Tx, bool) {
	tx, held := ctx.Value(txContextKey{}).(*firestorebind.Tx)
	return tx, held && tx != nil
}

// keyName renders a binary identifier as a key name. A Datastore key name is a
// string, and a credential ID is bytes, so it is encoded rather than coerced:
// the raw bytes are not valid UTF-8 and a lossy conversion would collide.
func keyName(raw []byte) string { return base64.RawURLEncoding.EncodeToString(raw) }

// decodeKeyName reverses keyName, for a decode that recovers the identifier
// from the key rather than from a property.
func decodeKeyName(name string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(name)
	if err != nil {
		return nil, fmt.Errorf("auth: stored key %q is not an encoded identifier: %w", name, err)
	}
	return raw, nil
}

// entitySize measures an entity before anything is sent, so a record over the
// limit is refused with the limit named rather than through a service error
// that does not say what the limit was.
//
// It measures the entity rather than the mutation, so it understates by the
// partition the client attaches and the commit envelope. Both are small and
// the check is a guard against a payload that is orders of magnitude over,
// not a byte-exact budget: datastore.Client.MutationSize is the exact figure
// and needs a client, which a validation running before one is resolved does
// not have.
func entitySize(encoder firestorebind.EntityEncoder) int {
	raw, err := json.Marshal(encoder.EncodeEntity())
	if err != nil {
		return 0
	}
	return len(raw)
}

// valueOf returns a property, or the zero value when it is absent.
func valueOf(stored datastore.Entity, name string) datastore.Value {
	value, _ := stored.Get(name)
	return value
}

func readString(stored datastore.Entity, name string) string {
	value, _ := valueOf(stored, name).AsString()
	return value
}

func readBytes(stored datastore.Entity, name string) []byte {
	value, _ := valueOf(stored, name).AsBytes()
	return value
}

func readInt(stored datastore.Entity, name string) int64 {
	value, _ := valueOf(stored, name).AsInt()
	return value
}

func readBool(stored datastore.Entity, name string) bool {
	value, _ := valueOf(stored, name).AsBool()
	return value
}

// readTime reads an optional timestamp. An absent or null property is the zero
// time, which is what "never" means for every timestamp here.
func readTime(stored datastore.Entity, name string) time.Time {
	value, present := stored.Get(name)
	if !present || value.IsNull() {
		return time.Time{}
	}
	at, held := value.AsTime()
	if !held {
		return time.Time{}
	}
	return at.UTC()
}

// unavailable names the missing middleware rather than repeating a driver error
// a reader cannot act on.
func unavailable(op string, err error) error {
	if errors.Is(err, firestorebind.ErrNoClient) {
		return fmt.Errorf(
			"auth: %s: no Datastore client in context; import database/firestore and enable middleware.firestore: %w",
			op, err)
	}
	return fmt.Errorf("auth: %s: %w", op, err)
}
