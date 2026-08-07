package main

import (
	"context"
	"log"

	"github.com/shibukawa/popcornwave/pw"
	"passkeylogin/handlers"

	_ "github.com/shibukawa/popcornwave/authstate/sqlite"
	_ "github.com/shibukawa/popcornwave/database/sqlite"
	_ "github.com/shibukawa/popcornwave/sessionstore/sqlite"
)

func main() {
	// The resolver is installed before Run, because the framework calls it
	// during the OIDC callback.
	handlers.RegisterAccounts()
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
