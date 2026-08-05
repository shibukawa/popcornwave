//go:build pwdev

package pw

import (
	"encoding/json"
	"os"
	"strings"
)

// DevConsoleURLVar names the environment variable pw dev sets on the
// application process to say where its console listens.
//
// It is read here rather than passed through configuration because it is not a
// runtime setting an application owns: pw dev resolves the address at startup
// and injects it, exactly as it injects the OTLP endpoint and the development
// issuer.
const DevConsoleURLVar = "PW_DEV_CONSOLE_URL"

// DevConsoleOverlayVar turns the overlay off while leaving the console address
// injected.
//
// The address is no longer the overlay's switch. It has a second consumer now —
// the data pane announces to the same console — so a project that turned the
// overlay off would otherwise have it back the moment it enabled the data pane.
// Absent means enabled, so only a project that turned it off sets it.
const DevConsoleOverlayVar = "PW_DEV_CONSOLE_OVERLAY"

// DevConsoleReloadVar carries the reload half of the overlay configuration. It
// is a second variable rather than a field in the state record, because whether
// to reload is configuration and the record describes what the loop did.
//
// Absent means enabled, so only a project that turned it off sets it.
const DevConsoleReloadVar = "PW_DEV_CONSOLE_RELOAD"

// developmentModuleName is the capability module the core imports under pwdev.
const developmentModuleName = "dev.js"

// developmentImport is appended to the core module under pwdev.
//
// A dynamic import rather than a second script tag in the document shell,
// because the shell is written once by pw init and then owned by the author: a
// development tag committed there would ship to production in every project
// that forgot to remove it. It also keeps true the rule that no framework code
// injects into the head.
//
// The specifier is relative, so it resolves inside the revision directory the
// core was served from, and nothing has to rewrite it.
//
// The import is not awaited and its failure is swallowed. A development
// convenience that could break the page it is attached to would be worse than
// no development convenience.
func developmentImport() string {
	if !developmentOverlay() || developmentConsoleURL() == "" {
		// No console is running, so there is nothing for the module to talk
		// to. Leaving the import out entirely also keeps the revision equal to
		// the release one in that case, which makes the difference between the
		// two sets exactly the presence of a console.
		return ""
	}
	return "\n\nimport(\"./" + developmentModuleName + "\").catch(() => {});\n"
}

func developmentScripts() map[string]string {
	console := developmentConsoleURL()
	if !developmentOverlay() || console == "" {
		return nil
	}
	return map[string]string{developmentModuleName: developmentModule(console, developmentReload())}
}

func developmentConsoleURL() string {
	return strings.TrimSpace(os.Getenv(DevConsoleURLVar))
}

// developmentOverlay reads the overlay switch, which is on unless turned off.
func developmentOverlay() bool { return !switchedOff(DevConsoleOverlayVar) }

// developmentReload reads the reload switch, which is on unless turned off.
func developmentReload() bool { return !switchedOff(DevConsoleReloadVar) }

func switchedOff(name string) bool {
	switch strings.TrimSpace(os.Getenv(name)) {
	case "0", "false":
		return true
	}
	return false
}

