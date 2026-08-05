package firestore

import "github.com/shibukawa/popcornwave/pw"

// Binding has to happen after the generated definition exists, because
// configbind.Bind panics on a type it has no definition for. Go initializes a
// package's files in lexical file name order, and this file sorts after
// configbind_gen.go; keep it that way when renaming either file.
func init() {
	pw.RegisterConfig[Config]("middleware.firestore")
}
