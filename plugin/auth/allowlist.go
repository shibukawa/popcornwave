package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Allowlist reads the pre-registration table owned by this package.
type Allowlist struct {
	db *sql.DB
}

// registered reports whether the verified identity appears in the allowlist.
//
// Each compared claim is read from the verified claim set and matched as one
// (issuer, claim, value) row. The configured identity claim is compared by
// default, because that is the value a deployment knows in advance. Listing
// further claims lets a deployment also recognize someone it registered by
// another attribute, such as an email address.
func (a Allowlist) registered(ctx context.Context, claims []string, identity Identity) (bool, error) {
	if a.db == nil {
		return false, errors.New("auth: allowlist is not available")
	}
	if len(claims) == 0 {
		claims = []string{identity.KeyClaim}
	}
	conditions := make([]string, 0, len(claims))
	arguments := make([]any, 0, len(claims)*2+1)
	arguments = append(arguments, identity.Issuer)
	for _, claim := range claims {
		value, ok := claimLookupValue(claim, identity)
		if !ok {
			continue
		}
		conditions = append(conditions, `(claim = ? AND value = ?)`)
		arguments = append(arguments, claim, value)
	}
	if len(conditions) == 0 {
		return false, nil
	}
	query := `SELECT 1 FROM ` + AllowlistTable + ` WHERE issuer = ? AND (` +
		strings.Join(conditions, " OR ") + `) LIMIT 1`
	var found int
	err := a.db.QueryRowContext(ctx, query, arguments...).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		// A backend failure must not be reported as "not registered", which
		// would turn an outage into a silent access change.
		return false, fmt.Errorf("auth: read allowlist: %w", err)
	}
	return true, nil
}
