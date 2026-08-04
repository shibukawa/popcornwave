package pwcli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/shibukawa/popcornwave/internal/devconsole"
	"github.com/shibukawa/popcornwave/internal/pwenv"
	"github.com/shibukawa/tinybind-go/minitoml"
)

// startDevConsole serves the development console beside the application.
//
// A console that cannot listen is reported and skipped rather than fatal: the
// developer loop exists to run the application, and an unobservable run is
// still a working one. The port is fixed, so a collision is a real conflict
// with a real remedy, and saying which address is taken is more useful than
// quietly moving to another one.
func startDevConsole(root string, config projectConfig, telemetry *devTelemetryViewer, stdout, stderr io.Writer) *devconsole.Console {
	if !config.Console.Enabled {
		return nil
	}
	console, err := devconsole.New(
		"127.0.0.1:"+strconv.Itoa(config.Console.Port),
		devconsole.Project{
			Name:           config.Name,
			Environment:    developmentEnvironment(),
			ApplicationURL: applicationURL(root),
		},
		devConsolePanes(root, config, telemetry),
	)
	if err != nil {
		fmt.Fprintln(stderr, "pw dev: console:", err)
		return nil
	}
	fmt.Fprintf(stdout, "pw dev: console %s\n", console.URL())
	return console
}

// devConsolePanes lists every pane the console knows about, enabled or not. A
// disabled pane is listed with the key that would enable it rather than left
// out, so a developer who expected a surface is told why it is missing instead
// of wondering whether the version they run has it.
func devConsolePanes(root string, config projectConfig, telemetry *devTelemetryViewer) []devconsole.Pane {
	panes := []devconsole.Pane{{
		Slug:    "telemetry",
		Title:   "telemetry",
		Summary: "traces, logs, and the timing of what the application did",
		// The whole handler is mounted under the pane prefix, so this names no
		// path. The UI resolves its API against the served document and its
		// assets relatively, which is what lets receiver, snapshot API, and
		// page all follow the mount together.
		//
		// The endpoint the page displays follows too: it is the mount base,
		// and an exporter pointed there appends the OTLP paths, which this
		// mount serves. What pw injects into the application is still the
		// receiver's own listener, so both addresses work and neither lies.
		Handler:    telemetry.paneHandler(),
		DisabledBy: "dev.otel.enabled",
	}}
	assets := devconsole.Pane{
		Slug:       "assets",
		Title:      "assets",
		Summary:    "what public/ serves now, and what a release build would serve",
		DisabledBy: "dev.console.assets.enabled",
	}
	if config.Console.Assets {
		assets.Handler = devconsole.AssetPane(devconsole.AssetSource{
			Root:            root,
			Mount:           publicMount(root),
			TailwindEnabled: config.Tailwind.Enabled,
			TailwindInput:   config.Tailwind.Input,
			TailwindOutput:  config.Tailwind.Output,
			// The build's own eligibility test, so the pane and pw build
			// cannot disagree about what would be compressed.
			Compressible: publicAssetCompressible,
		})
	}
	return append(panes, assets)
}

// envDevConsoleURL names the variable the application reads to find the
// console. It matches pw.DevConsoleURLVar, which is declared in the pwdev half
// of the framework and so cannot be referenced from a host build.
const envDevConsoleURL = "PW_DEV_CONSOLE_URL"

// consoleEnviron adds the resolved console address to the application
// environment, preserving a value the developer already exported.
//
// It is injected rather than configured for the same reason the OTLP endpoint
// is: pw dev resolves the address at startup, and a project that wrote it down
// would be committing a development port.
func consoleEnviron(console *devconsole.Console, base []string) []string {
	if console == nil {
		return base
	}
	if value, ok := os.LookupEnv(envDevConsoleURL); ok && value != "" {
		return base
	}
	return append(base, envDevConsoleURL+"="+console.URL())
}

// developmentEnvironment is the APP_ENV the loop runs the application under,
// which is what startApplication injects.
func developmentEnvironment() string {
	if value, ok := os.LookupEnv(pwenv.Var); ok && value != "" {
		return value
	}
	return pwenv.Development
}

// applicationURL reads the port the application is configured to listen on.
//
// This is a best-effort read of the development configuration file and not the
// full resolution api:runtime-configuration performs: an environment variable
// or a flag outranks the file and is not consulted here. An unreadable or
// absent value returns the empty string, which the index reports as
// undetermined rather than filling in with a default that may be wrong.
func applicationURL(root string) string {
	for _, name := range []string{
		pwenv.FileName(pwenv.Development),
		filepath.Join("config", pwenv.FileName(pwenv.Development)),
	} {
		source, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue
		}
		document, err := minitoml.Parse(source)
		if err != nil {
			continue
		}
		value, ok := document.Get("server.port")
		if !ok {
			continue
		}
		port, err := value.AsInt()
		if err != nil || port <= 0 || port > 65535 {
			continue
		}
		return "http://localhost:" + strconv.FormatInt(port, 10)
	}
	return ""
}

// publicMount reads the configured public mount the same best-effort way, so
// the asset pane can name the URL each file answers. An absent key means the
// endpoint is not configured here, which the pane reports as undetermined.
func publicMount(root string) string {
	for _, name := range []string{
		pwenv.FileName(pwenv.Development),
		filepath.Join("config", pwenv.FileName(pwenv.Development)),
	} {
		source, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue
		}
		document, err := minitoml.Parse(source)
		if err != nil {
			continue
		}
		if value, ok := document.Get("server.public.mount"); ok {
			if mount, err := value.AsString(); err == nil && mount != "" {
				return mount
			}
		}
	}
	return ""
}
