//go:build fasthttp

package main

import (
	"context"
	"log"

	"helloworld/handlers"

	_ "github.com/shibukawa/popcornwave/database/sqlite"
	"github.com/shibukawa/popcornwave/pwfast"
)

func main() {
	handlers.RegisterConfig()
	mux := pwfast.NewServeMux()
	handlers.RegisterRoutes(pwfast.Routes(mux))
	if err := pwfast.Run(context.Background(), mux.Handler); err != nil {
		log.Fatal(err)
	}
}
