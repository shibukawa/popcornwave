//go:build fasthttp

package main

import (
	"context"
	"log"

	"helloworld/handlers"

	_ "github.com/shibukawa/popcornweb/database/sqlite"
	"github.com/shibukawa/popcornweb/pwfast"
)

func main() {
	handlers.RegisterConfig()
	handlers.RegisterSeed()
	if command, ok := handlers.Seed(); ok {
		// The subcommand answers and the server never starts, which is the
		// whole point of typed CLI-only input.
		log.Printf("seeding %d rows into %s", command.Count, command.Table)
		return
	}
	mux := pwfast.NewServeMux()
	handlers.RegisterRoutes(pwfast.Routes(mux))
	if err := pwfast.Run(context.Background(), mux.Handler); err != nil {
		log.Fatal(err)
	}
}
