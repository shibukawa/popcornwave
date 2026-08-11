package pwcli

import (
	"bytes"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/internal/pwgen"
	"github.com/shibukawa/tinybind-go/routetree"
)

// generateFixtureTree runs the page tree generator over one fixture tree with
// one emitter, through the same computation planPageTrees uses.
//
// The tree is named because the two emitters cannot share the tree that
// declares a server action: an action handler is written for one transport, and
// the fasthttp shape recognizer correctly refuses the net/http one. Generating
// the second transport's tree from the first transport's source is not a real
// scenario, since the transform rewrites the handler first.
func generateFixtureTree(t *testing.T, tree string, emitter *routetree.Emitter) map[string]string {
	t.Helper()
	root, _ := fixtureConfig(t)
	module, moduleDir, err := moduleImportPath(root)
	if err != nil {
		t.Fatal(err)
	}
	treeRoot := filepath.Join(root, tree)
	importBase, err := treeImportPath(module, moduleDir, treeRoot)
	if err != nil {
		t.Fatal(err)
	}
	files, err := routetree.Generate(routetree.GenerateOptions{
		Config:          pwgen.PageConfig(treeRoot, importBase),
		Emitter:         emitter,
		ComponentSuffix: pwgen.PageComponentSuffix,
		DecoderOutput:   pwgen.PageDecoderOutput,
		RegistryOutput:  pwgen.PageRegistryOutput,
	})
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}
	out := map[string]string{}
	for _, file := range files {
		out[filepath.Base(file.Path)] = string(file.Source)
	}
	return out
}

// The second transport's page tree comes out of the same templates with a
// different symbol table, so what is asserted is that the symbols reached the
// output: the router, the request type, and the two pattern spellings a trie
// router does not share with Go 1.22.
func TestTheFastPageEmitterWritesTheSecondTransportsTree(t *testing.T) {
	emitter, err := pwgen.FastPageEmitter()
	if err != nil {
		t.Fatal(err)
	}
	files := generateFixtureTree(t, "fastpages", emitter)

	registry, ok := files[pwgen.PageRegistryOutput]
	if !ok {
		t.Fatalf("no registry was emitted; got %v", emittedNames(files))
	}
	for _, want := range []string{
		"pwfastpage.Router",
		"*fasthttp.RequestCtx",
		"github.com/shibukawa/popcornwave/pwfastpage",
	} {
		if !strings.Contains(registry, want) {
			t.Errorf("the registry does not name %q:\n%s", want, registry)
		}
	}
	// net/http must not be imported: this is the tree the second build
	// compiles, and the first transport appearing there is the thing the split
	// exists to avoid.
	if strings.Contains(registry, `"net/http"`) {
		t.Errorf("the registry imported net/http:\n%s", registry)
	}
	// The root pattern is the one that fails silently when wrong. A trie router
	// reads "{$}" as a parameter named "$" and installs the route somewhere
	// else without complaining.
	if strings.Contains(registry, `HandleFunc("GET /{$}"`) {
		t.Errorf("the registry registered the Go 1.22 root marker verbatim:\n%s", registry)
	}
	if !strings.Contains(registry, `HandleFunc("GET /"`) {
		t.Errorf("the root was not registered under the router's own spelling:\n%s", registry)
	}
}

// The decoder reads through the framework accessors rather than off the request
// value, which is what lets one template serve either transport.
func TestTheFastDecoderReadsThroughTheFrameworkAccessors(t *testing.T) {
	emitter, err := pwgen.FastPageEmitter()
	if err != nil {
		t.Fatal(err)
	}
	files := generateFixtureTree(t, "fastpages", emitter)

	decoder, ok := files[pwgen.PageDecoderOutput]
	if !ok || !strings.Contains(decoder, "func DecodeRoute") {
		t.Fatalf("no route decoder was emitted; got %v", emittedNames(files))
	}
	if strings.Contains(decoder, "r.PathValue(") || strings.Contains(decoder, "r.URL") {
		t.Errorf("the decoder read off the request value:\n%s", decoder)
	}
	for _, want := range []string{"pwfast.PathValue(r,", "pwfast.Queries(r)", "pwfast.QueryLookup("} {
		if !strings.Contains(decoder, want) {
			t.Errorf("the decoder does not call %q:\n%s", want, decoder)
		}
	}
	if !strings.Contains(decoder, "*fasthttp.RequestCtx") {
		t.Errorf("the decoder does not take the second transport's request:\n%s", decoder)
	}
}

