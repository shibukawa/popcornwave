package firestore

import (
	"context"
	"fmt"
	"strings"

	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/tinybind-go/firestorebind"
	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

// Allowlist property names.
const (
	allowlistIssuerProperty = "issuer"
	allowlistClaimProperty  = "claim"
	allowlistValueProperty  = "value"
	allowlistNoteProperty   = "note"
)

// allowlistEntry is one pre-registration an operator provisioned.
//
// The key holds the issuer, the claim name, and the value joined, rather than
// making the issuer an ancestor. A deployment usually has one issuer, so an
// ancestor path would put every login's admission read into a single entity
// group. What joining gives up is an issuer-scoped listing, which an
// administrator answers with a query over the kind: the kind is bounded by how
// many identities an operator registered, and listing them is administrative
// rather than per-request.
type allowlistEntry struct {
	issuer string
	claim  string
	value  string
	note   string
}

var (
	_ firestorebind.EntityEncoder = allowlistEntry{}
	_ firestorebind.EntityDecoder = (*allowlistEntry)(nil)
	_ firestorebind.Keyer         = allowlistEntry{}
	_ firestorebind.Kinder        = allowlistEntry{}
)

func (allowlistEntry) Kind() string { return DeclaredAllowlistKind }

func (e allowlistEntry) EntityKey() datastore.Key {
	return datastore.NameKey(DeclaredAllowlistKind, entryName(e.issuer, e.claim, e.value))
}

func (e allowlistEntry) EncodeEntity() datastore.Entity {
	out := datastore.NewEntity(e.EntityKey()).
		Set(allowlistIssuerProperty, datastore.String(e.issuer)).
		Set(allowlistClaimProperty, datastore.String(e.claim)).
		Set(allowlistValueProperty, datastore.String(e.value))
	if strings.TrimSpace(e.note) != "" {
		out = out.Set(allowlistNoteProperty, datastore.String(e.note))
	}
	return out
}

func (e *allowlistEntry) DecodeEntity(stored datastore.Entity) error {
	e.issuer = readString(stored, allowlistIssuerProperty)
	e.claim = readString(stored, allowlistClaimProperty)
	e.value = readString(stored, allowlistValueProperty)
	e.note = readString(stored, allowlistNoteProperty)
	return nil
}

// entryName joins the three key fields. The separator is a byte no registrable
// claim name may contain and that a value carrying it cannot forge a different
// entry with, because the claim name is compared too.
func entryName(issuer, claim, value string) string {
	return issuer + "\x1f" + claim + "\x1f" + value
}

// Allowlist reads the pre-registration entries an operator provisioned. It
// writes nothing: provisioning is administrator tooling.
type Allowlist struct{}

var _ auth.AllowlistStore = Allowlist{}

// NewAllowlist builds the store. It opens nothing: the client comes from the
// request context, installed by the database/firestore middleware.
func NewAllowlist() Allowlist { return Allowlist{} }

// Registered answers the whole question in one lookup, so a login costs one
// round trip however many claims a deployment compares. Any found entry is a
// match.
func (Allowlist) Registered(ctx context.Context, issuer string, candidates []auth.AllowlistCandidate) (bool, error) {
	if len(candidates) == 0 {
		return false, nil
	}
	keys := make([]datastore.Key, 0, len(candidates))
	for _, candidate := range candidates {
		keys = append(keys, datastore.NameKey(
			DeclaredAllowlistKind, entryName(issuer, candidate.Claim, candidate.Value)))
	}
	found, _, deferred, err := firestorebind.LoadAll[allowlistEntry](ctx, keys)
	if err != nil {
		// A backend failure must not be reported as "not registered", which
		// would turn an outage into a silent access change.
		return false, unavailable("read allowlist", err)
	}
	if len(deferred) > 0 {
		// An incomplete answer is not a non-match: the unread key might be the
		// one that would have admitted this login.
		return false, fmt.Errorf("auth: read allowlist: %d of %d entries went unread",
			len(deferred), len(keys))
	}
	return len(found) > 0, nil
}

// EntryKey returns the key of one allowlist entry, so administrator tooling
// provisioning a registration writes the same key this store reads.
func EntryKey(issuer, claim, value string) datastore.Key {
	return datastore.NameKey(DeclaredAllowlistKind, entryName(issuer, claim, value))
}

// Entry builds one allowlist registration, including the note an operator
// records beside it. Store it with firestorebind.Store.
func Entry(issuer, claim, value, note string) firestorebind.EntityEncoder {
	return allowlistEntry{issuer: issuer, claim: claim, value: value, note: note}
}
