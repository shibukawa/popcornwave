//go:build fasthttp

package main

import (
	"context"
	"log"

	"async_render/handlers"
	"github.com/shibukawa/popcornweb/pwfast"
)

func main() {
	mux := pwfast.NewServeMux()
	handlers.RegisterRoutes(pwfast.Routes(mux))
	if err := pwfast.Run(context.Background(), mux.Handler); err != nil {
		log.Fatal(err)
	}
}
