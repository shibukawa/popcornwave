package id_

import "context"

// The loaders the template declares as externals and binds with {val}. They are
// ordinary Go in the route package: no entry point, no registration, and
// nothing generated around them.
//
// The leading context is the request's, which routetree threads by scanning the
// directory the template sits in. A loader needing none declares none.
func LoadName(ctx context.Context, id string, page *int) (string, error) {
	_ = ctx
	return "user " + id, nil
}

// PageNumber is where the default for an absent query value lives.
//
// page is declared optional, so an absent value arrives as nil rather than as
// 0. Telling those apart is the whole reason to declare it that way, and the
// default belongs in Go rather than in the decoder.
func PageNumber(page *int) int {
	if page == nil {
		return 1
	}
	return *page
}
