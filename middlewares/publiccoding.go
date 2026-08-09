package middlewares

import "strings"

// staticContentCodings are the precompressed forms a public asset may carry,
// in the order a response prefers them.
//
// The order here is pure ratio, unlike the one a dynamic response uses. Every
// representation was produced by the build, so preferring the smallest costs a
// request nothing: brotli leads because at build levels it beats zstd by
// roughly 15 percent and gzip by 17, a margin that exists only at levels no
// per-request encoder could afford.
//
// The suffix is the sidecar's, and it is never part of a URL. A request naming
// one is refused by publicAssetName, because a URL that addressed a
// representation directly would let a cache learn about bytes the negotiation
// never offered it.
var staticContentCodings = [maxStaticCodings]struct {
	token  string
	suffix string
}{
	{token: "br", suffix: ".br"},
	{token: "zstd", suffix: ".zstd"},
	{token: "gzip", suffix: ".gz"},
}

// maxStaticCodings sizes the per-request negotiation scratch, so an
// Accept-Encoding parse stays off the heap.
const maxStaticCodings = 3

// staticCodingRank locates a coding token in the preference order, or reports
// -1 for one this middleware does not serve. A manifest naming an unknown
// coding is not an error: it is a representation this build will not choose,
// and the identity form still answers.
func staticCodingRank(token string) int {
	for i := range staticContentCodings {
		if strings.EqualFold(token, staticContentCodings[i].token) {
			return i
		}
	}
	return -1
}

// hasStaticCodingSuffix reports whether a path names a sidecar rather than an
// asset.
func hasStaticCodingSuffix(name string) bool {
	for i := range staticContentCodings {
		if strings.HasSuffix(name, staticContentCodings[i].suffix) {
			return true
		}
	}
	return false
}
