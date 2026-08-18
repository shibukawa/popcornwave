package publicassets

import (
	"embed"
	"io/fs"

	"github.com/shibukawa/popcornweb/middlewares"
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
