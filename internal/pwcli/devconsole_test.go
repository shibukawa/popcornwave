package pwcli

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

const consoleProject = otelProject + `
[dev.console]
port = 0
`

func startTestConsole(t *testing.T, files map[string]string) (string, *bytes.Buffer) {
	t.Helper()
	if _, ok := files["popcornwave.toml"]; !ok {
		files["popcornwave.toml"] = consoleProject
	}
	root := writeProject(t, files)
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	telemetry, err := startDevTelemetryViewer(config, stdout)
	if err != nil {
		t.Fatalf("viewer: %v", err)
	}
	t.Cleanup(telemetry.close)
	console := startDevConsole(root, config, telemetry, stdout, stderr)
	if console == nil {
		t.Fatalf("the console did not start:\n%s", stderr)
	}
	t.Cleanup(console.Close)
	return console.URL(), stdout
}

func body(t *testing.T, url string) string {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer response.Body.Close()
	content, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read %s: %v", url, err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d for %s", response.StatusCode, url)
	}
	return string(content)
}

func TestDevConsolePrintsOneURLAndServesTheIndex(t *testing.T) {
	url, stdout := startTestConsole(t, map[string]string{})
	if !strings.Contains(stdout.String(), url) {
		t.Fatalf("the loop never printed the console URL:\n%s", stdout)
	}
	page := body(t, url+"/")
	for _, want := range []string{"app", "telemetry", "assets"} {
		if !strings.Contains(page, want) {
			t.Errorf("the index never mentioned %q:\n%s", want, page)
		}
	}
}

// The viewer UI moves onto the console listener while the receiver keeps its
// own, so both addresses answer and they are not the same one.
func TestTelemetryPaneIsServedByTheConsoleWhileTheReceiverKeepsItsPort(t *testing.T) {
	root := writeProject(t, map[string]string{"popcornwave.toml": consoleProject})
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	telemetry, err := startDevTelemetryViewer(config, stdout)
	if err != nil {
		t.Fatalf("viewer: %v", err)
	}
	t.Cleanup(telemetry.close)
	console := startDevConsole(root, config, telemetry, stdout, stderr)
	if console == nil {
		t.Fatalf("the console did not start:\n%s", stderr)
	}
	t.Cleanup(console.Close)

	if console.URL() == telemetry.url() {
		t.Fatal("the console and the receiver took the same address")
	}
	if page := body(t, console.URL()+"/telemetry/"); !strings.Contains(page, "<div id=\"root\">") {
		t.Errorf("the console did not serve the viewer UI:\n%s", page)
	}
	// The API follows the page under the prefix, because the UI resolves it
	// against the served document rather than against the origin.
	if snapshot := body(t, console.URL()+"/telemetry/api/snapshot"); !strings.Contains(snapshot, "traces") {
		t.Errorf("the snapshot API did not follow the mount: %s", snapshot)
	}
	// The receiver still answers on its own port, which is what the
	// application exports to.
	if snapshot := body(t, telemetry.url()+"/api/snapshot"); !strings.Contains(snapshot, "traces") {
		t.Errorf("the receiver stopped answering on its own port: %s", snapshot)
	}
}

func TestDevConsoleReportsADisabledPaneWithItsKey(t *testing.T) {
	url, _ := startTestConsole(t, map[string]string{
		"popcornwave.toml": consoleProject + "\n[dev.console.assets]\nenabled = false\n",
	})
	page := body(t, url+"/")
	if !strings.Contains(page, "dev.console.assets.enabled") {
		t.Errorf("a disabled pane was hidden rather than explained:\n%s", page)
	}
}

func TestDisabledConsoleStartsNoListener(t *testing.T) {
	root := writeProject(t, map[string]string{
		"popcornwave.toml": otelProject + "\n[dev.console]\nenabled = false\n",
	})
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if console := startDevConsole(root, config, nil, stdout, stderr); console != nil {
		console.Close()
		t.Fatal("a disabled console still took a port")
	}
	if stdout.Len() != 0 {
		t.Errorf("a disabled console still reported an address:\n%s", stdout)
	}
}

