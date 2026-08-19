package pwcli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/shibukawa/popcornweb/internal/pwlsp"
)

const lspUsage = "usage: pw lsp [--stdio] [--log=<path>] [--root=<path>]"

type lspOptions struct {
	Log  string
	Root string
}

// parseLSPOptions reads the api:cli-lsp arguments. --stdio is accepted and
// does nothing, because stdio is the only transport the first release ships
// and a client that passes the flag should not be refused for it.
func parseLSPOptions(args []string) (lspOptions, error) {
	options := lspOptions{}
	for _, arg := range args {
		name, value, hasValue := strings.Cut(arg, "=")
		switch {
		case arg == "--stdio":
		case name == "--log" && hasValue:
			if value == "" {
				return options, fmt.Errorf("lsp: --log needs a path; %s", lspUsage)
			}
			options.Log = value
		case name == "--root" && hasValue:
			if value == "" {
				return options, fmt.Errorf("lsp: --root needs a path; %s", lspUsage)
			}
			options.Root = value
		default:
			return options, fmt.Errorf("lsp: unknown argument %q; %s", arg, lspUsage)
		}
	}
	return options, nil
}

// runLSP serves one editor session on stdin and stdout.
//
// Nothing is written to stdout but protocol messages: a stray line there ends
// the session, so every diagnostic of the command itself goes to stderr and
// every trace goes to the --log file.
func runLSP(args []string, stdin io.Reader, stdout io.Writer) error {
	options, err := parseLSPOptions(args)
	if err != nil {
		return err
	}

	var log io.Writer
	if options.Log != "" {
		file, err := os.OpenFile(options.Log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("lsp: cannot write the log: %w", err)
		}
		defer file.Close()
		log = file
	}

	root := options.Root
	if root != "" {
		if root, err = filepath.Abs(root); err != nil {
			return fmt.Errorf("lsp: --root: %w", err)
		}
	}

	info, ok := debug.ReadBuildInfo()
	server := pwlsp.NewServer(stdout, pwlsp.Options{
		Root:    root,
		Log:     log,
		Name:    "pw lsp",
		Version: resolveVersion(version, info, ok).Version,
		Load:    loadLSPProject,
	})

	// A stream that ends without a shutdown request is a client that went
	// away, which api:cli-lsp reports as a nonzero exit so the client's
	// restart policy applies. It is not an error to print: the connection is
	// already gone and nobody is reading stderr for it.
	if code := server.Serve(stdin); code != 0 {
		return &exitError{command: "lsp", message: "the client disconnected without a shutdown request"}
	}
	return nil
}
