//go:build pwdev

package pw

import (
	_ "embed"
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

// DevConsoleLauncherVar turns the floating console launcher off. Absent means
// enabled, on the same terms as the overlay: only a project that turned it off
// sets it.
const DevConsoleLauncherVar = "PW_DEV_CONSOLE_LAUNCHER"

// DevConsoleLauncherCornerVar places the launcher. pw dev validates the value
// against the four corners before injecting it, so an unrecognised one is a
// configuration error there rather than a silent fallback here.
//
// An empty value means the default, which is the only fallback this file has:
// it is what an application started outside pw dev would see.
const DevConsoleLauncherCornerVar = "PW_DEV_CONSOLE_LAUNCHER_CORNER"

// developmentModuleName is the capability module the core imports under pwdev.
const developmentModuleName = "dev.js"

// developmentMarkName is the launcher's mark, served beside the module that
// references it. It is a file rather than a data URI because an inlined image
// answers to the application's own img-src, and a project that tightened its
// policy to default-src 'self' would lose it; served from the application's
// origin it is covered like every other framework asset.
const developmentMarkName = "devmark.webp"

//go:embed devmark.webp
var developmentMark []byte

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
	if !developmentWanted() {
		// No console is running, or nothing wants to talk to it, so there is
		// nothing for the module to do. Leaving the import out entirely also
		// keeps the revision equal to the release one in that case, which makes
		// the difference between the two sets exactly the presence of a
		// console.
		return ""
	}
	return "\n\nimport(\"./" + developmentModuleName + "\").catch(() => {});\n"
}

func developmentScripts() map[string]string {
	if !developmentWanted() {
		return nil
	}
	scripts := map[string]string{
		developmentModuleName: developmentModule(
			developmentConsoleURL(),
			developmentReload(),
			developmentOverlay(),
			developmentLauncher(),
			developmentLauncherCorner(),
		),
	}
	if developmentLauncher() {
		// Only when something references it. The set feeds the revision digest,
		// so an asset nobody loads would still move every URL.
		scripts[developmentMarkName] = string(developmentMark)
	}
	return scripts
}

// developmentWanted reports whether a console is reachable and either behavior
// that runs inside the application's pages is on.
//
// Both off is what makes a served page byte-identical to a production render:
// there is no module, no import of one, and no address in the bytes.
func developmentWanted() bool {
	if developmentConsoleURL() == "" {
		return false
	}
	return developmentOverlay() || developmentLauncher()
}

func developmentConsoleURL() string {
	return strings.TrimSpace(os.Getenv(DevConsoleURLVar))
}

// developmentOverlay reads the overlay switch, which is on unless turned off.
func developmentOverlay() bool { return !switchedOff(DevConsoleOverlayVar) }

// developmentReload reads the reload switch, which is on unless turned off.
func developmentReload() bool { return !switchedOff(DevConsoleReloadVar) }

// developmentLauncher reads the launcher switch, which is on unless turned off.
func developmentLauncher() bool { return !switchedOff(DevConsoleLauncherVar) }

// developmentLauncherCorner reads the corner, defaulting to the one an
// application is least likely to have taken for itself.
func developmentLauncherCorner() string {
	switch corner := strings.TrimSpace(os.Getenv(DevConsoleLauncherCornerVar)); corner {
	case DevLauncherBottomLeft, DevLauncherBottomRight, DevLauncherTopLeft, DevLauncherTopRight:
		return corner
	default:
		return DevLauncherBottomLeft
	}
}

// The corners the launcher may take. pw dev keeps its own copy of these names
// and validates against it, for the same reason it keeps its own copy of the
// variable names above: it is a host build and cannot reference the pwdev half
// of the framework at all.
const (
	DevLauncherBottomLeft  = "bottom-left"
	DevLauncherBottomRight = "bottom-right"
	DevLauncherTopLeft     = "top-left"
	DevLauncherTopRight    = "top-right"
)

func switchedOff(name string) bool {
	switch strings.TrimSpace(os.Getenv(name)) {
	case "0", "false":
		return true
	}
	return false
}

// cornerEdges splits a corner into the two CSS edges it anchors to and the flex
// direction that grows the label inward, so a revealed label never runs off the
// side of the viewport it is nearest to.
func cornerEdges(corner string) (vertical, horizontal, direction string) {
	vertical, horizontal, direction = "bottom", "left", "row"
	switch corner {
	case DevLauncherBottomRight:
		horizontal, direction = "right", "row-reverse"
	case DevLauncherTopLeft:
		vertical = "top"
	case DevLauncherTopRight:
		vertical, horizontal, direction = "top", "right", "row-reverse"
	}
	return vertical, horizontal, direction
}

