package id_

// Load runs between the request and the render. Its parameters are the route's
// own inputs, and its results are the page component's parameters.
//
// page is declared optional, so an absent query value arrives as nil rather
// than as 0. Telling those apart is the whole reason to declare it that way:
// the default belongs here, not in the decoder.
func Load(id string, page *int) (string, int, error) {
	number := 1
	if page != nil {
		number = *page
	}
	return "user " + id, number, nil
}
