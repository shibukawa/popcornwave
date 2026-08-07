package main

import (
	"context"
	"log"

	"todo-popcornwave/handlers"
	"github.com/shibukawa/popcornwave/pw"
	// Registers the engine the configured DSN names.
	_ "github.com/shibukawa/popcornwave/database/postgres"
)

func main() {
	servePprof()

	// Names the API document served at server.openapi_path and shown by the
	// reference UI at /docs. Without it both fall back to "Application API".
	if err := pw.SetOpenAPIInfo(pw.OpenAPIInfo{Title: "popcornwave", Version: "0.1.0"}); err != nil {
		log.Fatal(err)
	}

	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
