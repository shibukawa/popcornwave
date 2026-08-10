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
// The router target is left at upstream's default and nothing reads it. This
// framework emits its own route registration from the page tree, onto
// pwfastpage.Router, which pwfast.ServeMux satisfies; the target only names the
// router upstream's own route emitter would install on, and that emitter is not
// run here. Normalization requires a valid one, so it is left valid.
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

	return transform
}
