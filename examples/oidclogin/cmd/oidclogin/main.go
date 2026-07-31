package main

import (
	"context"
	"log"

	"github.com/shibukawa/popcornwave/pw"
	"oidclogin/handlers"

	// session.backend = "rdb" is served by this import: storage is opt-in, so
	// an application links the backend it configured and no other.
	_ "github.com/shibukawa/popcornwave/sessionstore/sqlite"

	// The single-use login and ceremony records this engine stores.
	_ "github.com/shibukawa/popcornwave/authstate/sqlite"
)

func main() {
	// The resolver is installed before Run, because the framework calls it
	// during the OIDC callback.
	handlers.RegisterAccountResolver()
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
