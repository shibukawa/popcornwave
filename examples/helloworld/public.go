package publicassets

import (
	"embed"
	"io/fs"

	"github.com/shibukawa/popcornwave/pw"
)

//go:embed all:public
var embeddedPublic embed.FS

func init() {
	pw.RegisterPublicFS(PublicFS())
}

func PublicFS() fs.FS {
	result, err := fs.Sub(embeddedPublic, "public")
	if err != nil {
		panic(err)
	}
	return result
}
