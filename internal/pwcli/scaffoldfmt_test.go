package pwcli

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/templatefmt"
)

// scaffoldVariants covers every shape pw init writes template sources for, so
// a literal that drifts from canonical form fails here rather than in the
// first pw fmt --check of a new project.
func scaffoldVariants() map[string]initOptions {
	base := defaultInitOptions()
	base.Name = "demo"
	variants := map[string]initOptions{}
	add := func(name string, edit func(*initOptions)) {
		options := base
		edit(&options)
		variants[name] = options
	}
	add("default", func(*initOptions) {})
	add("discovered", func(o *initOptions) { o.Router = routerDiscovered })
	add("both routers with tailwind", func(o *initOptions) {
		o.Router = routerBoth
		o.Tailwind = true
	})
	add("host toolchain", func(o *initOptions) { o.TinyGo = false })
	add("dynamo and firestore", func(o *initOptions) {
		o.Dynamo = true
		o.Firestore = true
	})
	add("login", func(o *initOptions) {
		o.Auth = authOIDC
		o.AuthEmulator = true
	})
	add("package", func(o *initOptions) {
		o.Kind = kindPackage
		o.Name = "github.com/example/mycomponent"
	})
	return variants
}

// TestScaffoldSourcesAreCanonical asserts that every template source pw init
// writes is already what pw fmt would leave behind: formatting it again
// changes nothing, and every identified source actually parses — the
// canonicalization helper falls back to the authored bytes on a parse error,
// and this is the check that keeps that fallback from hiding a broken literal.
func TestScaffoldSourcesAreCanonical(t *testing.T) {
	options := fmtOptions{}.formatOptions()
	for name, variant := range scaffoldVariants() {
		t.Run(name, func(t *testing.T) {
			templates := 0
			for sourcePath, content := range scaffoldFiles(variant) {
				format, err := templatefmt.Identify(path.Base(sourcePath), options)
				if err != nil {
					continue
				}
				templates++
				formatted, err := templatefmt.SourceAs(format, sourcePath, []byte(content), options)
				if err != nil {
					t.Errorf("%s does not parse: %v", sourcePath, err)
					continue
				}
				if string(formatted) != content {
					t.Errorf("%s is not canonical; pw fmt would rewrite it", sourcePath)
				}
			}
			if templates == 0 && variant.Kind != kindPackage {
				t.Fatal("the variant scaffolded no template source, so this test checked nothing")
			}
		})
	}
}

// TestCapabilityPlanCreatesCanonicalSources covers the other write path: a
// file pw add or pw new creates is formatted on the same terms as the init
// scaffold, while appends into files the application owns pass through
// untouched.
func TestCapabilityPlanCreatesCanonicalSources(t *testing.T) {
	root := t.TempDir()
	appended := filepath.Join(root, "queries", "existing.pw.sql")
	if err := os.MkdirAll(filepath.Dir(appended), 0o755); err != nil {
		t.Fatal(err)
	}
	// Deliberately non-canonical: single quotes and an unindented body.
	authored := "package queries\n\nexport statement Find(id: int): sql.one<Example> {\nSELECT id FROM example WHERE id = {id} AND name != ''\n}\n"
	if err := os.WriteFile(appended, []byte(authored), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := newCapabilityPlan()
	plan.creates["queries/users.pw.sql"] = authored
	plan.appends["queries/existing.pw.sql"] = "\n-- trailing note\n"
	changes, err := plan.changes(root)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, change := range changes {
		byPath[filepath.ToSlash(strings.TrimPrefix(change.path, root+string(filepath.Separator)))] = string(change.source)
	}
	created := byPath["queries/users.pw.sql"]
	if created == authored {
		t.Fatal("a created source was written as authored rather than canonical")
	}
	formatted, err := templatefmt.SourceAs(templatefmt.SQL, "users.pw.sql", []byte(created), fmtOptions{}.formatOptions())
	if err != nil {
		t.Fatal(err)
	}
	if string(formatted) != created {
		t.Fatal("the created source is still not canonical")
	}
	if got := byPath["queries/existing.pw.sql"]; got != authored+"\n-- trailing note\n" {
		t.Fatalf("an append was reformatted; the file belongs to the application\n%s", got)
	}
}
