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

	"github.com/shibukawa/popcornwave/internal/pwenv"
)

func runInit(args []string, stdout io.Writer) error {
	tailwind := false
	var positional []string
	for _, arg := range args {
		switch arg {
		case "--tailwind":
			tailwind = true
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("init: unknown option %q", arg)
			}
			positional = append(positional, arg)
		}
	}
	if len(positional) != 1 {
		return fmt.Errorf("usage: pw init <project-name> [--tailwind]")
	}
	name := strings.TrimSpace(positional[0])
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
	files := scaffoldFilesWithTailwind(name, tailwind)
	for path, content := range files {
		target := filepath.Join(destination, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := writeScaffoldFile(target, []byte(content)); err != nil {
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

func writeScaffoldFile(target string, content []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, target)
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
	return scaffoldFilesWithTailwind(name, false)
}

func scaffoldFilesWithTailwind(name string, tailwind bool) map[string]string {
	moduleExtra := frameworkModuleDirective()
	configTailwind := ""
	devboxTailwind := ""
	homeStylesheet := ""
	homeClasses := ""
	if tailwind {
		configTailwind = `
[assets.tailwind]
enabled = true
input = "` + defaultTailwindInput + `"
output = "` + defaultTailwindOutput + `"
minify = true
`
		devboxTailwind = `, "tailwindcss_4@4.1.18"`
		homeStylesheet = `<link rel="stylesheet" href="/public/generated/app.css">`
		homeClasses = ` class="mx-auto max-w-3xl p-8 text-slate-900"`
	}
	files := map[string]string{
		"go.mod": "module " + name + "\n\ngo 1.26.0\n\n" + moduleExtra,
		"popcornwave.toml": `[project]
name = "` + name + `"
main = "./cmd/` + name + `"

[dev]
extra_watch = []
` + configTailwind,
		pwenv.FileName(pwenv.Development): `# Development runtime configuration.
# APP_ENV selects this file; add config.stg.toml and config.prod.toml as needed.
[server]
port = 8080

[observability]
minimum_level = "debug"
service_name = "` + name + `"
`,
		"devbox.json": `{
  "$schema": "https://raw.githubusercontent.com/jetify-com/devbox/0.14.2/.schema/devbox.schema.json",
  "packages": ["go@latest", "valkey@latest"` + devboxTailwind + `],
  "shell": {"init_hook": ["echo 'Popcorn Wave development environment'"]}
}
`,
		"devbox.lock": "{}\n",
		"cmd/" + name + "/main.go": `package main

import (
	"context"
	"log"

	"` + name + `/handlers"
	"github.com/shibukawa/popcornwave/pw"
)

func main() {
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
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
	pw.WriteHTML(w, r, Home(HomeParams{Name: input.Name}))
}
`,
		"handlers/home.pw.html": `package handlers

export component Home(name: string): html {
<h1 class="text-3xl font-bold">Hello, {name}</h1>
}
`,
		"templates/document.pw.html": `package templates

export component Document(children: html?): html {
<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Popcorn Wave</title>` + homeStylesheet + `</head>
<body` + homeClasses + `><slot /></body></html>
}
`,
		"templates/templates.go": "package templates\n",
		"queries/users.pw.sql": `package queries

type User {
  id: int
  name: string
}

export statement FindUser(id: int): sql.one<User> {
SELECT id, name FROM users WHERE id = {id}
}
`,
		"migrations/00001_init.sql": `-- +goose Up
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE users;
`,
		"templates/400.pw.html": errorTemplate("templates", "Error400", "Bad Request"),
		"templates/404.pw.html": errorTemplate("templates", "Error404", "Not Found"),
		"templates/500.pw.html": errorTemplate("templates", "Error500", "Internal Server Error"),
		"public.go": `package publicassets

import (
	"embed"
	"io/fs"

	"github.com/shibukawa/popcornwave/middlewares"
)

//go:embed all:public
var embeddedPublic embed.FS

func init() {
	middlewares.RegisterPublicFS(PublicFS())
}

func PublicFS() fs.FS {
	result, err := fs.Sub(embeddedPublic, "public")
	if err != nil {
		panic(err)
	}
	return result
}
`,
		"public/.keep": "",
		".vscode/settings.json": `{
    "files.exclude": {
        "**/*_pw_gen.go": true
    }
}
`,
		// The binary pattern is anchored: a bare name would also ignore cmd/<name>/.
		".gitignore": ".devbox/\n/" + name + "\n*_pw_gen.go\npublic/**/*.zstd\n*.db\n",
	}
	if tailwind {
		files["assets/app.css"] = `@import "tailwindcss";
@source "../handlers";
@source "../templates";
`
		files["public/generated/app.css"] = "/* Generated by Tailwind CSS. */\n"
	}
	return files
}

func errorTemplate(pkg, component, title string) string {
	return "package " + pkg + "\n\nexport component " + component + "(): html {\n" +
		"<h1>" + title + "</h1>\n}\n"
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
