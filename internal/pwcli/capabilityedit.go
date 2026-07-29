package pwcli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// tailwindDevboxPackage is the pinned toolchain of decision:tailwind-host-toolchain.
const tailwindDevboxPackage = "tailwindcss_4@4.1.18"

// tailwindToolchainRequirement is what an operator installing the toolchain
// themselves has to satisfy. The Devbox package name is a nixpkgs identifier,
// so it means nothing to someone reaching for mise, Homebrew, or Scoop.
const tailwindToolchainRequirement = "the standalone tailwindcss CLI, version 4 or later"

// capabilityQueriesPurpose is the generate purpose the database capability has
// to open before its SQL example is read by anything.
const capabilityQueriesPurpose = "queries"

// setGeneratePurpose rewrites one generate purpose in popcornwave.toml. The
// key is edited in place rather than appended, because every purpose is
// required and therefore already present.
func setGeneratePurpose(state projectState, purpose string, values []string) (string, error) {
	source, err := os.ReadFile(filepath.Join(state.root, "popcornwave.toml"))
	if err != nil {
		return "", err
	}
	table := ""
	var out strings.Builder
	replaced := false
	for line := range strings.Lines(string(source)) {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			table = strings.Trim(trimmed, "[]")
		}
		if !replaced && table == "generate" && isKeyLine(trimmed, purpose) {
			out.WriteString(purpose + " = [" + quotedList(values) + "]\n")
			replaced = true
			continue
		}
		out.WriteString(line)
	}
	if !replaced {
		return "", fmt.Errorf("popcornwave.toml: generate.%s not found", purpose)
	}
	return out.String(), nil
}

// setProjectDatabase records the engine .pw.sql sources are generated for,
// replacing an existing key or writing one under [project]. It takes the
// document rather than reading the file, so it composes with the other edits a
// plan makes to popcornwave.toml instead of overwriting them.
func setProjectDatabase(document, engine string) (string, error) {
	assignment := "database = " + strconv.Quote(engine) + "\n"
	table := ""
	var out strings.Builder
	written := false
	// The key belongs at the end of [project], which is wherever the next
	// table header starts or the document ends.
	for line := range strings.Lines(document) {
		trimmed := strings.TrimSpace(line)
		header := strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")
		if !written && table == "project" && (header || isKeyLine(trimmed, "database")) {
			out.WriteString(assignment)
			written = true
			if isKeyLine(trimmed, "database") {
				continue
			}
		}
		if header {
			table = strings.Trim(trimmed, "[]")
		}
		out.WriteString(line)
	}
	if !written {
		if table != "project" {
			return "", fmt.Errorf("popcornwave.toml: [project] not found")
		}
		out.WriteString(assignment)
	}
	return out.String(), nil
}

// isKeyLine reports whether a trimmed line assigns the named key.
func isKeyLine(trimmed, key string) bool {
	rest, ok := strings.CutPrefix(trimmed, key)
	return ok && strings.HasPrefix(strings.TrimSpace(rest), "=")
}

// addDevboxPackage adds one package to the devbox.json array. The file is
// application-owned, so an unrecognized shape stops the command with the edit
// to make rather than a rewrite that loses the operator's formatting.
func addDevboxPackage(devbox, pkg string) (string, error) {
	if devbox == "" {
		return "", fmt.Errorf("devbox.json not found; add %q to the development environment yourself", pkg)
	}
	if strings.Contains(devbox, strconv.Quote(pkg)) {
		return devbox, nil
	}
	key := strings.Index(devbox, `"packages"`)
	if key < 0 {
		return "", fmt.Errorf(`devbox.json declares no "packages" array; add %q to it yourself`, pkg)
	}
	open := strings.Index(devbox[key:], "[")
	if open < 0 {
		return "", fmt.Errorf(`devbox.json "packages" is not an array; add %q to it yourself`, pkg)
	}
	open += key
	closing := strings.Index(devbox[open:], "]")
	if closing < 0 {
		return "", fmt.Errorf(`devbox.json "packages" is unterminated; add %q to it yourself`, pkg)
	}
	closing += open
	separator := ", "
	if strings.TrimSpace(devbox[open+1:closing]) == "" {
		separator = ""
	}
	return devbox[:closing] + separator + strconv.Quote(pkg) + devbox[closing:], nil
}

// devboxScaffold is the development environment api:cli-init writes and
// api:cli-add installs, so both reach the same file state.
func devboxScaffold(packages []string) string {
	return `{
  "$schema": "https://raw.githubusercontent.com/jetify-com/devbox/0.14.2/.schema/devbox.schema.json",
  "packages": [` + quotedList(packages) + `],
  "shell": {"init_hook": ["echo 'Popcorn Wave development environment'"]}
}
`
}

// tailwindProjectConfig is the assets section api:cli-init scaffolds and
// api:cli-add appends, so both reach the same file state.
func tailwindProjectConfig() string {
	return `
[assets.tailwind]
enabled = true
input = "` + defaultTailwindInput + `"
output = "` + defaultTailwindOutput + `"
minify = true
`
}

// tailwindEntryScaffold writes the CSS entry point. Tailwind scans the sources
// named here, so the entry follows the template purpose: those are exactly the
// directories that can contain a class name.
func tailwindEntryScaffold(scope generationScope) string {
	sources := scope.Templates
	if len(sources) == 0 {
		sources = []string{"handlers", "templates"}
	}
	entry := "@import \"tailwindcss\";\n"
	for _, source := range sources {
		entry += "@source \"../" + source + "\";\n"
	}
	return entry
}
