package pwcli

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/shibukawa/popcornweb/internal/pwgen"
	"github.com/shibukawa/popcornweb/internal/pwscript"
	"github.com/shibukawa/tinybind-go/generator"
	"github.com/shibukawa/tinybind-go/routetree"
	templatehtmlbind "github.com/shibukawa/tinybind-go/templates/htmlbind"
	"golang.org/x/mod/modfile"
)

// planPageTrees generates every configured page tree and returns its output
// grouped by the directory it belongs in.
//
// Nothing is written here. The artifacts join the plan of the directory they
// land in, so one directory keeps one staleness sweep and one atomic write.
//
// A tree's extracted assets come back beside its Go, in their own list, because
// an asset has no path this package can compute: a compiled component belongs
// beside its template and an asset belongs wherever PublicURLBase is served
// from. They are grouped under the root rather than under a template's own
// directory for the same reason — the URL they were compiled against is one
// place, not one per page. The tree root is what they are grouped under,
// because that is a directory whose purposes admit them.
func planPageTrees(root string, config projectConfig, messages messagePlan) (map[string][]generator.Artifact, error) {
	planned, _, err := planPageTreeFiles(root, config, messages)
	return planned, err
}

// planPageTreeFiles is planPageTrees plus the typed server actions each route
// package declared, keyed by the absolute directory holding them.
//
// The two halves of a typed action are built by different phases: routetree
// reads the declaration, because it parses a route package before that package
// can compile, and the binding phase builds the argument struct and the codecs,
// because it type-checks. So the list has to travel from the first to the
// second, and this is where it is picked up.
func planPageTreeFiles(root string, config projectConfig, messages messagePlan) (map[string][]generator.Artifact, map[string][]routetree.Action, error) {
	if len(config.Generate.Pages) == 0 {
		return nil, nil, nil
	}
	module, moduleDir, err := moduleImportPath(root)
	if err != nil {
		return nil, nil, err
	}
	emitter, err := pwgen.PageEmitter()
	if err != nil {
		return nil, nil, err
	}
	planned := map[string][]generator.Artifact{}
	actions := map[string][]routetree.Action{}
	for _, relative := range config.Generate.Pages {
		treeRoot := filepath.Join(root, filepath.FromSlash(relative))
		importBase, err := treeImportPath(module, moduleDir, treeRoot)
		if err != nil {
			return nil, nil, err
		}
		result, err := generatePageTree(treeRoot, importBase, emitter, messages)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", relative, err)
		}
		treeRootAbs, err := filepath.Abs(treeRoot)
		if err != nil {
			return nil, nil, err
		}
		for _, asset := range result.Assets {
			// Under the tree root rather than the project root: an artifact is
			// filtered by the purposes of the directory it is planned into, and
			// the project root is not a page directory, so anything grouped there
			// is dropped without a word.
			planned[treeRootAbs] = append(planned[treeRootAbs], generator.Artifact{
				Kind:        assetArtifactKind(asset),
				Destination: generator.DestinationPublicAsset,
				OutputBase:  asset.Base,
				Extension:   asset.Extension,
				Content:     asset.Content,
				PublicPath:  asset.URL,
			})
		}
		for _, file := range result.Files {
			artifact, err := pageArtifact(file)
			if err != nil {
				return nil, nil, err
			}
			directory, err := filepath.Abs(filepath.Dir(file.Path))
			if err != nil {
				return nil, nil, err
			}
			planned[directory] = append(planned[directory], artifact)
			// The typed actions of this package travel to the phase that emits
			// their wrappers. It is keyed by directory because that is what the
			// binding phase is handed, and RelDir is what selects them.
			if declared := typedActionsIn(result.Actions, treeRootAbs, directory); len(declared) > 0 {
				actions[directory] = declared
			}
			// The registry knows both halves nothing else holds together: which
			// route a pattern is, and which server functions its package
			// exports. A component script calling one by name needs the join,
			// and only a file that has read both can make it.
			if registration, declares := pageActionRegistrationArtifact(artifact); declares {
				planned[directory] = append(planned[directory], registration)
			}
		}
	}
	return planned, actions, nil
}

// typedActionsIn selects the actions a route package declared, by matching the
// directory the binding phase will type-check against the relative directory
// routetree reported.
func typedActionsIn(all []routetree.Action, treeRoot, directory string) []routetree.Action {
	relative, err := filepath.Rel(treeRoot, directory)
	if err != nil {
		return nil
	}
	if relative == "." {
		relative = ""
	}
	relative = filepath.ToSlash(relative)
	var out []routetree.Action
	for _, action := range all {
		if action.Typed && action.RelDir == relative {
			out = append(out, action)
		}
	}
	return out
}

