package pwcli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/shibukawa/popcornwave/internal/configview"
	"github.com/shibukawa/popcornwave/internal/pwmigrate"
	"github.com/shibukawa/popcornwave/migrate"
)

// migrateActions is the accepted action set. Help and validation read the same
// slice, so an added action cannot be accepted by one and unknown to the other.
var migrateActions = []string{
	"status", "version", "up", "up-by-one", "up-to",
	"down", "down-to", "create", "validate", "snapshot",
}

const migrateUsage = "usage: pw migrate <action> [<version>|<name>] [--dir=path] [--dsn=dsn] [--dry-run] [--yes]"

type migrateOptions struct {
	action  string
	version int64
	name    string
	dir     string
	dsn     string
	confirm bool
	dryRun  bool
}

// project carries the optional project context of a migrate invocation. A
// delegated child process runs with an explicit directory and an environment
// DSN, so it has no project to load.
type project struct {
	root   string
	config projectConfig
	err    error
}

func (context project) require() error {
	if context.err != nil {
		return context.err
	}
	return nil
}

func runMigrate(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	options, err := parseMigrateArgs(args)
	if err != nil {
		return err
	}
	var located project
	located.root, located.err = projectRoot(".")
	if located.err == nil {
		located.config, located.err = loadProjectConfig(located.root)
	}
	return executeMigrate(ctx, located, options, stdout, stderr)
}

func executeMigrate(ctx context.Context, located project, options migrateOptions, stdout, stderr io.Writer) error {
	directory := options.dir
	if directory == "" {
		if err := located.require(); err != nil {
			return err
		}
		directory = located.config.Migration.Dir
	}
	if !filepath.IsAbs(directory) {
		base := located.root
		if base == "" {
			working, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("migrate: resolve migration directory: %w", err)
			}
			base = working
		}
		directory = filepath.Join(base, filepath.FromSlash(directory))
	}
	root := located.root
	if root == "" {
		root = directory
	}
	var err error

	switch options.action {
	case "create":
		path, err := pwmigrate.Create(directory, options.name)
		if err != nil {
			return fmt.Errorf("migrate create: %w", err)
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			relative = path
		}
		fmt.Fprintln(stdout, "created", relative)
		return nil
	case "validate":
		sources, err := pwmigrate.Sources(directory)
		if err != nil {
			return fmt.Errorf("migrate validate: %w", err)
		}
		listed, err := pwmigrate.Validate(sources)
		if err != nil {
			return fmt.Errorf("migrate validate: %w", err)
		}
		for _, item := range listed {
			fmt.Fprintf(stdout, "%d\t%s\n", item.Version, item.Path)
		}
		fmt.Fprintf(stdout, "%d migration(s) are valid\n", len(listed))
		return nil
	case "snapshot":
		sources, err := pwmigrate.Sources(directory)
		if err != nil {
			return fmt.Errorf("migrate snapshot: %w", err)
		}
		script, err := pwmigrate.Snapshot(ctx, sources)
		if err != nil {
			return fmt.Errorf("migrate snapshot: %w", err)
		}
		_, err = io.WriteString(stdout, script)
		return err
	}

	sources, err := pwmigrate.Sources(directory)
	if err != nil {
		return fmt.Errorf("migrate %s: %w", options.action, err)
	}
	dsn := options.dsn
	if dsn == "" {
		// A delegated child process hands the DSN over in the environment so it
		// never appears in a process argument.
		dsn = strings.TrimSpace(os.Getenv(migrate.DSNEnv))
	}
	if dsn == "" {
		if err := located.require(); err != nil {
			return err
		}
		dsn, err = resolveApplicationDSN(ctx, located.root, located.config.Main, stderr)
		if err != nil {
			return fmt.Errorf("migrate %s: %w", options.action, err)
		}
	}
	target, err := pwmigrate.Open(dsn)
	if err != nil {
		return fmt.Errorf("migrate %s: %w", options.action, redactDSN(err, dsn))
	}
	defer target.Close()

	switch options.action {
	case "status":
		return reportStatus(ctx, target, sources, stdout)
	case "version":
		version, err := pwmigrate.Version(ctx, target, sources)
		if err != nil {
			return fmt.Errorf("migrate version: %w", err)
		}
		fmt.Fprintln(stdout, version)
		return nil
	}

	action, err := migrateAction(options.action)
	if err != nil {
		return err
	}
	if options.dryRun {
		return reportPlan(ctx, target, sources, options, stdout)
	}
	if isRollback(action) && !options.confirm {
		confirmed, err := confirmRollback(ctx, target, sources, options, stdout)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Fprintln(stdout, "cancelled")
			return nil
		}
	}
	// Every declared package's stream applies before the application's own, so a
	// package's tables exist before anything the application writes can
	// reference them. A rollback leaves them alone: the application is reversing
	// its own schema, and a package's stream is not its to move.
	if !isRollback(action) {
		if err := applyPackageStreams(ctx, located, target, stdout); err != nil {
			return fmt.Errorf("migrate %s: %w", options.action, redactDSN(err, dsn))
		}
	}
	result, err := pwmigrate.Apply(ctx, target, sources, action, options.version)
	for _, applied := range result.Applied {
		fmt.Fprintf(stdout, "%s\t%d\t%s\t%s\n",
			applied.Direction, applied.Version, filepath.Base(applied.Path), applied.Duration.Round(1e6))
	}
	fmt.Fprintf(stdout, "version %d -> %d\n", result.Previous, result.Current)
	if err != nil {
		return fmt.Errorf("migrate %s: %w", options.action, redactDSN(err, dsn))
	}
	return nil
}

