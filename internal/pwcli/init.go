package pwcli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
)

func runInit(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: pw init <project-name>")
	}
	name := strings.TrimSpace(args[0])
	if !validProjectName(name) {
		return fmt.Errorf("invalid project name %q", name)
	}
	destination, err := filepath.Abs(name)
	if err != nil {
		return err
	}
	if entries, readErr := os.ReadDir(destination); readErr == nil && len(entries) > 0 {
		return fmt.Errorf("destination %s is not empty", destination)
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	files := scaffoldFiles(name)
	for path, content := range files {
		target := filepath.Join(destination, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return err
		}
	}
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = destination
	tidy.Stdout = stdout
	tidy.Stderr = stdout
	tidy.Env = os.Environ()
	if err := tidy.Run(); err != nil {
		return fmt.Errorf("initialize Go module: %w", err)
	}
	previous, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(destination); err != nil {
		return err
	}
	generateErr := runGenerate(context.Background(), nil, stdout)
	restoreErr := os.Chdir(previous)
	if generateErr != nil {
		return fmt.Errorf("generate starter: %w", generateErr)
	}
	if restoreErr != nil {
		return restoreErr
	}
	fmt.Fprintf(stdout, "\nCreated %s\n\n  cd %s\n  devbox shell\n  pw dev\n", name, name)
	return nil
}

func validProjectName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		if !(r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func scaffoldFiles(name string) map[string]string {
	moduleExtra := frameworkModuleDirective()
	return map[string]string{
		"go.mod": "module " + name + "\n\ngo 1.26.0\n\n" + moduleExtra,
		"popcornwave.toml": `[project]
name = "` + name + `"
main = "./cmd/` + name + `"

[generate]
html = ["handlers/**/*.pw.html", "templates/**/*.pw.html"]
sql = ["queries/**/*.pw.sql"]

[dev]
watch = ["**/*.go", "**/*.pw.html", "**/*.pw.sql", "popcornwave.toml"]
`,
		"devbox.json": `{
  "$schema": "https://raw.githubusercontent.com/jetify-com/devbox/0.14.2/.schema/devbox.schema.json",
  "packages": ["go@latest", "valkey@latest"],
  "shell": {"init_hook": ["echo 'Popcorn Wave development environment'"]}
}
`,
		"devbox.lock": "{}\n",
		"cmd/" + name + "/main.go": `package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"` + name + `/handlers"
	"github.com/shibukawa/popcornwave/pw"
)

type GenerateConfigCommand struct {
	Format string ` + "`arg:\"required\" help:\"output format: toml or env\"`" + `
}

func main() {
	pw.SubCommand[GenerateConfigCommand]("generate-config", "write merged configuration scaffolds")
	if err := pw.ParseConfig(); err != nil {
		log.Fatal(err)
	}
	if command, ok := pw.Command[GenerateConfigCommand](); ok {
		if err := generateConfig(command.Format); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}

func generateConfig(format string) error {
	switch format {
	case "toml":
		return pw.WriteScaffoldTOML(os.Stdout)
	case "env":
		return pw.WriteScaffoldEnv(os.Stdout)
	default:
		return fmt.Errorf("unknown config format %q (want toml or env)", format)
	}
}
`,
		"handlers/index.go": `package handlers

import "github.com/shibukawa/popcornwave/pw"

var mux = pw.NewServeMux()

func Handlers() *pw.ServeMux { return mux }
`,
		"handlers/home_handler.go": `package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
)

type homeInput struct {
	Name string ` + "`query:\"name\" default:\"World\"`" + `
}

func init() { mux.HandleFunc("GET /", home) }

func home(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[homeInput](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	pw.WriteHTML(w, r, Home, HomeParams{Name: input.Name})
}
`,
		"handlers/home.pw.html": `package handlers

export component Home(name: string): html {
<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Popcorn Wave</title></head>
<body><h1>Hello, {name}</h1></body></html>
}
`,
		"queries/users.pw.sql": `package queries

type User {
  id: int
  name: string
}

export statement FindUser(id: int): sql.one<User> {
SELECT id, name FROM users WHERE id = {id}
}
`,
		"templates/400.pw.html": errorTemplate("templates", "Error400", "Bad Request"),
		"templates/404.pw.html": errorTemplate("templates", "Error404", "Not Found"),
		"templates/500.pw.html": errorTemplate("templates", "Error500", "Internal Server Error"),
		".gitignore":            ".devbox/\n" + name + "\n",
	}
}

func errorTemplate(pkg, component, title string) string {
	return "package " + pkg + "\n\nexport component " + component + "(): html {\n" +
		"<!doctype html><html lang=\"en\"><body><h1>" + title + "</h1></body></html>\n}\n"
}

func frameworkModuleDirective() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Path == "github.com/shibukawa/popcornwave" &&
		info.Main.Version != "" && info.Main.Version != "(devel)" && !strings.Contains(info.Main.Version, "+dirty") {
		return "require github.com/shibukawa/popcornwave " + info.Main.Version + "\n"
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "require github.com/shibukawa/popcornwave latest\n"
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	return "require github.com/shibukawa/popcornwave v0.0.0\n\nreplace github.com/shibukawa/popcornwave => " + filepath.ToSlash(root) + "\n"
}
