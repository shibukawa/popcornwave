package handlers

// The request types these routes read. They sit apart from the handlers that
// read them because a build tag excludes a whole file, and the derived handlers
// of the second build bind the same types — so a type beside a handler is a
// type one of the two builds would not have.

type listInput struct {
	// input reads the query string and falls back to the body, which is what
	// makes this handler indifferent to where htmx put the value: a GET carries
	// it in the URL, and a DELETE does so only depending on the client's
	// configuration.
	Query string `input:"q" check:"maxlen=40"`
}

type createInput struct {
	Title    string `payload:"title" check:"required,maxlen=60"`
	Owner    string `payload:"owner" check:"required,maxlen=24"`
	Priority string `payload:"priority" enum:"low,normal,high" default:"normal"`
	Query    string `input:"q" check:"maxlen=40"`
}

type removeInput struct {
	ID    string `path:"id"`
	Query string `input:"q" check:"maxlen=40"`
}
