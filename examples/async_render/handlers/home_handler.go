package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

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
	failing := r.URL.Query().Get("fail")

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

func loadOrders(ctx context.Context, fail bool) ([]Order, error) {
	if err := sleep(ctx, 900*time.Millisecond); err != nil {
		return nil, err
	}
	if fail {
		return nil, errors.New("order service returned 503")
	}
	return []Order{
		{Id: "A-1043", Total: "¥12,800"},
		{Id: "A-1088", Total: "¥3,200"},
		{Id: "A-1120", Total: "¥45,000"},
	}, nil
}

// recommend is the slower of the two dependencies, and the one with a recover
// clause behind it. The error it returns never reaches the page: a recover
// subtree sees a safe pw.AsyncError, and this text goes to the log instead.
func recommend(ctx context.Context, fail bool) (string, error) {
	if err := sleep(ctx, 1500*time.Millisecond); err != nil {
		return "", err
	}
	if fail {
		return "", errors.New("recommendation service returned 503")
	}
	return "Because you bought A-1043, you may like the analytical engine starter kit.", nil
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