// developmentModule is the pwdev-only browser module: the error overlay and the
// reload that clears it.
//
// The console address is baked into the bytes rather than read from markup,
// because the address is known when the module is served and markup would mean
// the framework injecting into a document it does not own. Baking it in also
// means the revision moves when the console moves, which is correct: they are
// one deployment as far as a cached module is concerned.
func developmentModule(console string, reload bool) string {
	address, err := json.Marshal(console)
	if err != nil {
		// The value came from the environment as a string, so encoding it
		// cannot fail; a build that proves otherwise should not serve a module
		// with an unquoted address spliced into it.
		return ""
	}
	reloadLiteral := "false"
	if reload {
		reloadLiteral = "true"
	}
	return `// Popcorn Wave development overlay. Served only under the pwdev build mode,
// and never present in a binary pw build produced.

const consoleURL = ` + string(address) + `;
const reloadOnRecovery = ` + reloadLiteral + `;

const overlayID = "pw-dev-overlay";

// pageBuild is the application the page in front of us came from. It is learned
// from the first record rather than rendered into the document, because the
// framework injects nothing into a page it does not own.
let pageBuild = null;

function removeOverlay() {
	const existing = document.getElementById(overlayID);
	if (existing) existing.remove();
}

function showOverlay(state) {
	removeOverlay();
	const host = document.createElement("div");
	host.id = overlayID;
	// A shadow root so the application's stylesheet cannot restyle the
	// overlay and the overlay cannot restyle the application.
	const root = host.attachShadow({ mode: "open" });
	const style = document.createElement("style");
	style.textContent = ` + "`" + `
:host { all: initial; }
.sheet { position: fixed; inset: 0; z-index: 2147483647; overflow: auto;
  background: #12100fee; color: #f5f2f0; padding: 2.5rem 2rem;
  font: 14px/1.6 ui-monospace, SFMono-Regular, Menlo, monospace; }
.frame { max-width: 60rem; margin: 0 auto; }
.what { color: #f2b8b5; font-weight: 700; letter-spacing: .04em;
  text-transform: uppercase; font-size: 12px; }
.phase { font-size: 1.25rem; margin: .3rem 0 1.2rem; font-family: ui-sans-serif, system-ui, sans-serif; }
pre { background: #00000055; border: 1px solid #ffffff1f; border-radius: 6px;
  padding: 1rem; overflow-x: auto; white-space: pre-wrap; margin: 0; }
.where { margin-top: 1rem; color: #b9b2ad; }
a { color: #9fd0ff; }
` + "`" + `;
	const sheet = document.createElement("div");
	sheet.className = "sheet";
	const frame = document.createElement("div");
	frame.className = "frame";

	const what = document.createElement("div");
	what.className = "what";
	what.textContent = "pw dev — " + (state.status === "failed" ? "failed" : state.status);
	const phase = document.createElement("div");
	phase.className = "phase";
	phase.textContent = state.phase || "";
	const text = document.createElement("pre");
	// textContent, so a diagnostic quoting the developer's own markup is read
	// as the text it is.
	text.textContent = state.diagnostic ? state.diagnostic.text : "";
	frame.append(what, phase, text);

	if (state.diagnostic && state.diagnostic.file) {
		const where = document.createElement("div");
		where.className = "where";
		const link = document.createElement("a");
		const line = state.diagnostic.line || 1;
		link.href = "vscode://file/" + state.diagnostic.file + ":" + line;
		link.textContent = state.diagnostic.file + ":" + line;
		where.append("open ", link);
		frame.append(where);
	}

	const console_ = document.createElement("div");
	console_.className = "where";
	const consoleLink = document.createElement("a");
	consoleLink.href = consoleURL;
	consoleLink.textContent = consoleURL;
	console_.append("console ", consoleLink);
	frame.append(console_);

	sheet.append(frame);
	root.append(style, sheet);
	(document.body || document.documentElement).append(host);
}

function apply(state) {
	if (state.status === "failed") {
		showOverlay(state);
		return;
	}
	removeOverlay();
	if (state.status !== "healthy" || !state.build) return;
	if (pageBuild === null) {
		pageBuild = state.build;
		return;
	}
	// A different application is serving than the one this page came from, so
	// what is on screen is stale. Nothing here holds client state worth
	// preserving, which is what makes a full reload the whole feature.
	if (state.build !== pageBuild && reloadOnRecovery) {
		pageBuild = state.build;
		location.reload();
	}
}

// The stream terminates at the console, never at the application, so the
// overlay survives the process that served the page. EventSource reconnects on
// its own, which is what a page open across a restart needs.
const stream = new EventSource(consoleURL + "/api/loop-state/stream");
stream.onmessage = event => {
	let state;
	try {
		state = JSON.parse(event.data);
	} catch {
		return;
	}
	window.__pwDevLoopState = state;
	apply(state);
	window.dispatchEvent(new CustomEvent("pw:loop-state", { detail: state }));
};
`
}
