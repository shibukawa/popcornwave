package pwcheck

import (
	"strings"
	"testing"
)

// The identifier is what a reader searches for and what an issue cites, so the
// catalog has to keep them unique, ranged by group, and documented.
func TestCatalogIdentifiersAreWellFormed(t *testing.T) {
	ranges := map[string]string{
		GroupProject:   "PW01",
		GroupRoutes:    "PW02",
		GroupStorage:   "PW03",
		GroupConfig:    "PW04",
		GroupReadiness: "PW05",
	}
	seenTitles := map[string]string{}
	for _, check := range All() {
		prefix, ok := ranges[check.Group]
		if !ok {
			t.Fatalf("%s: unknown group %q", check.ID, check.Group)
		}
		if !strings.HasPrefix(check.ID, prefix) {
			t.Errorf("%s is in group %q, whose range is %sxx", check.ID, check.Group, prefix)
		}
		if len(check.ID) != 6 {
			t.Errorf("%s is not a four-digit identifier", check.ID)
		}
		if check.Title == "" || check.Remedy == "" {
			t.Errorf("%s must carry a title and a remedy", check.ID)
		}
		if previous, clash := seenTitles[check.Title]; clash {
			t.Errorf("%s repeats the title of %s; one condition has one identifier", check.ID, previous)
		}
		seenTitles[check.Title] = check.ID
		if check.Inputs == 0 {
			t.Errorf("%s declares no inputs, so no runner can decide whether it may run", check.ID)
		}
		if check.Phase == 0 {
			t.Errorf("%s declares no phase", check.ID)
		}
	}
}

func TestDocsURLIsDerivedFromTheIdentifier(t *testing.T) {
	check := MustLookup(LiteralSecretInFile)
	want := DocsBase + "#pw0412-a-secret-is-set-from-the-configuration-file"
	if got := check.DocsURL(); got != want {
		t.Fatalf("DocsURL() = %q, want %q", got, want)
	}
}

// Severity is a function of the diagnosed token and the check, and of nothing
// else. A deployed-scope check stays silent in dev rather than being softened
// at the call site.
func TestScopeAndSeverityFollowTheDiagnosedToken(t *testing.T) {
	deployed := MustLookup(QueryDiagnosticsOn)
	if deployed.AppliesTo("dev") {
		t.Error("a deployed-scope check must not apply to dev")
	}
	for _, token := range []string{"prod", "stg", "sandbox"} {
		if !deployed.AppliesTo(token) {
			t.Errorf("a deployed-scope check must apply to %q, which is not dev", token)
		}
	}
	both := MustLookup(LiteralSecretInFile)
	if got := both.SeverityFor("dev"); got != Note {
		t.Errorf("dev severity = %v, want note", got)
	}
	if got := both.SeverityFor("prod"); got != Error {
		t.Errorf("prod severity = %v, want error", got)
	}
	development := MustLookup(ProviderInjectedDev)
	if development.AppliesTo("prod") {
		t.Error("a development-scope check must not apply to a deployment")
	}
}

func TestNeedsReportsDeclaredInputs(t *testing.T) {
	check := MustLookup(MissingSQLDriver)
	if !check.Needs(ImportGraph) {
		t.Error("the missing-driver check needs the import graph")
	}
	if check.Needs(Network) {
		t.Error("the missing-driver check must not need the network")
	}
}
