package pwcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/internal/pwcheck"
	"github.com/shibukawa/popcornweb/internal/pwroutes"
)

func routeRun(t *testing.T, root string, values map[string]string) *checkRun {
	t.Helper()
	config := environmentConfig{Env: "dev", Values: map[string]configValue{}}
	for key, value := range values {
		config.Values[key] = configValue{Key: key, Raw: value}
	}
	return &checkRun{checkContext: checkContext{Env: "dev", Root: root, Config: config}}
}

func TestNoTableIsALimitRatherThanACleanRouteSection(t *testing.T) {
	// A report that looks clean because it did not look is the failure the
	// whole Input mechanism exists to prevent.
	run := routeRun(t, t.TempDir(), nil)

	limit := run.checkRoutes()

	if limit == nil {
		t.Fatal("a project with no table was reported as examined")
	}
	if !strings.Contains(limit.Reason, "pw generate") {
		t.Fatalf("reason = %q, want what to run", limit.Reason)
	}
	if len(run.findings) != 0 {
		t.Fatalf("findings = %v, want none", findingIDs(run.findings))
	}
}

func TestADuplicatePatternNamesBothRegistrations(t *testing.T) {
	// api:serve-mux panics at registration, and the reader chooses which one
	// to remove, so naming one of them chooses for them.
	root := t.TempDir()
	if err := pwroutes.Write(root, &pwroutes.Table{Entries: []pwroutes.Entry{
		{Pattern: "POST /rooms", Origin: pwroutes.OriginApplication, Site: &pwroutes.Site{File: "handlers/a.go", Line: 4}},
		{Pattern: "POST /rooms", Origin: pwroutes.OriginApplication, Site: &pwroutes.Site{File: "handlers/b.go", Line: 9}},
	}}); err != nil {
		t.Fatal(err)
	}
	run := routeRun(t, root, nil)

	if limit := run.checkRoutes(); limit != nil {
		t.Fatalf("limit = %+v, want none", limit)
	}
	if ids := findingIDs(run.findings); len(ids) != 1 || ids[0] != pwcheck.DuplicateRoutePattern {
		t.Fatalf("findings = %v, want PW0201", ids)
	}
	evidence := run.findings[0].Evidence
	if !strings.Contains(evidence, "handlers/a.go:4") || !strings.Contains(evidence, "handlers/b.go:9") {
		t.Fatalf("evidence = %q, want both sides", evidence)
	}
}

func TestAMountCollisionIsFoundForTheEnvironmentBeingDiagnosed(t *testing.T) {
	// The mount paths are runtime configuration, so which mounts exist is a
	// property of the environment rather than of the generated table.
	root := t.TempDir()
	if err := pwroutes.Write(root, &pwroutes.Table{Entries: []pwroutes.Entry{
		{Pattern: "/healthz", Origin: pwroutes.OriginApplication, Site: &pwroutes.Site{File: "handlers/ops.go", Line: 5}},
	}}); err != nil {
		t.Fatal(err)
	}

	enabled := routeRun(t, root, map[string]string{"server.health": "/healthz"})
	enabled.checkRoutes()
	if ids := findingIDs(enabled.findings); len(ids) != 1 || ids[0] != pwcheck.FrameworkMountClash {
		t.Fatalf("findings = %v, want PW0202", ids)
	}
	if !strings.Contains(enabled.findings[0].Message, "server.health") {
		t.Fatalf("message = %q, want the key that enables the mount", enabled.findings[0].Message)
	}

	// The same project with the endpoint unset collides with nothing.
	off := routeRun(t, root, nil)
	off.checkRoutes()
	if ids := findingIDs(off.findings); len(ids) != 0 {
		t.Fatalf("findings = %v, want none when the mount is off", ids)
	}
}

func TestAnUnresolvedRegistrationIsANoteRatherThanASilence(t *testing.T) {
	// The route runs; what is missing is that no other route check covers it,
	// and the OpenAPI document omits it. A clean report that said nothing here
	// would be claiming coverage it does not have.
	root := t.TempDir()
	if err := pwroutes.Write(root, &pwroutes.Table{
		Entries: []pwroutes.Entry{{Pattern: "GET /", Origin: pwroutes.OriginApplication}},
		Unresolved: []pwroutes.Unresolved{
			{Site: pwroutes.Site{File: "handlers/api.go", Line: 30}, Reason: "dynamic_pattern"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	run := routeRun(t, root, nil)

	if limit := run.checkRoutes(); limit != nil {
		t.Fatalf("limit = %+v, want a finding instead", limit)
	}

	if ids := findingIDs(run.findings); len(ids) != 1 || ids[0] != pwcheck.UnresolvedRegistration {
		t.Fatalf("findings = %v, want PW0203", ids)
	}
	if !strings.Contains(run.findings[0].Evidence, "handlers/api.go:30") {
		t.Fatalf("evidence = %q, want the site a reader can open", run.findings[0].Evidence)
	}
	if !strings.Contains(run.findings[0].Evidence, "dynamic_pattern") {
		t.Fatalf("evidence = %q, want why it could not be read", run.findings[0].Evidence)
	}
	// A note rather than an error: the project is not broken, the report is
	// simply blind to that path.
	if run.findings[0].Severity != pwcheck.Note {
		t.Fatalf("severity = %v, want a note", run.findings[0].Severity)
	}
}

func TestAPageRouteAndARegistrationCanStillCollide(t *testing.T) {
	// decision:dual-router-coexistence lets both halves serve, and net/http
	// panics either way, so the check spans them.
	root := t.TempDir()
	if err := pwroutes.Write(root, &pwroutes.Table{Entries: []pwroutes.Entry{
		{Pattern: "GET /{$}", Origin: pwroutes.OriginApplication, Site: &pwroutes.Site{File: "handlers/home.go", Line: 7}},
		{Pattern: "GET /{$}", Origin: pwroutes.OriginPage, Page: "pages/page.pw.html"},
	}}); err != nil {
		t.Fatal(err)
	}
	run := routeRun(t, root, nil)
	run.checkRoutes()

	if ids := findingIDs(run.findings); len(ids) != 1 || ids[0] != pwcheck.DuplicateRoutePattern {
		t.Fatalf("findings = %v, want the collision across the two routers", ids)
	}
	if !strings.Contains(run.findings[0].Evidence, "pages/page.pw.html") {
		t.Fatalf("evidence = %q, want the page named", run.findings[0].Evidence)
	}
}

func TestAnUnreadableTableIsDistinctFromAnAbsentOne(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(pwroutes.File))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, "{not json")
	run := routeRun(t, root, nil)

	limit := run.checkRoutes()

	if limit == nil || !strings.Contains(limit.Reason, "could not be read") {
		t.Fatalf("limit = %+v, want a read failure", limit)
	}
}
