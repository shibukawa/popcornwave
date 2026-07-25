package pwcli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	var err error
	switch args[0] {
	case "init":
		err = runInit(args[1:], stdout)
	case "generate":
		err = runGenerate(ctx, args[1:], stdout)
	case "schema-init":
		err = runSchemaInit(ctx, args[1:], stdout, stderr)
	case "seed":
		err = runSeed(ctx, args[1:], stdout, stderr)
	case "build":
		err = runBuild(ctx, args[1:], stdout, stderr)
	case "dev":
		err = runDev(ctx, args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		err = fmt.Errorf("unknown command %q", args[0])
	}
	if err != nil {
		fmt.Fprintln(stderr, "pw:", err)
		return 1
	}
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: pw <command>")
	fmt.Fprintln(w, "Commands: init, generate, schema-init, seed, build, dev")
	fmt.Fprintln(w, "  seed [--dir=testdata/seed] [name...]  load datasets into the configured database")
}
