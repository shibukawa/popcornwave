package pwcli

import (
	"fmt"
	"go/parser"
	"go/token"
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
		// GenerateTree rather than Generate: the latter is documented as the
		// variant that discards the extracted assets, and a page declaring a
		// script block through it produced a reference to a file that answered
		// 404. The URL base travels too, or every tree asset would be compiled
		// against the module default regardless of where this writes them.
		result, err := routetree.GenerateTree(routetree.GenerateOptions{
			Config:          pwgen.PageConfig(treeRoot, importBase),
			Emitter:         emitter,
			ComponentSuffix: pwgen.PageComponentSuffix,
			DecoderOutput:   pwgen.PageDecoderOutput,
			RegistryOutput:  pwgen.PageRegistryOutput,
			PublicURLBase:   generator.DefaultPublicURLBase,
			// Threaded rather than left to the default. Both paths happen to use
			// the module default today, which is why one document has held one
			// spelling; passing it is what keeps that true if this framework ever
			// brands the prefix, instead of a page tree quietly taking the
			// default while a registered-router template took the brand.
			DataAttributePrefix: pwgen.AttributePrefix(),
		})
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
