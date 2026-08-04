// Popcorn Wave update bootstrap.
//
// The two halves above this file are libraries: the boundary runtime installs
// its own custom elements because the parser needs them present, and the update
// runtime installs nothing at all. This builds the one instance and gives it the
// name this framework owns.
//
// The configuration arrives as an inert escaped meta element rather than as an
// attribute on this script's own tag. A module script has no
// document.currentScript, so the tag is unreachable from here; a meta is
// readable from anywhere and carries no script for a policy to allow.

const updateConfigMeta = document.querySelector('meta[name="pw-runtime"]');

if (updateConfigMeta) {
	let updateConfig = null;
	try {
		updateConfig = JSON.parse(updateConfigMeta.getAttribute("content") || "");
	} catch (error) {
		// A malformed configuration disables updates rather than throwing during
		// load. Throwing here would take the boundary runtime above down with
		// it, and that half works without any of this.
		console.error("Popcorn Wave: unreadable runtime configuration", error);
	}
	if (updateConfig) {
		const runtime = createUpdateRuntime(updateConfig);
		if (updateConfig.global) {
			window[updateConfig.global] = runtime;
		}
	}
}
