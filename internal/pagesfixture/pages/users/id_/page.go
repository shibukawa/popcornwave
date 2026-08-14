package id_

import "context"

// Load runs between the request and the render. Its parameters are the route's
// own inputs, and its results are the page component's parameters.
//
// page is declared optional, so an absent query value arrives as nil rather
// than as 0. Telling those apart is the whole reason to declare it that way:
// the default belongs here, not in the decoder.
//
// The leading context is the request's, which is what a page reading a database
// pool or the signed-in reader needs — both live on it. It is optional and only
// the first position counts, so a page needing none declares none.
func Load(ctx context.Context, id string, page *int) (string, int, error) {
	number := 1
	if page != nil {
		number = *page
	}
	_ = ctx
	return "user " + id, number, nil
}
