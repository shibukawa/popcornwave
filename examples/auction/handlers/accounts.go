package handlers

import (
	"context"

	"auction/queries"
	"github.com/shibukawa/popcornweb/plugin/auth"
)

// RegisterAccounts installs the account seams. Call it from main before
// pw.Run.
func RegisterAccounts() {
	auth.SetAccountResolver(resolveAccount)
}

// resolveAccount answers with the account behind a verified identity.
//
// This starter derives one instead of storing it, which is enough to log in
// and read the user. Replace it with a lookup against your own table as soon
// as the application owns accounts: the link is the issuer plus the verified
// claim auth.oidc.identity_claim selected, never the email address.
func resolveAccount(ctx context.Context, identity auth.Identity, provision bool) (auth.Account, error) {
	displayName, _ := identity.Claims.String("name")
	if displayName == "" {
		displayName = identity.Key
	}
	email, _ := identity.Claims.String("email")
	account := auth.Account{
		ID:          identity.Issuer + "|" + identity.Key,
		DisplayName: displayName,
		Email:       email,
	}
	if _, err := queries.UpsertAccount(ctx, account.ID, account.DisplayName, account.Email); err != nil {
		return auth.Account{}, err
	}
	return account, nil
}
