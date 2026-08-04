package dynamo

import (
	"context"
	"errors"

	"github.com/shibukawa/popcornwave/database/dynamo"
	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/popcornwave/session"
)

// Importing this package registers the dynamo session backend and puts the
// session table into the desired schema:
//
//	import _ "github.com/shibukawa/popcornwave/database/dynamo"
//	import _ "github.com/shibukawa/popcornwave/sessionstore/dynamo"
//
// Registration opens nothing. The client belongs to database/dynamo, which
// installs it into every request context, so this backend borrows one it did
// not open and hands back no Close.
func init() {
	pw.RegisterSessionBackend(pw.SessionBackendDynamo, open)
	dynamo.RegisterTable(DeclaredTable, Table)
}

// open builds the store. It verifies that the client is reachable now rather
// than answering the first login with a backend failure, which is the same
// promise the other backends make by dialing at startup.
func open(ctx context.Context, config pw.SessionConfig, _ pw.SessionResources) (session.Backend, error) {
	// A setup context is not a request context, so the client is read from the
	// middleware that opened it one slot earlier rather than from the chain.
	if _, opened := dynamo.EnsureClient(ctx); !opened {
		return session.Backend{}, errors.New(
			`session.backend = "dynamo" requires middleware.dynamo.enabled and the ` +
				`github.com/shibukawa/popcornwave/database/dynamo import`)
	}
	store := NewStore(Options{
		Table:          config.Dynamo.Table,
		ConsistentRead: config.Dynamo.ConsistentRead,
	})
	// No Close, because the client is the host's. No Prune, because nothing
	// here sweeps: a record is judged expired when it is read, and removing
	// the bytes is TTL on the table a deployment owns.
	return session.Backend{Store: store}, nil
}
