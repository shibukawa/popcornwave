//go:build fasthttp

package main

import (
	"context"
	"log"

	"github.com/shibukawa/popcornwave/pwfast"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

func main() {
	mux := pwfast.NewServeMux()
	mux.HandleFunc("GET /", func(ctx *fasthttp.RequestCtx) { _, _ = ctx.WriteString("ok") })
	if err := pwfast.Run(context.Background(), mux.Handler); err != nil {
		log.Fatal(err)
	}
}
