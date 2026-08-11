//go:build !fasthttp

package main

import (
	"context"
	"log"

	"partial_update/pages"

	"github.com/shibukawa/popcornwave/pw"
)

func main() {
	mux := pw.NewServeMux()
	// The page tree registers itself: every route below pages/ is discovered at
	// generation time, so this is the only line that knows a tree exists.
	pages.Register(mux)
	if err := pw.Run(context.Background(), mux); err != nil {
		log.Fatal(err)
	}
}
