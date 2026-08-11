//go:build !fasthttp

package main

import (
	"context"
	"log"

	"async_render/handlers"

	"github.com/shibukawa/popcornwave/pw"
)

func main() {
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