// GenerateTree rather than Generate: the latter is documented as the variant
// that discards the extracted assets, and a page declaring a script block
// through it produced a reference to a file that answered 404.
//
// Two options travel that the tree used to take by omission. The URL base, or
// every tree asset is compiled against the module default regardless of where
// this writes them. And the attribute prefix — both paths happen to use the
// module default today, which is why one document has held one spelling, and
// passing it is what keeps that true if this framework ever brands the prefix
// rather than leaving a page tree quietly on the default.
func generatePageTree(treeRoot, importBase string, emitter *routetree.Emitter, messages messagePlan) (routetree.Result, error) {
	// A page template reads the same bindings and resolves against the same
	// symbol table a flat template does. The two compile paths taking different
	// options is exactly the failure requirement:route-package-context-externals
	// records upstream, where a seam reached one path and not the other and
	// shipped a feature that was simply absent on filesystem routes.
	bindings := pwgen.MessageBindings()
	symbols := pwgen.MessageSymbolTable(messages.Symbols, messages.ImportPath)
	return routetree.GenerateTree(routetree.GenerateOptions{
		ImplicitBindings:      bindings,
		Messages:              symbols,
		MessageContextBinding: pwgen.MessageLocaleBinding,
		Config:                pwgen.PageConfig(treeRoot, importBase),
		Emitter:               emitter,
		ComponentSuffix:       pwgen.PageComponentSuffix,
		DecoderOutput:         pwgen.PageDecoderOutput,
		RegistryOutput:        pwgen.PageRegistryOutput,
		PublicURLBase:         generator.DefaultPublicURLBase,
		DataAttributePrefix:   pwgen.AttributePrefix(),
		ScriptResolver:        resolveComponentScripts,
	})
}

// resolveComponentScripts answers what the module asks about a component's
// script block, which is the seam that lets it lower a named handler and emit a
// named parameter without reading any JavaScript itself.
//
// The reading is internal/pwscript, and what it declines to read is the point:
// a component whose block it could not understand is left out of Handlers
// entirely, which the module documents as unchecked. Reporting every name of
// such a block as unresolved would fail a build over this scanner's limits
// rather than over the author's code.
func resolveComponentScripts(path string, scripts []templatehtmlbind.ComponentScript) (routetree.ScriptAnswers, error) {
	answers := routetree.ScriptAnswers{
		Handlers:   map[string]templatehtmlbind.ClientHandlerSet{},
		Parameters: map[string][]string{},
	}
	for _, script := range scripts {
		block, err := pwscript.Read(script.Script)
		if err != nil {
			// A block this scanner cannot walk at all is a different thing from
			// one it walks and does not understand, and only the first is worth
			// stopping for: it means the block is not the JavaScript it claims
			// to be, which the browser will meet next.
			return routetree.ScriptAnswers{}, fmt.Errorf("%s: component %s: %w", path, script.Component, err)
		}
		if block.Unread != "" {
			continue
		}
		set := templatehtmlbind.ClientHandlerSet{Resolved: block.Handlers}
		// A name the markup referenced and the block does not publish is refused
		// here rather than left to the module's own comparison, so the reason
		// travels with it and an author reads what to change.
		for _, referenced := range script.Handlers {
			if slices.Contains(block.Handlers, referenced) {
				continue
			}
			if set.Unresolved == nil {
				set.Unresolved = map[string]string{}
			}
			set.Unresolved[referenced] = "the component's script block returns no handler by that name"
		}
		answers.Handlers[script.Component] = set
		// Only parameters the component actually declares. The block asking for
		// one it does not have is the author's mistake and belongs in a
		// diagnostic, not in an emitted object naming nothing.
		var emit []string
		for _, wanted := range block.Parameters {
			if slices.Contains(script.Parameters, wanted) {
				emit = append(emit, wanted)
			}
		}
		if len(emit) > 0 {
			answers.Parameters[script.Component] = emit
		}
	}
	return answers, nil
}

