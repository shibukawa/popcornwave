package pwcli

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shibukawa/popcornwave/internal/pwgen"
	"github.com/shibukawa/tinybind-go/generator"
	"github.com/shibukawa/tinybind-go/routetree"
	templatehtmlbind "github.com/shibukawa/tinybind-go/templates/htmlbind"
	"golang.org/x/mod/modfile"
)

// reservedPageTemplates are the template names a page tree gives a meaning.
// Anything else ending in .pw.html inside a root is compiled by nothing, which
// is worth saying rather than leaving to be noticed.
var reservedPageTemplates = map[string]bool{
	pwgen.PageFile:     true,
	pwgen.LayoutFile:   true,
	pwgen.DocumentFile: true,
}

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
func planPageTrees(root string, config projectConfig) (map[string][]generator.Artifact, error) {
	if len(config.Generate.Pages) == 0 {
		return nil, nil
	}
	module, moduleDir, err := moduleImportPath(root)
	if err != nil {
		return nil, err
	}
	emitter, err := pwgen.PageEmitter()
	if err != nil {
		return nil, err
	}
	planned := map[string][]generator.Artifact{}
	for _, relative := range config.Generate.Pages {
		treeRoot := filepath.Join(root, filepath.FromSlash(relative))
		importBase, err := treeImportPath(module, moduleDir, treeRoot)
		if err != nil {
			return nil, err
		}
		result, err := generatePageTree(treeRoot, importBase, emitter)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", relative, err)
		}
		treeRootAbs, err := filepath.Abs(treeRoot)
		if err != nil {
			return nil, err
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
				return nil, err
			}
			directory, err := filepath.Abs(filepath.Dir(file.Path))
			if err != nil {
				return nil, err
			}
			planned[directory] = append(planned[directory], artifact)
		}
	}
	return planned, nil
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
func generatePageTree(treeRoot, importBase string, emitter *routetree.Emitter) (routetree.Result, error) {
	return routetree.GenerateTree(routetree.GenerateOptions{
		Config:              pwgen.PageConfig(treeRoot, importBase),
		Emitter:             emitter,
		ComponentSuffix:     pwgen.PageComponentSuffix,
		DecoderOutput:       pwgen.PageDecoderOutput,
		RegistryOutput:      pwgen.PageRegistryOutput,
		PublicURLBase:       generator.DefaultPublicURLBase,
		DataAttributePrefix: pwgen.AttributePrefix(),
	})
}

// planSecondBuildPages is the second transport's page tree step of one run.
//
// It exists to hold the three cases apart, because they are decided by facts
// this step does not otherwise see: whether the project declares the second
// build, and whether the run is allowed to write.
func planSecondBuildPages(root string, config projectConfig, check bool, changes []fileChange) ([]fileChange, error) {
	if !config.FastHTTP {
		// Sweeping runs whether or not the declaration holds. Turning it off has
		// to take these files with it, or the tree keeps a half that compiles
		// under a tag nothing writes for any more.
		return planFastPageTrees(root, config, false, changes)
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
		return planFastPageTrees(root, config, true, changes)
	}
	if len(changes) > 0 {
		// This step reads the derived handlers the stages above produced, so what
		// they planned is written before it looks.
		if err := applyFileChanges(changes); err != nil {
			return nil, err
		}
	}
	return planFastPageTrees(root, config, true, changes)
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
func planFastPageTrees(root string, config projectConfig, derive bool, changes []fileChange) ([]fileChange, error) {
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
			derived, err := deriveTreeFor(treeRoot, importBase)
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
func deriveTreeFor(treeRoot, importBase string) ([]routetree.Generated, error) {
	netHTTP, err := pwgen.PageEmitter()
	if err != nil {
		return nil, err
	}
	fastHTTP, err := pwgen.FastPageEmitter()
	if err != nil {
		return nil, err
	}
	shared, err := generatePageTree(treeRoot, importBase, netHTTP)
	if err != nil {
		return nil, err
	}
	files, err := generatePageTree(treeRoot, importBase, fastHTTP)
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
		return generator.Artifact{}, fmt.Errorf("popcornwave: page tree emitted %q, which does not end in _pw_gen.go", file.Path)
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file.Path, file.Source, parser.PackageClauseOnly)
	if err != nil {
		return generator.Artifact{}, fmt.Errorf("popcornwave: parse generated %s: %w", file.Path, err)
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
// popcornwave.toml but nothing requires it to be.
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
