//go:build tinygo

package main

// servePprof does nothing on TinyGo, where net/http/pprof does not compile.
// The declaration exists so main needs no build tag of its own.
func servePprof() {}
