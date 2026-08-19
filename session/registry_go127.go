//go:build go1.27

package session

// Register declares one piece of per-browser state.
//
// It is the package function with the registry as the receiver — the registry
// was its first argument only because the typed codec needs a type parameter
// and a method could not declare one. See that function for what key means in
// each placement, why the call belongs in main rather than in an init, and
// which duplicates are errors; this is the same call with the same rules.
//
// The type is inferred from a non-nil codec. A slot taking the default codec
// passes nil and writes the type: registry.Register[Account]("account",
// session.Private, nil).
//
// See pwruntime/cache_go127.go for why this file is tagged go1.27 rather than
// shipped, and for what has to happen if that release arrives without methods
// that take type parameters.
func (r *Registry) Register[T any](key string, placement Placement, codec Codec[T], options ...SlotOption) error {
	return Register[T](r, key, placement, codec, options...)
}
