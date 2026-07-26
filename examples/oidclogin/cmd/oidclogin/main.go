package main

import (
	"context"
	"log"

	"github.com/shibukawa/popcornwave/pw"
	"oidclogin/handlers"
)

func main() {
	// The resolver is installed before Run, because the framework calls it
	// during the OIDC callback.
	handlers.RegisterAccountResolver()
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
