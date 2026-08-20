package pwlsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/internal/pwroutes"
)

func writeRouteTable(t *testing.T, root string, table *pwroutes.Table) {
	t.Helper()
	if err := pwroutes.Write(root, table); err != nil {
		t.Fatal(err)
	}
}

func TestTheViewReadsTheRegisteredHalfFromTheTable(t *testing.T) {
	// Finding it needs the resolved import graph, so the view reads what
	// api:cli-generate wrote rather than running that analysis a second time.
	root, project := writeProject(t)
	writeRouteTable(t, root, &pwroutes.Table{Entries: []pwroutes.Entry{
		{Pattern: "POST /rooms", Origin: pwroutes.OriginApplication,
			Site: &pwroutes.Site{File: "handlers/rooms.go", Line: 12}, Handler: "createRoom"},
	}})

	report := routesOf(project)

	var registered *Route
	for index, route := range report.Routes {
		if route.Path == "POST /rooms" {
			registered = &report.Routes[index]
		}
	}
	if registered == nil {
		t.Fatalf("routes = %+v, want the registered one", report.Routes)
	}
	if registered.Handler != "createRoom" {
		t.Fatalf("route = %+v, want the handler identity", registered)
	}
	if !strings.HasSuffix(registered.PageURI, "handlers/rooms.go") {
		t.Fatalf("uri = %q, want the registration site", registered.PageURI)
	}
}

func TestTheViewStillWalksThePageTreeItself(t *testing.T) {
	// The table is only as fresh as the last generation; the tree is read from
	// the sources, so a page added a moment ago is there.
	root, project := writeProject(t)
	writeRouteTable(t, root, &pwroutes.Table{})

	report := routesOf(project)

	if len(report.Routes) == 0 {
		t.Fatal("an empty table emptied the view")
	}
	for _, route := range report.Routes {
		if strings.HasPrefix(route.Path, "/") && route.Page != "" {
			return
		}
	}
	t.Fatalf("routes = %+v, want a page tree route", report.Routes)
}

func TestNoTableSaysSoRatherThanShowingHalfTheURLSpace(t *testing.T) {
	// A view that silently covers one router reads as if it covered both,
	// which is the failure decision:dual-router-coexistence makes easy.
	_, project := writeProject(t)

	report := routesOf(project)

	if report.NotCovered == "" {
		t.Fatal("a project with no table claimed to cover everything")
	}
	if !strings.Contains(report.NotCovered, "pw generate") {
		t.Fatalf("notCovered = %q, want what to run", report.NotCovered)
	}
}

func TestUnresolvedRegistrationsAreStatedAsALimit(t *testing.T) {
	root, project := writeProject(t)
	writeRouteTable(t, root, &pwroutes.Table{
		Entries: []pwroutes.Entry{{Pattern: "GET /", Origin: pwroutes.OriginApplication}},
		Unresolved: []pwroutes.Unresolved{
			{Site: pwroutes.Site{File: "handlers/api.go", Line: 3}, Reason: "dynamic_pattern"},
		},
	})

	report := routesOf(project)

	if !strings.Contains(report.NotCovered, "one registration") {
		t.Fatalf("notCovered = %q, want the unresolved count", report.NotCovered)
	}
}

func TestTheTableIsReadFromTheProjectRoot(t *testing.T) {
	root, project := writeProject(t)
	if _, err := os.Stat(filepath.Join(root, "dist")); err == nil {
		t.Fatal("the fixture already has a dist directory")
	}
	writeRouteTable(t, root, &pwroutes.Table{Entries: []pwroutes.Entry{
		{Pattern: "GET /health", Origin: pwroutes.OriginApplication},
	}})

	for _, route := range routesOf(project).Routes {
		if route.Path == "GET /health" {
			return
		}
	}
	t.Fatal("the table under the project root was not read")
}