// planSecondBuildPages is the second transport's page tree step of one run.
//
// It exists to hold the three cases apart, because they are decided by facts
// this step does not otherwise see: whether the project declares the second
// build, and whether the run is allowed to write.
func planSecondBuildPages(root string, config projectConfig, check bool, changes []fileChange, messages messagePlan) ([]fileChange, error) {
	if !config.FastHTTP {
		// Sweeping runs whether or not the declaration holds. Turning it off has
		// to take these files with it, or the tree keeps a half that compiles
		// under a tag nothing writes for any more.
		return planFastPageTrees(root, config, false, changes, messages)
	}
	if check {
		// Check mode writes nothing, so it can only judge this step when its
		// input is already current. With anything still to write, the project is
		// stale whatever this step would find, and running it against the tree as
		// it stands would report the absence of a handler this run was about to
		// generate rather than the staleness that actually holds.
		if len(changes) > 0 {
			return changes, nil
		}
		return planFastPageTrees(root, config, true, changes, messages)
	}
	if len(changes) > 0 {
		// This step reads the derived handlers the stages above produced, so what
		// they planned is written before it looks.
		if err := applyFileChanges(changes); err != nil {
			return nil, err
		}
	}
	return planFastPageTrees(root, config, true, changes, messages)
}

// planFastPageTrees plans the second transport's copy of every page tree, and
// sweeps whatever it no longer produces.
//
// It runs after the derived handlers are on disk, and that ordering is the
// whole reason it is a step of its own rather than part of planPageTrees. A
// server action is discovered by its signature, so the fasthttp-shaped one this
// emitter looks for is generated code: run any earlier and a tree declaring an
// action is refused for want of a handler that this run had not written yet.
//
// A page tree is not transport-shaped throughout. A compiled component renders
// into an io.Writer and names nothing about the request, so both emitters
// produce it byte for byte; the route decoder and the registry read the request
// and install on a router, so both are per transport. Emitting the whole tree
// twice and comparing is what decides which is which, rather than a list of file
// names kept in agreement with an emitter this framework does not own — and it
// is why the net/http tree is generated again here rather than carried down from
// the first pass, which would tie this step to how that one happened to group
// its output.
func planFastPageTrees(root string, config projectConfig, derive bool, changes []fileChange, messages messagePlan) ([]fileChange, error) {
	if len(config.Generate.Pages) == 0 {
		return changes, nil
	}
	module, moduleDir, err := moduleImportPath(root)
	if err != nil {
		return nil, err
	}
	for _, relative := range config.Generate.Pages {
		treeRoot := filepath.Join(root, filepath.FromSlash(relative))
		expected := map[string]bool{}
		if derive {
			importBase, err := treeImportPath(module, moduleDir, treeRoot)
			if err != nil {
				return nil, err
			}
			derived, err := deriveTreeFor(treeRoot, importBase, messages)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", relative, err)
			}
			for _, file := range derived {
				expected[file.Path] = true
				changes, err = appendIfChanged(changes, file.Path, file.Source)
				if err != nil {
					return nil, err
				}
			}
		}
		changes, err = sweepFastPages(treeRoot, expected, changes)
		if err != nil {
			return nil, err
		}
	}
	return changes, nil
}

// deriveTreeFor emits one tree for both transports and returns the second
// transport's half: the files the two emitters did not agree on, named apart
// and constrained to the build that compiles them.
func deriveTreeFor(treeRoot, importBase string, messages messagePlan) ([]routetree.Generated, error) {
	netHTTP, err := pwgen.PageEmitter()
	if err != nil {
		return nil, err
	}
	fastHTTP, err := pwgen.FastPageEmitter()
	if err != nil {
		return nil, err
	}
	shared, err := generatePageTree(treeRoot, importBase, netHTTP, messages)
	if err != nil {
		return nil, err
	}
	files, err := generatePageTree(treeRoot, importBase, fastHTTP, messages)
	if err != nil {
		return nil, err
	}
	// The Go only. A tree's extracted assets are transport-independent — one
	// file, one URL, compiled from one template — so the second build reuses
	// what the first already planned rather than writing them twice.
	out := make([]routetree.Generated, 0, len(files.Files))
	for _, file := range files.Files {
		if same(shared.Files, file) {
			continue
		}
		out = append(out, routetree.Generated{
			Path:   fastPagePath(file.Path),
			Source: append([]byte(fastHTTPConstraint), file.Source...),
		})
	}
	return out, nil
}

// same reports whether the other emitter produced this file identically, which
// makes it the shared one rather than the second transport's.
func same(shared []routetree.Generated, file routetree.Generated) bool {
	for _, candidate := range shared {
		if candidate.Path == file.Path {
			return bytes.Equal(candidate.Source, file.Source)
		}
	}
	return false
}

