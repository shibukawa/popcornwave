//go:build !fasthttp

package main

import (
	"context"
	"log"

	"htmx_fragment/handlers"

	"github.com/shibukawa/popcornwave/pw"
)

func main() {
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
