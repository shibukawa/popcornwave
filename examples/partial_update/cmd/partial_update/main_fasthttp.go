//go:build fasthttp

package main

import (
	"context"
	"log"

	"github.com/shibukawa/popcornwave/pwfast"
	"partial_update/pages"
)

func main() {
	mux := pwfast.NewServeMux()
	pages.Register(mux)
	if err := pwfast.Run(context.Background(), mux.Handler); err != nil {
		log.Fatal(err)
	}
}