// sweepFastPages deletes what this step used to produce and no longer does.
//
// The per-directory sweep cannot do it: it runs before this step and would
// delete every one of these files on every run, so it spares them by name and
// the producer cleans up after itself instead.
func sweepFastPages(treeRoot string, expected map[string]bool, changes []fileChange) ([]fileChange, error) {
	err := filepath.WalkDir(treeRoot, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir(), !isFastPagePath(path), expected[path]:
			return nil
		}
		changes = append(changes, fileChange{path: path, remove: true})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return changes, nil
}

// fastHTTPConstraint admits a generated file into the fasthttp build alone. It
// is the counterpart of netHTTPConstraint, and is written the same way: above
// the generated-code header, with the blank line that keeps it a constraint
// rather than an ordinary comment.
const fastHTTPConstraint = "//go:build fasthttp\n\n"

// fastPagePath is where the second transport's copy of one page tree file goes.
//
// The infix goes before the generated suffix rather than after, so the file
// still ends the way every generated file in a project does: the .gitignore, the
// editor rules, and the staleness sweep all key on that suffix and none of them
// needs to learn a second one.
func fastPagePath(path string) string {
	directory, name := filepath.Split(path)
	return filepath.Join(directory,
		strings.TrimSuffix(name, pwgen.PageComponentSuffix)+fastPageSuffix)
}

func isFastPagePath(path string) bool {
	return strings.HasSuffix(filepath.Base(path), fastPageSuffix)
}

// fastPageSuffix ends every file the second transport's page tree run writes,
// and nothing else.
const fastPageSuffix = "_fast" + pwgen.PageComponentSuffix

