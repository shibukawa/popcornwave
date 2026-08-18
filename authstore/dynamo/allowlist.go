package dynamo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/shibukawa/popcornweb/plugin/auth"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

const allowlistKeyAttribute = "entry"

// AllowlistTable is the definition of the admission allowlist table.
//
// The partition key holds the issuer, the claim name, and the value joined,
// rather than the issuer alone with the claim and value as a sort key. A
// deployment usually has one issuer, so that shape would put every login's
// admission read on one partition. What it gives up is an issuer-scoped
// listing, which an administrator answers with a Scan: the table is bounded by
// how many identities an operator registered, and listing them is an
// administrative operation rather than a per-request one.
func AllowlistTable(name string) dynamodb.TableDefinition {
	return dynamodb.TableDefinition{
		Name:         name,
		PartitionKey: dynamodb.KeyAttribute{Name: allowlistKeyAttribute, Type: dynamodb.TypeString},
	}
}

// Allowlist reads the pre-registration entries an operator provisioned. It
// writes nothing: provisioning is administrator tooling.
type Allowlist struct{}

var _ auth.AllowlistStore = Allowlist{}

// NewAllowlist builds the store. It opens nothing: the client comes from the
// request context, installed by the database/dynamo middleware.
func NewAllowlist() Allowlist { return Allowlist{} }

// allowlistEntry joins the three key fields. The separator is a byte no
// registrable claim name may contain and that a value carrying it cannot forge
// a different entry with, because the claim name is compared too.
func allowlistEntry(issuer, claim, value string) string {
	return issuer + "\x1f" + claim + "\x1f" + value
}

// Registered answers the whole question in one BatchGetItem, so a login costs
// one round trip however many claims a deployment compares. Any returned item
// is a match.
func (Allowlist) Registered(ctx context.Context, issuer string, candidates []auth.AllowlistCandidate) (bool, error) {
	if len(candidates) == 0 {
		return false, nil
	}
	client, table, err := resolve(ctx, DeclaredAllowlistTable)
	if err != nil {
		return false, err
	}
	keys := make([]dynamodb.Key, 0, len(candidates))
	for _, candidate := range candidates {
		keys = append(keys, dynamodb.Key{
			allowlistKeyAttribute: dynamodb.S(allowlistEntry(issuer, candidate.Claim, candidate.Value)),
		})
	}
	result, err := client.BatchGetItem(ctx, map[string][]dynamodb.Key{table: keys})
	if err != nil {
		// A backend failure must not be reported as "not registered", which
		// would turn an outage into a silent access change.
		return false, fmt.Errorf("auth: read allowlist: %w", err)
	}
	if result == nil {
		return false, errors.New("auth: read allowlist: empty response")
	}
	if len(result.UnprocessedKeys[table]) > 0 {
		// An incomplete answer is not a non-match: the unread key might be the
		// one that would have admitted this login.
		return false, fmt.Errorf("auth: read allowlist: %d of %d entries went unread",
			len(result.UnprocessedKeys[table]), len(keys))
	}
	return len(result.Items[table]) > 0, nil
}

// EntryKey returns the partition key of one allowlist entry, so administrator
// tooling provisioning a registration writes the same key this store reads.
func EntryKey(issuer, claim, value string) dynamodb.Key {
	return dynamodb.Key{
		allowlistKeyAttribute: dynamodb.S(allowlistEntry(issuer, claim, value)),
	}
}

// Entry builds the item of one allowlist registration, including the note an
// operator records beside it.
func Entry(issuer, claim, value, note string) dynamodb.Item {
	item := dynamodb.Item{
		allowlistKeyAttribute: dynamodb.S(allowlistEntry(issuer, claim, value)),
		"issuer":              dynamodb.S(issuer),
		"claim":               dynamodb.S(claim),
		"value":               dynamodb.S(value),
	}
	if strings.TrimSpace(note) != "" {
		item["note"] = dynamodb.S(note)
	}
	return item
}