// flyoutEdge is the side of the button the label and the dismiss control open
// from: away from the edge the launcher is anchored to, which is the middle of
// the viewport in every corner.
func flyoutEdge(horizontal string) string {
	if horizontal == "right" {
		return "right"
	}
	return "left"
}

// developmentModule is the pwdev-only browser module: the error overlay, the
// reload that clears it, and the launcher that opens the console.
//
// One module rather than two, because both halves want the console address and
// the same stream, and splitting them would open a second one.
//
// The console address is baked into the bytes rather than read from markup,
// because the address is known when the module is served and markup would mean
// the framework injecting into a document it does not own. Baking it in also
// means the revision moves when the console moves, which is correct: they are
// one deployment as far as a cached module is concerned.
func developmentModule(console string, reload, overlay, launcher bool, corner string) string {
	address, err := json.Marshal(console)
	if err != nil {
		// The value came from the environment as a string, so encoding it
		// cannot fail; a build that proves otherwise should not serve a module
		// with an unquoted address spliced into it.
		return ""
	}
	vertical, horizontal, direction := cornerEdges(corner)
	flyout := flyoutEdge(horizontal)
	return `// Popcorn Wave development runtime. Served only under the pwdev build mode,
// and never present in a binary pw build produced.

const consoleURL = ` + string(address) + `;
const reloadOnRecovery = ` + jsBool(reload) + `;
const overlayEnabled = ` + jsBool(overlay) + `;
const launcherEnabled = ` + jsBool(launcher) + `;

// pageBuild is the application the page in front of us came from. It is learned
// from the first record rather than rendered into the document, because the
// framework injects nothing into a page it does not own.
let pageBuild = null;

function attach(host) {
	(document.body || document.documentElement).append(host);
}

` + developmentOverlaySource() + `
` + developmentLauncherSource(vertical, horizontal, direction, flyout) + `
function apply(state) {
	if (state.status === "failed") {
		showOverlay(state);
		hideLauncher();
		return;
	}
	removeOverlay();
	showLauncher(state.status);
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

// The launcher is shown before the first record arrives, so a console that is
// slow or unreachable costs the status and not the way in.
showLauncher("healthy");

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

func jsBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

// developmentOverlaySource is the overlay half of the module. It is emitted
// whole rather than branched on at run time, so a project that turned the
// overlay off is served bytes that cannot show one.
func developmentOverlaySource() string {
	return `const overlayID = "pw-dev-overlay";

function removeOverlay() {
	const existing = document.getElementById(overlayID);
	if (existing) existing.remove();
}

function showOverlay(state) {
	if (!overlayEnabled) return;
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
	attach(host);
}
`
}

// developmentLauncherSource is the launcher half of the module.
//
// The mark is resolved against import.meta.url rather than against the
// document, because the module is served from a revision directory whose name
// is a digest of the set the mark is in: a URL written here would have to
// contain the digest of the bytes it is written into.
func developmentLauncherSource(vertical, horizontal, direction, flyout string) string {
	return `const launcherID = "pw-dev-launcher";
const dismissKey = "pw:dev-launcher-dismissed";
const markURL = new URL("./` + developmentMarkName + `", import.meta.url).href;

let launcherHost = null;

// Dismissal is remembered for the browsing session, so a developer who hid the
// launcher to reach the control underneath does not hide it again on every
// reload. A permanent hide is dev.console.launcher.enabled, which is versioned
// with the project; session storage is invisible to everyone else and would
// outlive the developer remembering they set it.
function dismissed() {
	try {
		return sessionStorage.getItem(dismissKey) === "1";
	} catch {
		return false;
	}
}

function rememberDismissed() {
	try {
		sessionStorage.setItem(dismissKey, "1");
	} catch {
		// A page served without storage access still dismisses for its own
		// lifetime, which is the case dismissal is mostly wanted in.
	}
}

function buildLauncher() {
	const host = document.createElement("div");
	host.id = launcherID;
	const root = host.attachShadow({ mode: "open" });
	const style = document.createElement("style");
	style.textContent = ` + "`" + `
:host { all: initial; }
/* Exactly the button, so the launcher covers 44px of the application and not a
   pixel more. The label and the dismiss control are taken out of flow below:
   left in the row they would hold their width while invisible, and the fixed
   box would swallow clicks along a strip the developer cannot see. */
.wrap { position: fixed; ` + vertical + `: 1rem; ` + horizontal + `: 1rem;
  z-index: 2147483646; width: 44px; height: 44px;
  font: 500 13px/1.2 ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif; }
.button { position: absolute; inset: 0; display: grid; place-items: center;
  border-radius: 14px; box-sizing: border-box;
  background: #fffdf8; border: 1px solid #00000017; text-decoration: none;
  box-shadow: 0 1px 2px #00000021, 0 6px 18px #00000014;
  -webkit-tap-highlight-color: transparent; }
.button:focus-visible { outline: 2px solid #4c8dff; outline-offset: 3px; }
.mark { display: block; width: 30px; height: 30px; }
/* Opens away from the edge the launcher is anchored to, so nothing revealed
   runs off the side of the viewport it is nearest to. The padding is what
   makes the gap, rather than a margin: it keeps the hover target continuous
   from the button, and a dead zone between them would collapse the reveal as
   the pointer crossed it. */
.flyout { position: absolute; top: 50%; ` + flyout + `: 100%;
  transform: translateY(-50%); padding-` + flyout + `: .5rem;
  display: flex; flex-direction: ` + direction + `; align-items: center; gap: .5rem;
  opacity: 0; pointer-events: none; transition: opacity .12s ease; }
.wrap:hover .flyout, .wrap:focus-within .flyout { opacity: 1; pointer-events: auto; }
.label { white-space: nowrap; border-radius: 8px; padding: .45rem .6rem;
  background: #26221fee; color: #f7f3ec; }
.dismiss { width: 22px; height: 22px; border-radius: 11px; cursor: pointer;
  border: 1px solid #00000017; background: #fffdf8; color: #5d564f;
  font: 600 13px/1 ui-sans-serif, system-ui, sans-serif; padding: 0; }
.dismiss:focus-visible { outline: 2px solid #4c8dff; outline-offset: 2px; }
.starting .button::after { content: ""; position: absolute; inset: -4px;
  border-radius: 18px; border: 2px solid #e0a02a;
  animation: pw-dev-pulse 1.4s ease-in-out infinite; }
@keyframes pw-dev-pulse { 0%, 100% { opacity: .25 } 50% { opacity: 1 } }
@media (prefers-reduced-motion: reduce) {
  .flyout { transition: none }
  .starting .button::after { animation: none; opacity: .9 }
}
@media (prefers-color-scheme: dark) {
  .button, .dismiss { background: #2a2523; border-color: #ffffff21; }
  .dismiss { color: #d9d2cb; }
  .label { background: #f7f3ecf2; color: #24201d; }
}
` + "`" + `;

	const wrap = document.createElement("div");
	wrap.className = "wrap";

	const link = document.createElement("a");
	link.className = "button";
	link.href = consoleURL;
	// A named target rather than rel=noopener: a browser ignores the name when
	// noopener is set and opens a fresh tab every time, and a control clicked
	// all day should return to the tab it already opened. Nothing is lost —
	// the console is a different loopback port, so the opener it gets is
	// cross-origin and can reach nothing.
	link.target = "pw-dev-console";
	link.setAttribute("aria-label", "Open the pw dev console");
	const mark = document.createElement("img");
	mark.className = "mark";
	mark.src = markURL;
	// The link already carries the name, so the image is decoration.
	mark.alt = "";
	link.append(mark);

	const flyout = document.createElement("div");
	flyout.className = "flyout";

	const label = document.createElement("span");
	label.className = "label";
	label.textContent = "pw dev console";

	const dismiss = document.createElement("button");
	dismiss.className = "dismiss";
	dismiss.type = "button";
	dismiss.textContent = "×";
	dismiss.setAttribute("aria-label", "Hide the pw dev launcher until this tab closes");
	dismiss.addEventListener("click", () => {
		rememberDismissed();
		hideLauncher();
	});

	flyout.append(label, dismiss);
	wrap.append(link, flyout);
	root.append(style, wrap);
	return host;
}

function showLauncher(status) {
	if (!launcherEnabled || dismissed()) return;
	if (!launcherHost) launcherHost = buildLauncher();
	// Removed and re-attached rather than hidden, so nothing of it ghosts
	// through a diagnostic the overlay is showing.
	if (!launcherHost.isConnected) attach(launcherHost);
	const wrap = launcherHost.shadowRoot.querySelector(".wrap");
	// The page on screen is already stale while the loop is between two working
	// applications, and nothing else on it says so.
	wrap.classList.toggle("starting", status === "starting");
}

function hideLauncher() {
	if (launcherHost) launcherHost.remove();
}
`
}
