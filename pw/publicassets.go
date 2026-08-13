package pw

import "github.com/shibukawa/popcornwave/middlewares"

// PublicAssetURL is the URL a document should name for one of the
// application's static assets, given its path inside the served tree.
//
// A template calls this rather than writing a literal path, for the reason it
// calls RuntimeScriptURL rather than writing one: the URL a build serves an
// asset under carries a revision derived from that asset's own bytes, and only
// a URL nobody can hold across a change may be cached forever. A literal has no
// revision, so it revalidates on every load — correct, and one round trip per
// asset per page.
//
// It answers for whichever transport is running, because the manifest it reads
// and the mount it resolves are both in the shared leaf.
func PublicAssetURL(name string) string { return middlewares.PublicAssetURL(name) }
