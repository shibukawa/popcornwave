package firestore

import (
	"context"
	"errors"

	"github.com/shibukawa/popcornweb/authstate"
	statefirestore "github.com/shibukawa/popcornweb/authstate/firestore"
	"github.com/shibukawa/popcornweb/database/firestore"
	"github.com/shibukawa/popcornweb/plugin/auth"
)

func init() {
	auth.RegisterBackend(auth.BackendFirestore, open)
}

// open supplies the four stores plugin/auth owns, over Firestore.
//
// Every store reads its client from the request context, so this opens nothing
// of its own. It does check that the client exists: there is no kind to verify
// and no schema to compare, so a missing middleware is the one startup mistake
// this backend can still catch, and catching it here is better than answering
// the first login with a backend failure.
func open(ctx context.Context, config auth.Config, _ auth.Resources) (auth.Backend, error) {
	if _, opened := firestore.EnsureClient(ctx); !opened {
		return auth.Backend{}, errors.New(
			`auth.backend = "firestore" requires middleware.firestore.enabled and the ` +
				`github.com/shibukawa/popcornweb/database/firestore import`)
	}
	return auth.Backend{
		OpenState: func(_ context.Context, namespace string) (authstate.RawStore, error) {
			return statefirestore.NewRawStore(statefirestore.Options{Namespace: namespace})
		},
		Allowlist: allowlistFor(config),
		// The credential and bootstrap stores are supplied whichever mode is
		// selected, because plugin/auth already skips them outside a passkey
		// mode and an unused store costs one struct.
		Credentials: NewCredentials(CredentialOptions{}),
		Bootstrap:   NewBootstrap(),
	}, nil
}

// allowlistFor supplies the admission store only for the mode that reads it, so
// a deployment on another admission mode is never asked for the kind.
func allowlistFor(config auth.Config) auth.AllowlistStore {
	if config.OIDC.Admission != auth.AdmissionRegistered {
		return nil
	}
	return NewAllowlist()
}
