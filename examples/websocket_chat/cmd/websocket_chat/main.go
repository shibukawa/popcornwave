//go:build !fasthttp

package main

import (
	"context"
	"log"

	"websocket_chat/handlers"

	"github.com/shibukawa/popcornwave/pw"
)

func main() {
	// A socket failure raised after the connection was accepted has no status
	// left to carry it. Nothing is logged without this — the name says stream
	// because one installer covers both.
	pw.SetStreamErrorHandler(func(err error) {
		log.Printf("socket: %v", err)
	})

	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
