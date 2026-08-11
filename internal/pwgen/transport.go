package pwgen

import "github.com/shibukawa/tinybind-go/generator"

// FastTransform is how this framework's handlers are derived for the second
// transport.
//
// Everything mechanical about the derivation belongs to system:tinybind: which
// occurrences are eligible, how the two transport parameters collapse into one
// context, and what a refusal says. What is here is the vocabulary that
// derivation runs against — the pw packages and their fasthttp siblings, and the
// registered call patterns without which every pw call reads as an unrecognized
// third-party one.
//
// calls is the pattern set from Options. It is a parameter rather than a second
// registration so that the two can never disagree: a pw entry gains a pattern
// once, and both the analysis and the rewrite see the same set.
//
// The router target names this framework's own installer rather than upstream's
// default, which is a vendored trie router an application here never imports.
// Two kinds of route reach the second build and neither goes through that:
// a page tree brings its own registry, on pwfastpage.Router, and the handlers
// an application registers itself are emitted onto pwfast.RouteInstaller.
//
// The catch-all suffix is Go's own, which reads like a no-op and is not. It
// tells the emitter to leave {rest...} exactly as the net/http source spelled
// it, so the one translation from Go 1.22 patterns to this transport's router
// happens inside pwfast where the subtree and the {$} marker are translated
// too — rather than half here and half there.
func FastTransform(calls []generator.CallPattern) generator.TransformOptions {
	transform := generator.DefaultTransformOptions()
	transform.Calls = calls

	// The pairs this framework owns. Upstream's defaults already map its own
	// two packages, and these are added rather than replacing them, because a
	// handler reaching tinybind directly is still ordinary application code.
	//
	// Only the import line moves; the local name is preserved, so a call
	// selector in a rewritten body is untouched. That is what makes the pair a
	// requirement rather than a convenience: pwfast must declare every name pw
	// does, which TestEveryRegisteredCallHasSomewhereToLand holds it to.
	transform.ImportRewrites[pwPackage] = pwFastPackage
	transform.ImportRewrites[pwPagePackage] = pwFastPagePackage

	transform.Router = generator.RouterTarget{
		Import:         pwFastPackage,
		Qualifier:      "pwfast",
		Type:           "pwfast.RouteInstaller",
		RegisterFunc:   "RegisterRoutes",
		CatchAllSuffix: "...",
	}
	return transform
}
