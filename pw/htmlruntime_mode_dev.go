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
	if developmentConsoleURL() == "" {
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
	if console == "" {
		return nil
	}
	return map[string]string{developmentModuleName: developmentModule(console)}
}

func developmentConsoleURL() string {
	return strings.TrimSpace(os.Getenv(DevConsoleURLVar))
}

// developmentModule is the pwdev-only browser module.
//
// The console address is baked into the bytes rather than read from markup,
// because the address is known when the module is served and markup would mean
// the framework injecting into a document it does not own. Baking it in also
// means the revision moves when the console moves, which is correct: they are
// one deployment as far as a cached module is concerned.
//
// What this module does today is read the loop state once. The overlay that
// renders it, and the stream that replaces this fetch, arrive with their own
// step; this is the loading mechanism they land on.
func developmentModule(console string) string {
	address, err := json.Marshal(console)
	if err != nil {
		// The value came from the environment as a string, so encoding it
		// cannot fail; a build that proves otherwise should not serve a module
		// with an unquoted address spliced into it.
		return ""
	}
	return `// Popcorn Wave development module. Served only under the pwdev build mode,
// and never present in a binary pw build produced.

const console_ = ` + string(address) + `;

export async function loopState() {
	const response = await fetch(console_ + "/api/loop-state", { cache: "no-store" });
	if (!response.ok) throw new Error("loop state: " + response.status);
	return await response.json();
}

// The state is published on the document rather than returned, because the
// module is imported for its effect and nothing awaits it.
loopState().then(state => {
	window.__pwDevLoopState = state;
	window.dispatchEvent(new CustomEvent("pw:loop-state", { detail: state }));
}).catch(error => {
	window.__pwDevLoopStateError = String(error);
});
`
}
