//go:build fasthttp

package main

import (
	"context"
	"log"

	"websocket_chat/handlers"

	"github.com/shibukawa/popcornwave/pwfast"
)

func main() {
	pwfast.SetStreamErrorHandler(func(err error) {
		log.Printf("socket: %v", err)
	})

	mux := pwfast.NewServeMux()
	handlers.RegisterRoutes(pwfast.Routes(mux))
	if err := pwfast.Run(context.Background(), mux.Handler); err != nil {
		log.Fatal(err)
	}
}
