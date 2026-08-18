//go:build !fasthttp

package main

import (
	"context"
	"log"

	"async_render/handlers"

	"github.com/shibukawa/popcornweb/pw"
)

func main() {
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
