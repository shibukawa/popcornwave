package id_

// LoadUser is the page's own loader. The template declares it as an external
// and binds it with {val}, so the call site is in the page rather than in
// generated code.
//
// The trailing error is what lets it choose the response. A chain member's
// top-level binding is evaluated before the first byte, so an error carrying
// HTTP intent still selects the status while the rest of the page streams.
//
// page is declared optional, so an absent query value arrives as nil rather
// than as 0. Telling those apart is the whole reason to declare it that way:
// the default belongs here, not in the decoder.
func LoadUser(id string, page *int) (View, error) {
	number := 1
	if page != nil {
		number = *page
	}
	return View{Name: "user " + id, Page: number}, nil
}
