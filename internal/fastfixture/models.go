package fastfixture

// Greeting is what a caller asks for. It has a field that can arrive in the
// body, which is what makes the generated binder read the body lazily — the
// shape that used to refuse the whole derivation on the second run.
type Greeting struct {
	Name string `json:"name" check:"required"`
}

// Greeted is what it gets back.
type Greeted struct {
	Message string `json:"message"`
}