// The port is fixed, so a collision is a real conflict. It is reported and the
// loop goes on, because an unobservable run is still a working one.
func TestConsolePortCollisionIsReportedAndNotFatal(t *testing.T) {
	first, _ := startTestConsole(t, map[string]string{})
	port := first[strings.LastIndex(first, ":")+1:]

	root := writeProject(t, map[string]string{
		"popcornwave.toml": otelProject + "\n[dev.console]\nport = " + port + "\n",
	})
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if console := startDevConsole(root, config, nil, stdout, stderr); console != nil {
		console.Close()
		t.Fatal("two consoles bound the same port")
	}
	if !strings.Contains(stderr.String(), port) {
		t.Errorf("the collision did not name the address:\n%s", stderr)
	}
}

func TestAssetPaneReadsTheProjectTree(t *testing.T) {
	url, _ := startTestConsole(t, map[string]string{
		"public/app.css":  "a{}",
		"public/logo.png": "binary",
	})
	page := body(t, url+"/assets/")
	if !strings.Contains(page, "app.css") || !strings.Contains(page, "logo.png") {
		t.Errorf("the asset pane did not list the public tree:\n%s", page)
	}
}

// The pane and pw build share one eligibility test, so what the pane promises
// is what a release build actually does.
func TestAssetPaneUsesTheBuildEligibilityTest(t *testing.T) {
	url, _ := startTestConsole(t, map[string]string{
		"public/app.css":  "a{}",
		"public/logo.png": "binary",
	})
	page := body(t, url+"/assets/")
	css := strings.Index(page, "app.css")
	png := strings.Index(page, "logo.png")
	if css < 0 || png < 0 {
		t.Fatalf("both files should be listed:\n%s", page)
	}
	if !strings.Contains(page[css:png], "on build") {
		t.Errorf("the stylesheet was not shown as compressible:\n%s", page[css:png])
	}
	if !strings.Contains(page[png:], "not compressible") {
		t.Errorf("the image was not shown as incompressible:\n%s", page[png:])
	}
}

func TestDevelopmentServerComesFromTheDevelopmentConfig(t *testing.T) {
	root := writeProject(t, map[string]string{
		"popcornwave.toml": consoleProject,
		"config.dev.toml": "[server]\nport = 9123\napi_doc = \"scalar\"\napi_doc_path = \"/reference\"\n" +
			"[server.public]\nmount = \"/static\"\n",
	})
	server := readDevelopmentServer(root)
	if server.URL != "http://localhost:9123" {
		t.Errorf("URL = %q, want the configured port", server.URL)
	}
	if server.PublicMount != "/static" {
		t.Errorf("PublicMount = %q, want the configured mount", server.PublicMount)
	}
	// The path is configuration, so a console that hardcoded /docs would send
	// the developer to a 404 in every project that moved it.
	if url := server.APIDocURL(); url != "http://localhost:9123/reference" {
		t.Errorf("APIDocURL = %q, want the moved path", url)
	}
}

// An absent path means the framework default applies, and the console says
// which path that is rather than leaving the link out.
func TestAPIDocFallsBackToTheFrameworkDefaultPath(t *testing.T) {
	root := writeProject(t, map[string]string{
		"popcornwave.toml": consoleProject,
		"config.dev.toml":  "[server]\nport = 8080\napi_doc = \"scalar\"\n",
	})
	if url := readDevelopmentServer(root).APIDocURL(); url != "http://localhost:8080/docs" {
		t.Errorf("APIDocURL = %q, want the default path", url)
	}
}

// An application serving no documentation UI gets no link, and the index says
// which key would turn it on.
func TestAPIDocIsEmptyWhenTheEndpointIsOff(t *testing.T) {
	root := writeProject(t, map[string]string{
		"popcornwave.toml": consoleProject,
		"config.dev.toml":  "[server]\nport = 8080\n",
	})
	if url := readDevelopmentServer(root).APIDocURL(); url != "" {
		t.Errorf("APIDocURL = %q, want none", url)
	}
}

