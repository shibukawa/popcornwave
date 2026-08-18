package firestore

import (
	"context"
	"errors"

	"github.com/shibukawa/popcornweb/database/firestore"
	"github.com/shibukawa/popcornweb/pw"
	"github.com/shibukawa/popcornweb/session"
)

// Importing this package registers the firestore session backend and publishes
// its kind:
//
//	import _ "github.com/shibukawa/popcornweb/database/firestore"
//	import _ "github.com/shibukawa/popcornweb/sessionstore/firestore"
//
// Registration opens nothing. The client belongs to database/firestore, which
// installs it into every request context, so this backend borrows one it did
// not open and hands back no Close.
//
// The kind is registered rather than created. Nothing creates a kind on this
// store — the first write is the creation — so what the registration publishes
// is the list a deployment needs in order to apply the TTL policy.
func init() {
	pw.RegisterSessionBackend(pw.SessionBackendFirestore, open)
	firestore.RegisterKind(entity{})
}

// open builds the store. It verifies that the client is reachable now rather
// than answering the first login with a backend failure, which is the same
// promise the other backends make by dialing at startup.
func open(ctx context.Context, config pw.SessionConfig, _ pw.SessionResources) (session.Backend, error) {
	// A setup context is not a request context, so the client is read from the
	// middleware that opened it one slot earlier rather than from the chain.
	if _, opened := firestore.EnsureClient(ctx); !opened {
		return session.Backend{}, errors.New(
			`session.backend = "firestore" requires middleware.firestore.enabled and the ` +
				`github.com/shibukawa/popcornweb/database/firestore import`)
	}
	store := NewStore(Options{Kind: config.Firestore.Kind})
	// No Close, because the client is the host's. No Prune, because nothing
	// here sweeps: a record is judged expired when it is read, and removing the
	// bytes is a TTL policy on the kind a deployment owns.
	return session.Backend{Store: store}, nil
}
