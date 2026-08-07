package pwcli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// isModulePath distinguishes a pw add argument that names a module from one that
// names a built-in capability. A module path carries a dot in its first element,
// which is the same rule the Go tool uses to tell an import path from a standard
// library one, and no capability name looks like that.
func isModulePath(arg string) bool {
	first, _, _ := strings.Cut(arg, "/")
	return strings.Contains(first, ".")
}

// addPackage installs a component package: the go.mod requirement and one entry
// in the packages array, and nothing else.
//
// There is no wizard and no review screen, because nothing is copied. The
// declaration is the install: pw generate emits the blank import that links the
// package, and pw migrate applies its stream. What used to be a plan of files to
// approve is now two lines the command can simply write.
func addPackage(ctx context.Context, root, module string, stdout io.Writer) error {
	config, err := loadProjectConfig(root)
	if err != nil {
		return err
	}
	if config.Kind != kindApplication {
		return fmt.Errorf("add: a package depends on another through go.mod; the packages array applies to an application")
	}
	for _, existing := range config.Packages {
		if existing.Module == module {
			return fmt.Errorf("add: this project already declares %s", module)
		}
	}
	// go get first, because the manifest read below needs the module on disk and
	// because a module that cannot be fetched should leave the project untouched.
	get := exec.CommandContext(ctx, "go", "get", module)
	get.Dir = root
	get.Stdout, get.Stderr = stdout, stdout
	if err := get.Run(); err != nil {
		return fmt.Errorf("add: go get %s: %w", module, err)
	}
	dir, err := moduleDir(ctx, root, module)
	if err != nil {
		return fmt.Errorf("add %s: %w", module, err)
	}
	manifest, err := readPackageManifest(dir)
	if err != nil {
		return fmt.Errorf("add %s: %w", module, err)
	}
	if manifest.Module != module {
		return fmt.Errorf("add %s: its manifest declares package.module = %q", module, manifest.Module)
	}
	resolved := []resolvedPackage{{Module: module, Dir: dir, Manifest: manifest}}
	existing, err := resolvePackages(ctx, root, config.Packages)
	if err != nil {
		return err
	}
	if err := checkPackageCompatibility(config, append(existing, resolved...), nil); err != nil {
		return fmt.Errorf("add: %w", err)
	}
	if err := appendPackageDeclaration(root, module); err != nil {
		return err
	}
	reportPackageInstall(manifest, stdout)
	return nil
}

// appendPackageDeclaration writes the entry at the end of popcornwave.toml. It
// appends rather than rewriting the document, so operator comments and hand
// tuned values survive the edit, which is the same rule every other pw edit of
// this file follows.
func appendPackageDeclaration(root, module string) error {
	path := filepath.Join(root, "popcornwave.toml")
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	entry := "\n[[packages]]\nmodule = " + strconv.Quote(module) + "\n"
	if len(current) > 0 && current[len(current)-1] != '\n' {
		entry = "\n" + entry
	}
	return os.WriteFile(path, append(current, entry...), 0o644)
}

// reportPackageInstall prints what the declaration did and what it deliberately
// did not do. The Register call is the one step a declaration cannot replace,
// because mounting is the contribution an application has an opinion about.
func reportPackageInstall(manifest packageManifest, stdout io.Writer) {
	fmt.Fprintf(stdout, "declared %s\n", manifest.Module)
	if manifest.Summary != "" {
		fmt.Fprintf(stdout, "  %s\n", manifest.Summary)
	}
	fmt.Fprintln(stdout, "next:")
	fmt.Fprintln(stdout, "  pw generate\t# links the package through the generated bootstrap")
	if manifest.Migrations.Dir != "" {
		fmt.Fprintln(stdout, "  pw migrate up\t# applies its migrations before the application's")
	}
	if manifest.Register != "" {
		importPath := manifest.Module
		if manifest.Import != "" {
			importPath = manifest.Import
		}
		fmt.Fprintf(stdout, "  mount it in your entry point: %s.%s(mux)\n", filepath.Base(importPath), manifest.Register)
	}
	if manifest.ConfigSection != "" {
		fmt.Fprintf(stdout, "  its defaults apply as they are; override them under [%s] when you want to\n", manifest.ConfigSection)
	}
}