// An address pw could not read is left empty, so the index says undetermined
// rather than printing a default that may be wrong.
func TestDevelopmentServerIsEmptyWhenUnreadable(t *testing.T) {
	server := readDevelopmentServer(writeProject(t, map[string]string{"popcornwave.toml": consoleProject}))
	if server.URL != "" || server.PublicMount != "" || server.APIDocURL() != "" {
		t.Errorf("server = %+v, want every value undetermined", server)
	}
}

// A disabled overlay injects no address, which is what leaves the served page
// byte-identical to a production render: with nothing to reach, the framework
// serves no development module and the core carries no import of one.
func TestOverlayInjectionFollowsTheConfiguration(t *testing.T) {
	root := writeProject(t, map[string]string{"popcornwave.toml": consoleProject})
	config, _ := loadProjectConfig(root)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	console := startDevConsole(root, config, nil, stdout, stderr)
	if console == nil {
		t.Fatalf("the console did not start:\n%s", stderr)
	}
	t.Cleanup(console.Close)

	on := strings.Join(consoleEnviron(console, true, true, nil), " ")
	if !strings.Contains(on, envDevConsoleURL+"="+console.URL()) {
		t.Errorf("environ = %q, want the console address", on)
	}
	if strings.Contains(on, envDevConsoleReload) {
		t.Errorf("environ = %q, want no reload variable when reload is on", on)
	}
	if noReload := strings.Join(consoleEnviron(console, true, false, nil), " "); !strings.Contains(noReload, envDevConsoleReload+"=0") {
		t.Errorf("environ = %q, want reload turned off", noReload)
	}
	if off := consoleEnviron(console, false, true, nil); len(off) != 0 {
		t.Errorf("environ = %v, want nothing injected with the overlay off", off)
	}
}

func TestOverlaySwitchesDefaultOnAndParse(t *testing.T) {
	root := writeProject(t, map[string]string{"popcornwave.toml": consoleProject})
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !config.Console.Overlay || !config.Console.Reload {
		t.Errorf("overlay=%v reload=%v, want both on by default", config.Console.Overlay, config.Console.Reload)
	}
	root = writeProject(t, map[string]string{
		"popcornwave.toml": consoleProject + "\n[dev.console.overlay]\nenabled = false\nreload = false\n",
	})
	config, err = loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if config.Console.Overlay || config.Console.Reload {
		t.Errorf("overlay=%v reload=%v, want both off", config.Console.Overlay, config.Console.Reload)
	}
}

// The page displays its own mount base as the export endpoint. An exporter
// pointed there appends the OTLP paths, which the same mount serves, so the
// displayed address is a working one. What pw injects into the application is
// still the receiver's own listener.
func TestTheMountedPaneAcceptsOTLPAtItsOwnBase(t *testing.T) {
	root := writeProject(t, map[string]string{"popcornwave.toml": consoleProject})
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	telemetry, err := startDevTelemetryViewer(config, stdout)
	if err != nil {
		t.Fatalf("viewer: %v", err)
	}
	t.Cleanup(telemetry.close)
	console := startDevConsole(root, config, telemetry, stdout, stderr)
	if console == nil {
		t.Fatalf("the console did not start:\n%s", stderr)
	}
	t.Cleanup(console.Close)

	response, err := http.Post(console.URL()+"/telemetry/v1/traces", "application/json", strings.NewReader(otlpTraceExport))
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("export status = %d, want 200", response.StatusCode)
	}
	// One store behind both addresses, so an export to the console is readable
	// from the receiver's own port.
	if snapshot := body(t, telemetry.url()+"/api/snapshot"); !strings.Contains(snapshot, "GET /users") {
		t.Errorf("the export did not reach the shared store: %s", snapshot)
	}
}
