// Package pwcheck holds the diagnostic check catalog.
//
// A condition worth reporting has one identifier, one title, one severity rule,
// and one remedy, wherever it is detected. Startup validation and pw doctor see
// different inputs -- a process knows its environment variables and its actual
// registrations, while the host tooling knows the project files and every other
// environment's configuration -- so a check declares what it needs and a runner
// skips it rather than guessing. The catalog is data, and this package imports
// nothing of the framework, so both sides can read it.
package pwcheck

import (
	"sort"
	"strings"
)

// Severity ranks a finding. Error means the process would refuse to start or a
// policy forbids the value in the diagnosed environment; Warning means it
// starts and the value is inadvisable there; Note states something worth seeing.
type Severity int

const (
	Note Severity = iota
	Warning
	Error
)

func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	default:
		return "note"
	}
}

// Scope selects the environments a check applies to. No check hard-codes a list
// of deployment names: data:runtime-environment treats any unknown token as a
// deployment, so the question is always whether the diagnosed token is dev.
type Scope int

const (
	// Every applies regardless of environment.
	Every Scope = iota
	// Deployed applies to every token other than dev.
	Deployed
	// Development applies only to dev, to state an arrangement that exists
	// only there.
	Development
	// Production applies only to prod, reserved for a policy that names it.
	Production
)

// Input names a fact a check reads. A runner that cannot build an input skips
// every check declaring it and says so, because a report that looks clean
// because it did not look is worse than one with a gap in it.
type Input uint16

const (
	// Config is the merged configuration for the diagnosed environment.
	Config Input = 1 << iota
	// ImportGraph is the resolved import set of the application main package.
	ImportGraph
	// ProjectFiles are the files of the project tree.
	ProjectFiles
	// RouteTable is the exported route analysis result.
	RouteTable
	// ProcessEnv is the environment of the process running the check.
	ProcessEnv
	// Network reaches the configured database or provider.
	Network
	// OtherEnvironments is every other environment's configuration file, which
	// only a reader of the project tree has.
	OtherEnvironments
)

// Phase records where a check can run.
type Phase uint8

const (
	// Doctor runs only in pw doctor.
	Doctor Phase = 1 << iota
	// Startup runs during application startup validation.
	Startup
)

// Check is one catalog entry.
type Check struct {
	ID string
	// Title is the one-line form printed after the identifier.
	Title string
	// Severity is the severity outside dev, or everywhere for an Every check.
	Severity Severity
	// DevSeverity is the severity in dev. A check whose consequence is milder
	// on a development machine softens here instead of being two checks.
	DevSeverity Severity
	Scope       Scope
	Inputs      Input
	Phase       Phase
	// Remedy is the fixed part of the advice; a finding may add the specifics.
	Remedy string
	// Group names the catalog section, matching the .knowledge rule concept.
	Group string
}

// Catalog sections, matching the identifier ranges.
const (
	GroupProject   = "project"
	GroupRoutes    = "routes"
	GroupStorage   = "storage"
	GroupConfig    = "configuration"
	GroupReadiness = "readiness"
)

// DocsBase is the diagnostics reference on the framework site. The page is
// generated from this catalog, so an identifier a report prints always has an
// entry to link to, and a check added without documentation fails the test that
// compares the generated page with the checked-in one.
const DocsBase = "https://shibukawa.github.io/popcornwave/appendix/diagnostics/"

// DocsURL returns the entry for this check. The anchor is the heading the
// generated page writes for it.
func (c Check) DocsURL() string {
	return DocsBase + "#" + strings.ToLower(c.ID) + "-" + anchorSlug(c.Title)
}

// anchorSlug renders a heading the way the documentation site does.
func anchorSlug(title string) string {
	var out strings.Builder
	previousDash := false
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
			previousDash = false
		case !previousDash && out.Len() > 0:
			out.WriteByte('-')
			previousDash = true
		}
	}
	return strings.TrimSuffix(out.String(), "-")
}

// AppliesTo reports whether the check is evaluated for env.
func (c Check) AppliesTo(env string) bool {
	switch c.Scope {
	case Deployed:
		return env != "dev"
	case Development:
		return env == "dev"
	case Production:
		return env == "prod" || env == "production"
	default:
		return true
	}
}

// SeverityFor resolves the severity for env. Severity is a function of the
// diagnosed token and the check, and of nothing else.
func (c Check) SeverityFor(env string) Severity {
	if env == "dev" {
		return c.DevSeverity
	}
	return c.Severity
}

// Needs reports whether the check declares every input in want.
func (c Check) Needs(want Input) bool { return c.Inputs&want == want }

var catalog = map[string]Check{}

func register(checks ...Check) {
	for _, check := range checks {
		if _, exists := catalog[check.ID]; exists {
			panic("pwcheck: duplicate check identifier " + check.ID)
		}
		catalog[check.ID] = check
	}
}

// MustLookup returns the check, and panics when the identifier is unknown. A
// caller reporting a finding names a compile-time constant, so an unknown
// identifier is a programming error rather than a runtime condition.
func MustLookup(id string) Check {
	check, ok := catalog[id]
	if !ok {
		panic("pwcheck: unknown check identifier " + id)
	}
	return check
}

// All returns every check, ordered by identifier, for documentation generation
// and for the tests that assert the catalog is complete.
func All() []Check {
	all := make([]Check, 0, len(catalog))
	for _, check := range catalog {
		all = append(all, check)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	return all
}