// withPageDirectories adds every directory a page tree writes into. A tree root
// holding only subdirectories has no source the package walk would find, and it
// is where the registry goes.
//
// Both inputs are resolved to absolute paths first. A page tree run reports
// where it wrote, and the package walk reports what it found: one directory
// reached by two spellings would be planned twice, and the plan without the
// tree's artifacts would delete what the other one just wrote.
func withPageDirectories(directories []string, planned map[string][]generator.Artifact) ([]string, error) {
	known := map[string]bool{}
	out := make([]string, 0, len(directories)+len(planned))
	add := func(directory string) error {
		absolute, err := filepath.Abs(directory)
		if err != nil {
			return err
		}
		if !known[absolute] {
			known[absolute] = true
			out = append(out, absolute)
		}
		return nil
	}
	for _, directory := range directories {
		if err := add(directory); err != nil {
			return nil, err
		}
	}
	for directory := range planned {
		if err := add(directory); err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

// pageArtifact rewrites one generated file as an artifact of the directory it
// belongs to. The output name is what carries the identity: a component and a
// binder that derive the same base share one file, which is the same rule every
// other generated source follows.
func pageArtifact(file routetree.Generated) (generator.Artifact, error) {
	base := strings.TrimSuffix(filepath.Base(file.Path), "_pw_gen.go")
	if base == filepath.Base(file.Path) {
		return generator.Artifact{}, fmt.Errorf("popcornweb: page tree emitted %q, which does not end in _pw_gen.go", file.Path)
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file.Path, file.Source, parser.PackageClauseOnly)
	if err != nil {
		return generator.Artifact{}, fmt.Errorf("popcornweb: parse generated %s: %w", file.Path, err)
	}
	return generator.Artifact{
		Kind:        generator.ArtifactHTMLTemplate,
		SourcePath:  file.Path,
		OutputBase:  base,
		PackageName: parsed.Name.Name,
		Content:     file.Source,
	}, nil
}

// moduleImportPath reads the module path a project's generated imports hang
// below, and the directory it is declared in. A page tree cannot derive either:
// a directory does not reveal where it sits inside its module.
//
// The search walks up, because the go.mod of a project is normally beside its
// popcornweb.toml but nothing requires it to be.
func moduleImportPath(root string) (string, string, error) {
	directory, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	for {
		source, err := os.ReadFile(filepath.Join(directory, "go.mod"))
		if err == nil {
			path := modfile.ModulePath(source)
			if path == "" {
				return "", "", fmt.Errorf("%s does not declare a module path", filepath.Join(directory, "go.mod"))
			}
			return path, directory, nil
		}
		if !os.IsNotExist(err) {
			return "", "", err
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", "", fmt.Errorf("no go.mod above %s", root)
		}
		directory = parent
	}
}

// treeImportPath is the Go import path of one page tree root, which the
// generated registry needs to import the packages below it.
func treeImportPath(module, moduleDir, treeRoot string) (string, error) {
	absolute, err := filepath.Abs(treeRoot)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(moduleDir, absolute)
	if err != nil {
		return "", err
	}
	relative = filepath.ToSlash(relative)
	if relative == "." {
		return module, nil
	}
	if strings.HasPrefix(relative, "../") {
		return "", fmt.Errorf("page tree root %s is outside the module at %s", treeRoot, moduleDir)
	}
	return module + "/" + relative, nil
}

// assetArtifactKind classifies one extracted asset the way the flat path does,
// so a stylesheet and a script reach the same purpose filter and the same
// write. The two paths produce the same htmlbind.Asset, and this is the one
// place the tree has to say which kind it is holding.
func assetArtifactKind(asset templatehtmlbind.Asset) generator.ArtifactKind {
	if asset.Kind == templatehtmlbind.AssetScript {
		return generator.ArtifactScript
	}
	return generator.ArtifactStylesheet
}

// The generated registry declares the two tables that have to be read together,
// and nothing reads them together on its own.
//
// Routes says which directory a pattern serves. Actions says which directory a
// server function was declared in. Joining them is what lets a component script
// on /users/{id} name Rename and reach it, with no element to read an address
// off and no way to compute one — the address holds a digest of the declaring
// directory.
var (
	// Params is what tells a Routes entry from an Actions one: both carry a
	// pattern, a path and a directory, and only a route carries the parameters
	// its path declares. Matching without it registered every action endpoint as
	// though it rendered a document.
	pageRouteEntry = regexp.MustCompile(`\{Pattern: "([^"]+)", Path: "[^"]*", Dir: "([^"]*)", Params:`)
	// Published rather than Handler: the identifier a script writes is the
	// wire name, which is the Go name in lowerCamelCase unless a declaration
	// overrode it. Reading the Go name here would publish Rename where every
	// caller writes rename.
	pageActionEntry = regexp.MustCompile(`\{Pattern: "[^"]*", Path: "([^"]+)", Dir: "([^"]*)", Handler: "[^"]+", Hash: "[^"]*", Published: "([^"]+)"`)
)

// pageActionRegistrationArtifact emits the init that publishes each route's own
// server functions.
//
// Without it a project would generate both tables and join them nowhere, so a
// script naming an action would find nothing on the page that says where it
// lives. The registration is derived rather than written by an author for the
// reason the reloadable one is: two lists that must agree should not be two
// things to remember.
func pageActionRegistrationArtifact(artifact generator.Artifact) (generator.Artifact, bool) {
	if artifact.OutputBase+"_pw_gen.go" != pwgen.PageRegistryOutput {
		return generator.Artifact{}, false
	}
	content := string(artifact.Content)
	byDirectory := map[string][][2]string{}
	for _, match := range pageActionEntry.FindAllStringSubmatch(content, -1) {
		byDirectory[match[2]] = append(byDirectory[match[2]], [2]string{match[3], match[1]})
	}
	if len(byDirectory) == 0 {
		return generator.Artifact{}, false
	}
	var body strings.Builder
	// The shared leaf rather than either runtime, for the reason the document
	// and reloadable registrations name it: one registry, read by both.
	body.WriteString("package " + artifact.PackageName + "\n\nimport \"github.com/shibukawa/popcornweb/pwruntime\"\n\nfunc init() {\n")
	registered := 0
	for _, route := range pageRouteEntry.FindAllStringSubmatch(content, -1) {
		actions := byDirectory[route[2]]
		if len(actions) == 0 {
			continue
		}
		fmt.Fprintf(&body, "\tpwruntime.RegisterPageActions(%q,\n", route[1])
		for _, action := range actions {
			fmt.Fprintf(&body, "\t\tpwruntime.PageAction{Name: %q, Path: %q},\n", action[0], action[1])
		}
		body.WriteString("\t)\n")
		registered++
	}
	body.WriteString("}\n")
	if registered == 0 {
		// Every action belongs to a directory no route serves, which a layout
		// exporting a handler produces. Nothing can call one by name from a page
		// it does not serve, so the file would register nothing.
		return generator.Artifact{}, false
	}
	return generator.Artifact{
		Kind:        artifact.Kind,
		SourcePath:  artifact.SourcePath,
		OutputBase:  artifact.OutputBase,
		PackageName: artifact.PackageName,
		Content:     []byte(body.String()),
	}, true
}
