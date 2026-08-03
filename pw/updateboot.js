// Popcorn Wave update bootstrap.
//
// The partial-update half above this is a factory: it installs nothing and
// names nothing on its own, so something has to build one instance and give it
// the names this framework owns. That is all this file does.
//
// The configuration arrives as an inert escaped meta element rather than as an
// attribute on this script's own tag. A module script has no
// document.currentScript, so the tag is unreachable from here; a meta is
// readable from anywhere and carries no script for a policy to allow.

const updateConfigMeta = document.querySelector('meta[name="pw-runtime"]');

if (updateConfigMeta && typeof createPartialUpdateRuntime === "function") {
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
		const runtime = createPartialUpdateRuntime(updateConfig);
		if (updateConfig.global) {
			window[updateConfig.global] = runtime;
		}
	}
}
