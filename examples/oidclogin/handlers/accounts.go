package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"oidclogin/queries"

	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/pw"
)

// RegisterAccountResolver installs the application account resolver. Call it
// from main before pw.Run: the framework verifies the OIDC identity and then
// asks this function which local account it belongs to.
func RegisterAccountResolver() { auth.SetAccountResolver(resolveAccount) }

// resolveAccount links a verified identity to a local account, and creates one
// when policy permits provisioning.
//
// The link is the issuer plus the claim auth.oidc.identity_claim selected and
// its verified value; the display name and email are refreshed copies with no
// authority over the link.
func resolveAccount(ctx context.Context, identity auth.Identity, provision bool) (auth.Account, error) {
	displayName, _ := identity.Claims.String("name")
	if displayName == "" {
		displayName = identity.Key
	}
	email, _ := identity.Claims.String("email")

	existing, err := queries.FindAccount(ctx, identity.Issuer, identity.KeyClaim, identity.Key)
	if err != nil {
		return auth.Account{}, err
	}
	if existing != nil {
		if existing.Display_name != displayName || existing.Email != email {
			if _, err := queries.UpdateAccountProfile(ctx, existing.Id, displayName, email); err != nil {
				return auth.Account{}, err
			}
		}
		return auth.Account{ID: existing.Id, DisplayName: displayName, Email: email}, nil
	}
	if !provision {
		// Admission decides what an unknown identity means; the resolver only
		// reports that no account exists.
		return auth.Account{}, auth.ErrUnknownIdentity
	}

	id, err := newAccountID()
	if err != nil {
		return auth.Account{}, err
	}
	// The account row and its identity link are written together, so a failed
	// login never leaves an unreachable account behind.
	err = pw.Transaction(ctx, func(ctx context.Context) error {
		if _, err := queries.InsertAccount(ctx, id, displayName, email); err != nil {
			return err
		}
		_, err := queries.LinkIdentity(ctx, identity.Issuer, identity.KeyClaim, identity.Key, id)
		return err
	})
	if err != nil {
		return auth.Account{}, err
	}
	return auth.Account{ID: id, DisplayName: displayName, Email: email}, nil
}

// newAccountID returns an opaque identifier. An email address or a database
// sequence must never become the account key.
func newAccountID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}
