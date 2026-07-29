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
	case "add":
		err = runAdd(ctx, args[1:], stdout)
	case "new":
		err = runNew(ctx, args[1:], stdout)
	case "generate":
		err = runGenerate(ctx, args[1:], stdout)
	case "migrate":
		err = runMigrate(ctx, args[1:], stdout, stderr)
	case "seed":
		err = runSeed(ctx, args[1:], stdout, stderr)
	case "build":
		err = runBuild(ctx, args[1:], stdout, stderr)
	case "dev":
		err = runDev(ctx, args[1:], stdout, stderr)
	case "version", "--version", "-v":
		err = runVersion(args[1:], stdout)
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
	fmt.Fprintln(w, "Commands: init, add, new, generate, migrate, seed, build, dev, version")
	fmt.Fprintln(w, "Init usage: pw init [<project-name>] [--interactive] [--tailwind] [--no-tinygo]")
	fmt.Fprintln(w, "  Omit the project name to answer the same questions in the wizard.")
	fmt.Fprintln(w, "  A capability declined here can be enabled later with pw add.")
	fmt.Fprintln(w, addUsage)
	fmt.Fprintln(w, "  Omit the capability to pick from what this project does not already have.")
	fmt.Fprintln(w, newUsage)
	fmt.Fprintln(w, "Migrate actions: status, version, up, up-by-one, up-to, down, down-to, create, validate, snapshot")
	fmt.Fprintln(w, "Seed usage: pw seed [--dir=testdata/seed] [name...]")
}
