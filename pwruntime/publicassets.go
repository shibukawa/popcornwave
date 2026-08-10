package pwruntime

// PublicAssetSettings is the static asset configuration.
//
// It is in the shared leaf for the reason the security headers and the CSRF
// configuration are: both chains read it, none of it names a transport, and one
// declaration means one set of binder tags.

// PublicAssetConfig controls the framework-owned static asset endpoint.
type PublicAssetSettings struct {
	Enabled   bool   `default:"true"`
	Mount     string `default:"/public" dependon:".enabled"`
	ReadLocal bool   `default:"false" dependon:".enabled"`
}
