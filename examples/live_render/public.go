// Package publicassets serves this example's static files.
//
// The embed names dist/public rather than public, and that is the whole of what
// there is to know: pw build derives the served tree — hashed names, compressed
// variants — from the authored one, and embedding the authored side would ship
// files the running server never looks up. pw build says so if you get it wrong.
package publicassets

import (
	"embed"
	"io/fs"

	"github.com/shibukawa/popcornwave/middlewares"
)

//go:embed all:dist/public
var embeddedPublic embed.FS

func init() {
	middlewares.RegisterPublicFS(PublicFS())
}

func PublicFS() fs.FS {
	result, err := fs.Sub(embeddedPublic, "dist/public")
	if err != nil {
		panic(err)
	}
	return result
}
