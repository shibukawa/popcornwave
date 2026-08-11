//go:build !fasthttp

package handlers

import (
	"context"
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
)

func init() {
	mux.HandleFunc("GET /{$}", index)
	mux.HandleFunc("GET /profile", profile)
}

func index(w http.ResponseWriter, r *http.Request) {
	pw.WriteHTML(w, r, Index(IndexParams{}))
}

// profile is an ordinary handler. Nothing here asks for streaming, sets a
// header, or writes a chunk: it supplies two pending values and hands the page
// to WriteHTML exactly as a fully synchronous handler would.
//
// The fail query parameter picks which dependency breaks, so both failure paths
// are reachable on purpose. An earlier draft failed at random instead, which
// made ordinary use log errors that looked like a malfunction.
func profile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	failing, _ := pw.QueryValue(r, "fail")

	pw.WriteHTML(w, r, Home(HomeParams{
		// An ordinary value renders in the first pass, so the page has something
		// real to show while the rest is still in flight.
		Profile: Profile{Name: "Ada Lovelace", Joined: "2026-02-11"},

		// pw.Go starts the work now, so both of these overlap each other and
		// overlap rendering. The context bounds the work and stays ours to
		// cancel; the render bounds only how long it waits for it.
		//
		// The orders boundary declares no recover clause and the recommendation
		// one does, which is the whole difference between the two failure pages.
		Orders: pw.Go(ctx, func(ctx context.Context) ([]Order, error) {
			return loadOrders(ctx, failing == "orders")
		}),
		Recommendation: pw.Go(ctx, func(ctx context.Context) (string, error) {
			return recommend(ctx, failing == "recommendation")
		}),
	}))
}