func migrateAction(name string) (pwmigrate.Action, error) {
	switch name {
	case "up":
		return pwmigrate.ActionUp, nil
	case "up-by-one":
		return pwmigrate.ActionUpByOne, nil
	case "up-to":
		return pwmigrate.ActionUpTo, nil
	case "down":
		return pwmigrate.ActionDown, nil
	case "down-to":
		return pwmigrate.ActionDownTo, nil
	}
	return "", fmt.Errorf("migrate: unknown action %q", name)
}

func isRollback(action pwmigrate.Action) bool {
	return action == pwmigrate.ActionDown || action == pwmigrate.ActionDownTo
}

func reportStatus(ctx context.Context, target *pwmigrate.Target, sources fs.FS, stdout io.Writer) error {
	statuses, err := pwmigrate.Statuses(ctx, target, sources)
	if err != nil {
		return fmt.Errorf("migrate status: %w", err)
	}
	pending := 0
	for _, status := range statuses {
		state := "pending"
		if status.Applied {
			state = status.AppliedAt.Format("2006-01-02 15:04:05")
		} else {
			pending++
		}
		fmt.Fprintf(stdout, "%d\t%s\t%s\n", status.Version, filepath.Base(status.Path), state)
	}
	fmt.Fprintf(stdout, "%d pending\n", pending)
	return nil
}

func reportPlan(ctx context.Context, target *pwmigrate.Target, sources fs.FS, options migrateOptions, stdout io.Writer) error {
	statuses, err := pwmigrate.Statuses(ctx, target, sources)
	if err != nil {
		return fmt.Errorf("migrate %s: %w", options.action, err)
	}
	planned := plannedMigrations(statuses, options)
	for _, status := range planned {
		fmt.Fprintf(stdout, "would %s\t%d\t%s\n", options.action, status.Version, filepath.Base(status.Path))
	}
	fmt.Fprintf(stdout, "%d migration(s) would run\n", len(planned))
	return nil
}

// plannedMigrations lists the migrations an action would touch, in the order it
// would touch them. Rollbacks run newest first.
func plannedMigrations(statuses []pwmigrate.Status, options migrateOptions) []pwmigrate.Status {
	var planned []pwmigrate.Status
	switch options.action {
	case "up", "up-by-one", "up-to":
		for _, status := range statuses {
			if status.Applied {
				continue
			}
			if options.action == "up-to" && status.Version > options.version {
				continue
			}
			planned = append(planned, status)
			if options.action == "up-by-one" {
				return planned
			}
		}
	case "down", "down-to":
		for index := len(statuses) - 1; index >= 0; index-- {
			status := statuses[index]
			if !status.Applied {
				continue
			}
			if options.action == "down-to" && status.Version <= options.version {
				continue
			}
			planned = append(planned, status)
			if options.action == "down" {
				return planned
			}
		}
	}
	return planned
}

