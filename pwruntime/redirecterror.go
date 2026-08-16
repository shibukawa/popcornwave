package pwruntime

import "fmt"

// A redirect a function can return rather than write.
//
// The writer-taking Redirect is what a handler uses, and a handler is not the
// only thing that decides a response any more: a template binds its loader and
// that loader runs before the first byte, so a page that has to send the
// browser elsewhere has a value to return and no writer to write to. This is
// that value.
//
// It is an error rather than a second result because the position it has to
// travel through is an error: a loader returns (T, error), and the render
// hands whatever it gets back to the response path unwrapped.

// RedirectError carries a location and the status that sends the browser to it.
//
// It is exported so a caller can inspect one with errors.As, and constructed
// through the framework's own constructors so the status cannot be a value no
// browser treats as a redirect.
type RedirectError struct {
	// Location is the target, checked for navigability where it is written
	// rather than here, so one check covers both the returned and the written
	// form.
	Location string
	// Status is one of the five redirect codes.
	Status int
}

func (e RedirectError) Error() string {
	return fmt.Sprintf("redirect %d to %s", e.Status, e.Location)
}

// NewRedirect returns the value. The status comes from a named constructor
// rather than from a caller, so there is nothing here to validate: a redirect
// answering a status no browser follows would render the page it meant to
// leave, and no constructor can spell one.
func NewRedirect(location string, status int) error {
	return RedirectError{Location: location, Status: status}
}
