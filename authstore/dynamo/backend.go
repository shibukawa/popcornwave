package dynamo

import (
	"context"

	"github.com/shibukawa/popcornwave/authstate"
	statedynamo "github.com/shibukawa/popcornwave/authstate/dynamo"
	"github.com/shibukawa/popcornwave/plugin/auth"
)

func init() {
	auth.RegisterBackend(auth.BackendDynamo, open)
}

// open supplies the four stores plugin/auth owns, over DynamoDB.
//
// It opens nothing and verifies nothing: every store reads its client from the
// request context, and the tables are created by pw migrate from the
// definitions this package registers. Startup still refuses to serve when a
// table is absent, which the migration check answers rather than this one.
func open(_ context.Context, config auth.Config, _ auth.Resources) (auth.Backend, error) {
	return auth.Backend{
		OpenState: func(_ context.Context, namespace string) (authstate.RawStore, error) {
			return statedynamo.NewRawStore(statedynamo.Options{Namespace: namespace})
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
// a deployment on another admission mode is never asked for the table.
func allowlistFor(config auth.Config) auth.AllowlistStore {
	if config.OIDC.Admission != auth.AdmissionRegistered {
		return nil
	}
	return NewAllowlist()
}
