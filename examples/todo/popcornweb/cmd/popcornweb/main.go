package main

import (
	"context"
	"log"

	"github.com/shibukawa/popcornweb/pw"
	"todo-popcornweb/handlers"
	// Registers the engine the configured DSN names.
	_ "github.com/shibukawa/popcornweb/database/postgres"
)

func main() {
	servePprof()

	// Names the API document served at server.openapi_path and shown by the
	// reference UI at /docs. Without it both fall back to "Application API".
	if err := pw.SetOpenAPIInfo(pw.OpenAPIInfo{Title: "popcornweb", Version: "0.1.0"}); err != nil {
		log.Fatal(err)
	}

	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