// Both emitters run over one fixture, which is what shows the difference is a
// symbol table rather than two template sets that could drift.
func TestBothEmittersProduceATreeFromOneFixture(t *testing.T) {
	netHTTP, err := pwgen.PageEmitter()
	if err != nil {
		t.Fatal(err)
	}
	fastHTTP, err := pwgen.FastPageEmitter()
	if err != nil {
		t.Fatal(err)
	}
	first, second := generateFixtureTree(t, "fastpages", netHTTP), generateFixtureTree(t, "fastpages", fastHTTP)
	if len(first) != len(second) {
		t.Errorf("the two emitters produced %d and %d files", len(first), len(second))
	}
	for name := range first {
		if _, ok := second[name]; !ok {
			t.Errorf("the second transport is missing %s", name)
		}
	}
}

// A page tree is not transport-shaped throughout, so only the part that is gets
// a second copy. The compiled components render into an io.Writer and name
// nothing about the request; the route decoder and the registry read the request
// and install on a router.
//
// What decides it is the bytes rather than a list of names: this asserts the
// outcome, and planFastPageTrees reaches it by emitting the whole tree twice and
// keeping what differed.
func TestTheSecondTransportsPageTreeIsOnlyTheFilesThatDiffer(t *testing.T) {
	root, _ := fixtureConfig(t)
	config := projectConfig{Generate: generationScope{Pages: []string{"fastpages"}}, FastHTTP: true}
	changes, err := planFastPageTrees(root, config, config.FastHTTP, nil)
	if err != nil {
		t.Fatal(err)
	}
	planned := map[string][]byte{}
	for _, change := range changes {
		if change.remove {
			t.Errorf("%s was planned for removal; nothing of this tree is generated yet", change.path)
			continue
		}
		planned[filepath.Base(change.path)] = change.source
	}
	for _, name := range []string{"route_fast_pw_gen.go", "routes_fast_pw_gen.go"} {
		source, ok := planned[name]
		if !ok {
			t.Errorf("%s was not planned; got %v", name, plannedNames(planned))
			continue
		}
		if constraint, _ := buildConstraint(source); constraint != strings.TrimSpace(fastHTTPConstraint) {
			t.Errorf("%s is not constrained to the fasthttp build:\n%s", name, source)
		}
		if bytes.Contains(source, []byte(`"net/http"`)) {
			t.Errorf("%s names the first transport:\n%s", name, source)
		}
	}
	// A component both emitters produce identically belongs to both builds, so a
	// second copy of it would be a second declaration under the fasthttp tag.
	for _, name := range []string{"layout_fast_pw_gen.go", "page_fast_pw_gen.go"} {
		if _, ok := planned[name]; ok {
			t.Errorf("%s was copied for the second transport although both emitters produce it identically", name)
		}
	}
}

// A project that declared no second build gets none of this, which is what
// keeps the option free to not take.
func TestNoSecondPageTreeWithoutTheDeclaration(t *testing.T) {
	root, _ := fixtureConfig(t)
	config := projectConfig{Generate: generationScope{Pages: []string{"fastpages"}}}
	changes, err := planFastPageTrees(root, config, config.FastHTTP, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) > 0 {
		t.Errorf("a project without the declaration planned %d files", len(changes))
	}
}

func plannedNames(planned map[string][]byte) []string {
	names := make([]string, 0, len(planned))
	for name := range planned {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// emittedNames lists what a generation run produced, for a failure message.
func emittedNames(files map[string]string) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	return names
}

// The handler-shape recognizer is what keeps a transport from reading the
// other's handler as a malformed page, and this is the case that proves it: the
// committed fixture declares a net/http action, and the fasthttp emitter
// refuses it by name rather than emitting something that would not compile.
func TestTheFastEmitterRefusesTheOtherTransportsActionHandler(t *testing.T) {
	emitter, err := pwgen.FastPageEmitter()
	if err != nil {
		t.Fatal(err)
	}
	root, _ := fixtureConfig(t)
	module, moduleDir, err := moduleImportPath(root)
	if err != nil {
		t.Fatal(err)
	}
	treeRoot := filepath.Join(root, "pages")
	importBase, err := treeImportPath(module, moduleDir, treeRoot)
	if err != nil {
		t.Fatal(err)
	}
	_, err = routetree.Generate(routetree.GenerateOptions{
		Config:          pwgen.PageConfig(treeRoot, importBase),
		Emitter:         emitter,
		ComponentSuffix: pwgen.PageComponentSuffix,
		DecoderOutput:   pwgen.PageDecoderOutput,
		RegistryOutput:  pwgen.PageRegistryOutput,
	})
	if err == nil {
		t.Fatal("a net/http action handler was accepted by the fasthttp emitter")
	}
	if !strings.Contains(err.Error(), "Rename") {
		t.Errorf("the refusal did not name the action: %v", err)
	}
}
