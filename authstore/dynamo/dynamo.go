// Package dynamo holds the account-side authentication stores of plugin/auth
// in DynamoDB.
//
// plugin/auth owns four framework tables. The ceremony store is
// authstate/dynamo; the other three are here: the admission allowlist, the
// passkey credentials, and the issued bootstrap credentials. Importing this
// package registers all three tables; the client itself belongs to
// database/dynamo, which these stores read from the request context:
//
//	import _ "github.com/shibukawa/popcornwave/database/dynamo"
//	import _ "github.com/shibukawa/popcornwave/authstore/dynamo"
//
// The driver has no TransactWriteItems, so a first passkey enrollment cannot be
// one unit of work here. CredentialStore fixes an order instead: the bootstrap
// credential is spent first, then the credential is written, then the account
// is activated. Every partial outcome of that order is safe, and none of them
// lets one issued secret enroll two authenticators. See SaveFirstCredential.
package dynamo

import (
	"context"
	"fmt"
	"time"

	"github.com/shibukawa/popcornwave/database/dynamo"
	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// Declared table names. They are the names plugin/auth already owns; a
// deployment maps them onto its own through middleware.dynamo.
const (
	DeclaredAllowlistTable  = "popcornwave_auth_allowlist"
	DeclaredCredentialTable = "popcornwave_passkey_credential"
	DeclaredBootstrapTable  = "popcornwave_auth_bootstrap"
)

// maxItemBytes is the DynamoDB item limit. A record over it is refused here,
// with the limit named, rather than through a service validation error that
// does not say what the limit was.
const maxItemBytes = 400 * 1024

func init() {
	dynamo.RegisterTable(DeclaredAllowlistTable, AllowlistTable)
	dynamo.RegisterTable(DeclaredCredentialTable, CredentialTable)
	dynamo.RegisterTable(DeclaredBootstrapTable, BootstrapTable)
}

// resolve returns the client and the deployed table name. Resolution happens
// inside tinybind, so no store here builds a deployed name itself.
func resolve(ctx context.Context, declared string) (*dynamodb.Client, string, error) {
	client, table, err := dynamobind.TableFromContext(ctx, declared)
	if err != nil {
		return nil, "", fmt.Errorf(
			"auth: no DynamoDB client in context; import database/dynamo and enable middleware.dynamo: %w", err)
	}
	return client, table, nil
}

// epoch stores a timestamp as a second-precision number, the only form
// DynamoDB TTL reads and the form every timestamp here uses for consistency.
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

func readString(item dynamodb.Item, name string) string {
	value, _ := item[name].AsString()
	return value
}

func readBytes(item dynamodb.Item, name string) []byte {
	value, _ := item[name].AsBytes()
	return value
}

func readInt(item dynamodb.Item, name string) int64 {
	value, _ := item[name].AsInt()
	return value
}

func readBool(item dynamodb.Item, name string) bool {
	value, _ := item[name].AsBool()
	return value
}
