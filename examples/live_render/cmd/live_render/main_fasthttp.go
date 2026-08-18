//go:build fasthttp

package main

import (
	"context"
	"log"

	"github.com/shibukawa/popcornweb/pwfast"
	"live_render/handlers"
)

func main() {
	mux := pwfast.NewServeMux()
	handlers.RegisterRoutes(pwfast.Routes(mux))
	if err := pwfast.Run(context.Background(), mux.Handler); err != nil {
		log.Fatal(err)
	}
}
