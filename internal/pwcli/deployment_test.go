package pwcli

import (
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFunctionEntrypointTransformsEitherBackendMain(t *testing.T) {
	root := filepath.Join("testdata", "serverlessapp")
	for _, backend := range []string{backendNetHTTP, backendFastHTTP} {
		destination := t.TempDir()
		if err := copyTransformedMain(root, destination, "handler", "./cmd/serverlessapp", backend); err != nil {
			t.Fatalf("%s: %v", backend, err)
		}
		name := "main.go"
		if backend == backendFastHTTP {
			name = "main_fasthttp.go"
		}
		source, err := os.ReadFile(filepath.Join(destination, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, expected := range []string{"package handler", "func initializeApplication()", "captureApplication("} {
			if !strings.Contains(string(source), expected) {
				t.Errorf("%s transformed main misses %q:\n%s", backend, expected, source)
			}
		}
		if strings.Contains(string(source), "func main()") {
			t.Errorf("%s transformed main still declares main", backend)
		}
	}
}

func TestFunctionEntrypointRecognizesAnAliasedRuntimeImport(t *testing.T) {
	root := t.TempDir()
	writeNestedTestFile(t, filepath.Join(root, "cmd", "app", "main.go"), `package main

import (
	"context"
	"net/http"
	framework "github.com/shibukawa/popcornweb/pw"
)

func main() { _ = framework.Run(context.Background(), http.NewServeMux()) }
`)
	destination := t.TempDir()
	if err := copyTransformedMain(root, destination, "handler", "./cmd/app", backendNetHTTP); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(destination, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "captureApplication(framework.Run") {
		t.Errorf("aliased Run was not captured:\n%s", source)
	}
}

func TestSourceFunctionWrappersAreProviderAndBackendSpecific(t *testing.T) {
	tests := []struct {
		target, backend string
		want            []string
		reject          []string
	}{
		{targetGoogleCloudRunFunctions, backendNetHTTP, []string{`functions.HTTP("PopcornWeb", Handler)`, "pw.Middlewares"}, []string{"pwfast.Start"}},
		{targetGoogleCloudRunFunctions, backendFastHTTP, []string{`functions.HTTP("PopcornWeb", Handler)`, "pwfast.Start", "pwfast.NetHTTPHandler"}, []string{"pw.Middlewares"}},
		{targetVercelGo, backendNetHTTP, []string{"func Handler(w http.ResponseWriter", "pw.Middlewares"}, []string{"functions.HTTP", "pwfast.Start"}},
		{targetVercelGo, backendFastHTTP, []string{"func Handler(w http.ResponseWriter", "pwfast.NetHTTPHandler"}, []string{"functions.HTTP", "pw.Middlewares"}},
	}
	for _, test := range tests {
		source := sourceFunctionWrapper("handler", test.target, test.backend)
		if _, err := format.Source([]byte(source)); err != nil {
			t.Fatalf("%s/%s is invalid Go: %v\n%s", test.target, test.backend, err, source)
		}
		for _, expected := range test.want {
			if !strings.Contains(source, expected) {
				t.Errorf("%s/%s misses %q", test.target, test.backend, expected)
			}
		}
		for _, rejected := range test.reject {
			if strings.Contains(source, rejected) {
				t.Errorf("%s/%s contains %q", test.target, test.backend, rejected)
			}
		}
	}
}

func TestFunctionSourceCopyExcludesLocalAndDeploymentState(t *testing.T) {
	root := t.TempDir()
	for name, value := range map[string]string{
		"go.mod":           "module example.test/app\n\ngo 1.26.0\n",
		"config.prod.toml": "[server]\nport=8080\n",
		"config.dev.toml":  "secret=true\n",
		".env":             "SECRET=value\n",
		"keep.txt":         "keep\n",
	} {
		writeTestFile(t, filepath.Join(root, name), value)
	}
	writeNestedTestFile(t, filepath.Join(root, ".pw", "build", "secret.txt"), "no")
	writeNestedTestFile(t, filepath.Join(root, "cmd", "app", "main.go"), "package main")
	destination := t.TempDir()
	if err := copyProjectForFunction(root, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, "keep.txt")); err != nil {
		t.Errorf("ordinary source was not copied: %v", err)
	}
	for _, excluded := range []string{".env", "config.dev.toml", ".pw", "cmd"} {
		if _, err := os.Stat(filepath.Join(destination, excluded)); !os.IsNotExist(err) {
			t.Errorf("local-only %s entered the source bundle", excluded)
		}
	}
}

// The external tree is placed at the stage root rather than in the application
// subtree, because the server resolves it against the working directory the
// function runs with. Copying it into the subtree as well would put the largest
// files in the project into the artifact twice, in a place nothing reads.
func TestFunctionSourceCopyLeavesTheExternalTreeToTheStageRoot(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.test/app\n\ngo 1.26.0\n")
	writeNestedTestFile(t, filepath.Join(root, externalPublicDir, "clip.mp4"), "payload")
	destination := t.TempDir()
	if err := copyProjectForFunction(root, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, externalPublicDir)); !os.IsNotExist(err) {
		t.Errorf("%s entered the application subtree", externalPublicDir)
	}
}

// Every deployment target resolves the tree against its working directory, so
// the stage root is where it has to land — and it has to exist even for a
// project that has never put anything in it, or a container COPY naming it
// fails the image build.
func TestDeploymentStageCarriesTheExternalTree(t *testing.T) {
	root := t.TempDir()
	writeNestedTestFile(t, filepath.Join(root, externalPublicDir, "media", "clip.mp4"), "payload")
	stage := t.TempDir()
	if err := copyExternalAssets(root, stage); err != nil {
		t.Fatal(err)
	}
	carried, err := os.ReadFile(filepath.Join(stage, externalPublicDir, "media", "clip.mp4"))
	if err != nil {
		t.Fatalf("the external tree did not reach the stage: %v", err)
	}
	if string(carried) != "payload" {
		t.Errorf("content = %q, want the source bytes", carried)
	}

	empty := t.TempDir()
	if err := copyExternalAssets(t.TempDir(), empty); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(empty, externalPublicDir)); err != nil || !info.IsDir() {
		t.Errorf("a project without the tree produced no directory for the artifact to copy: %v", err)
	}
}
