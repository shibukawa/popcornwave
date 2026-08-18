package main

import (
	"context"
	"log"

	"github.com/shibukawa/popcornweb/pw"
	"passkeylogin/handlers"

	_ "github.com/shibukawa/popcornweb/authstate/sqlite"
	_ "github.com/shibukawa/popcornweb/database/sqlite"
	_ "github.com/shibukawa/popcornweb/sessionstore/sqlite"
)

func main() {
	// The resolver is installed before Run, because the framework calls it
	// during the OIDC callback.
	handlers.RegisterAccounts()
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
