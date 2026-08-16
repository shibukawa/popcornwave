package id_

// LoadUser runs between the request and the render.
//
// It is an external the template declares and binds with {val}, which is what
// replaced the typed Load rung: the component names what it needs rather than
// a signature contract matching a result list to a parameter list positionally.
//
// page is declared optional, so an absent query value arrives as nil rather
// than as 0. Telling those apart is the whole reason to declare it that way:
// the default belongs here, not in the decoder.
func LoadUser(id string, page *int) UserView {
	number := 1
	if page != nil {
		number = *page
	}
	return UserView{Name: "user " + id, Page: number}
}