func confirmRollback(ctx context.Context, target *pwmigrate.Target, sources fs.FS, options migrateOptions, stdout io.Writer) (bool, error) {
	if err := reportPlan(ctx, target, sources, options, stdout); err != nil {
		return false, err
	}
	fmt.Fprint(stdout, "roll back these migrations? [y/N] ")
	var answer string
	if _, err := fmt.Fscanln(os.Stdin, &answer); err != nil {
		// A redirected or closed stdin cannot answer, and silently doing nothing
		// would look like the rollback succeeded.
		fmt.Fprintln(stdout)
		return false, fmt.Errorf("migrate %s: rollback needs an answer on stdin, or --yes", options.action)
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

// resolveApplicationDSN asks the application for its effective DSN so the CLI
// never reimplements TOML, environment, and flag precedence. The value arrives
// on a pipe and is never placed in a process argument.
//
// The build carries -tags=pwdev because that is what --pw-print-dsn now needs. A
// release build does not answer it: the flag prints the database password, and
// it used to do so for anyone who could execute the binary. Compiling from
// source here already, this costs nothing.
func resolveApplicationDSN(ctx context.Context, root, mainPackage string, stderr io.Writer) (string, error) {
	var output bytes.Buffer
	command := exec.CommandContext(ctx, "go", "run", "-tags=pwdev", mainPackage, "--pw-print-dsn")
	command.Dir = root
	command.Stdout = &output
	command.Stderr = stderr
	command.Env = os.Environ()
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("resolve database configuration: %w", err)
	}
	dsn := strings.TrimSpace(output.String())
	if dsn == "" {
		return "", errors.New("resolve database configuration: application reported no DSN")
	}
	return dsn, nil
}

// redactDSN keeps credentials out of reported failures. The DSN is replaced by
// the same form the startup summary and pw doctor show, so a reader comparing a
// failure with a summary sees one address written one way; a password that
// reached the message by some other route is scrubbed separately.
func redactDSN(err error, dsn string) error {
	if err == nil || dsn == "" {
		return err
	}
	message := strings.ReplaceAll(err.Error(), dsn, configview.DSN(dsn))
	if secret := credentialPart(dsn); secret != "" {
		message = strings.ReplaceAll(message, secret, configview.Redacted)
	}
	return errors.New(message)
}

func credentialPart(dsn string) string {
	_, remainder, ok := strings.Cut(dsn, "://")
	if !ok {
		return ""
	}
	credentials, _, ok := strings.Cut(remainder, "@")
	if !ok {
		return ""
	}
	if _, password, ok := strings.Cut(credentials, ":"); ok && password != "" {
		return password
	}
	return ""
}

func parseMigrateArgs(args []string) (migrateOptions, error) {
	if len(args) == 0 {
		return migrateOptions{}, errors.New("migrate: an action is required; " + migrateUsage)
	}
	options := migrateOptions{action: args[0]}
	rest := args[1:]
	needsVersion := options.action == "up-to" || options.action == "down-to"
	needsName := options.action == "create"
	var positional []string
	for index := 0; index < len(rest); index++ {
		arg := rest[index]
		switch {
		case arg == "--yes":
			options.confirm = true
		case arg == "--dry-run":
			options.dryRun = true
		case arg == "--dir", arg == "--dsn":
			if index+1 >= len(rest) {
				return migrateOptions{}, fmt.Errorf("migrate: %s requires a value", arg)
			}
			index++
			if arg == "--dir" {
				options.dir = rest[index]
			} else {
				options.dsn = rest[index]
			}
		case strings.HasPrefix(arg, "--dir="):
			options.dir = strings.TrimPrefix(arg, "--dir=")
		case strings.HasPrefix(arg, "--dsn="):
			options.dsn = strings.TrimPrefix(arg, "--dsn=")
		case strings.HasPrefix(arg, "-"):
			return migrateOptions{}, fmt.Errorf("migrate: unknown flag %s", arg)
		default:
			positional = append(positional, arg)
		}
	}
	switch {
	case needsVersion:
		if len(positional) != 1 {
			return migrateOptions{}, fmt.Errorf("migrate %s: a version is required", options.action)
		}
		version, err := strconv.ParseInt(positional[0], 10, 64)
		if err != nil {
			return migrateOptions{}, fmt.Errorf("migrate %s: invalid version %q", options.action, positional[0])
		}
		options.version = version
	case needsName:
		if len(positional) != 1 {
			return migrateOptions{}, errors.New("migrate create: a name is required")
		}
		options.name = positional[0]
	case len(positional) != 0:
		return migrateOptions{}, fmt.Errorf("migrate %s: unexpected argument %q", options.action, positional[0])
	}
	if !slices.Contains(migrateActions, options.action) {
		return migrateOptions{}, fmt.Errorf("migrate: unknown action %q; want %s",
			options.action, strings.Join(migrateActions, ", "))
	}
	return options, nil
}

// applyPackageStreams brings every declared package's migrations up before the
// application's own run.
//
// The pending statements are printed first. Nothing was copied into the project,
// so this listing is what replaces the review a migration written into the
// project used to get: an operator sees what a dependency is about to do to
// their database before it does it.
func applyPackageStreams(ctx context.Context, located project, target *pwmigrate.Target, stdout io.Writer) error {
	if len(located.config.Packages) == 0 {
		return nil
	}
	resolved, err := resolvePackages(ctx, located.root, located.config.Packages)
	if err != nil {
		return err
	}
	if err := checkPackageCompatibility(located.config, resolved, nil); err != nil {
		return err
	}
	streams, err := packageStreams(ctx, located.root, resolved)
	if err != nil {
		return err
	}
	for _, stream := range streams {
		pending, err := pwmigrate.StreamPending(ctx, target, stream)
		if err != nil {
			return err
		}
		for _, status := range pending {
			fmt.Fprintf(stdout, "pending\t%s\t%d\t%s\n", stream.Module, status.Version, filepath.Base(status.Path))
		}
	}
	results, err := pwmigrate.ApplyStreams(ctx, target, streams)
	for _, result := range results {
		for _, applied := range result.Result.Applied {
			fmt.Fprintf(stdout, "%s\t%s\t%d\t%s\t%s\n",
				applied.Direction, result.Module, applied.Version,
				filepath.Base(applied.Path), applied.Duration.Round(1e6))
		}
	}
	return err
}
