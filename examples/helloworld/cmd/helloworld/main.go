//go:build !fasthttp

package main

import (
	"context"
	"log"

	"helloworld/handlers"

	_ "github.com/shibukawa/popcornweb/database/sqlite"
	"github.com/shibukawa/popcornweb/pw"
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
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
