//go:build !fasthttp

package main

import (
	"context"
	"log"
	"net/http"

	"github.com/shibukawa/popcornweb/pw"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	if err := pw.Run(context.Background(), mux); err != nil {
		log.Fatal(err)
	}
}
