// Package fastfixture is a handler package laid out for both builds.
//
// It exists to compile. Every other test of the second build asserts something
// about the source that generation produced; this one is the only place both
// tag configurations are handed to the compiler, which is what would catch two
// halves that each look right and do not fit together.
//
// The layout is the one decision:transport-source-transform requires and this
// framework's scaffolds must teach: the shared declarations sit in an untagged
// file, and the transport handlers in a file the tag can exclude whole. Putting
// them in one file would take the types out of the second build along with the
// handler, and the derived copy would have nothing to bind.
package fastfixture
