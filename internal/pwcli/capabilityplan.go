package pwcli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/sessionstore"
)

// capabilityPlan is everything installing a capability would do. It is computed
// in full before anything is written, so a step that cannot succeed stops the
// command instead of leaving a half-installed capability in the project.
type capabilityPlan struct {
	// creates maps a project-relative path to the file to write. A destination
	// that already exists is a conflict, never an overwrite.
	creates map[string]string
	// appends maps a project-relative path to text added at its end. Appending
	// rather than rewriting keeps operator comments and tuned values.
	appends map[string]string
	// edits maps a project-relative path to its full replacement, for the few
	// changes that have to land inside an existing structure.
	edits map[string]string
	// directories are created empty, for a capability whose files are the
	// project's to write.
	directories []string
	// manual lists what the framework will not do to an application-owned file.
	manual []string
	// next lists the commands that finish the installation.
	next []string
	// generate reports whether the capability added generated sources.
	generate bool
}

func newCapabilityPlan() *capabilityPlan {
	return &capabilityPlan{
		creates: map[string]string{},
		appends: map[string]string{},
		edits:   map[string]string{},
	}
}

// summary lists every change, which is what the review screen shows: the
// operator approves the effect on the project, not just the answers.
func (p *capabilityPlan) summary() []string {
	var lines []string
	for _, path := range sortedKeys(p.creates) {
		lines = append(lines, "create  "+path)
	}
	for _, path := range p.directories {
		lines = append(lines, "create  "+path+"/")
	}
	for _, path := range sortedKeys(p.appends) {
		lines = append(lines, "append  "+path)
	}
	for _, path := range sortedKeys(p.edits) {
		lines = append(lines, "edit    "+path)
	}
	for _, line := range p.manual {
		lines = append(lines, "by hand "+line)
	}
	for _, command := range p.next {
		lines = append(lines, "then    "+command)
	}
	return lines
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// changes turns the plan into the atomic write applyFileChanges performs. A
// destination that already exists stops the command here, before a single
// file has been touched.
func (p *capabilityPlan) changes(root string) ([]fileChange, error) {
	var changes []fileChange
	for _, path := range sortedKeys(p.creates) {
		target := filepath.Join(root, filepath.FromSlash(path))
		if _, err := os.Stat(target); err == nil {
			return nil, fmt.Errorf("%s already exists; the framework never overwrites a file the application owns", path)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		changes = append(changes, fileChange{path: target, source: []byte(p.creates[path])})
	}
	for _, path := range sortedKeys(p.appends) {
		target := filepath.Join(root, filepath.FromSlash(path))
		current, err := os.ReadFile(target)
		if err != nil {
			return nil, err
		}
		changes = append(changes, fileChange{path: target, source: append(current, p.appends[path]...)})
	}
	for _, path := range sortedKeys(p.edits) {
		changes = append(changes, fileChange{
			path:   filepath.Join(root, filepath.FromSlash(path)),
			source: []byte(p.edits[path]),
		})
	}
	return changes, nil
}

// apply creates the directories and then writes every change at once.
func (p *capabilityPlan) apply(root string) error {
	changes, err := p.changes(root)
	if err != nil {
		return err
	}
	for _, directory := range p.directories {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(directory)), 0o755); err != nil {
			return err
		}
	}
	for _, change := range changes {
		if err := os.MkdirAll(filepath.Dir(change.path), 0o755); err != nil {
			return err
		}
	}
	return applyFileChanges(changes)
}

// planCapability computes what installing one capability would do.
func planCapability(state projectState, options addOptions) (*capabilityPlan, error) {
	plan := newCapabilityPlan()
	switch options.Capability {
	case capabilityDevbox:
		return plan, planDevbox(state, plan)
	case capabilityDatabase:
		return plan, planDatabase(state, options, plan)
	case capabilityRedis:
		return plan, planRedisValkey(state, plan)
	case capabilityAuth:
		return plan, planAuth(state, options, plan)
	case capabilityTailwind:
		return plan, planTailwind(state, plan)
	case capabilityDiscovered:
		return plan, planPages(state, plan)
	case capabilityRegistered:
		return plan, planHandlers(state, plan)
	}
	return nil, fmt.Errorf("unknown capability %q", options.Capability)
}

// planDatabase configures the pool and, for a project that has neither yet,
// scaffolds the same starter schema and query api:cli-init writes.
func planDatabase(state projectState, options addOptions, plan *capabilityPlan) error {
	engine := engineFor(options.Engine)
	for _, name := range state.configFiles {
		plan.appends[name] = databaseRuntimeSection(options.databaseDSN(state.config.Name), engine)
	}
	migrations := state.config.Migration.Dir
	if len(state.migrations) == 0 {
		plan.creates[migrations+"/00001_init.sql"] = engine.Schema
	} else {
		plan.directories = append(plan.directories, migrations)
	}
	// The SQL example needs a purpose to read it, and generate.queries has no
	// default, so the directory and its entry are written together or not at
	// all.
	document, err := os.ReadFile(filepath.Join(state.root, "popcornwave.toml"))
	if err != nil {
		return err
	}
	edited := string(document)
	if len(state.config.Generate.Queries) == 0 {
		plan.creates["queries/users.pw.sql"] = starterQuery()
		if edited, err = setGeneratePurpose(state, capabilityQueriesPurpose, []string{"queries"}); err != nil {
			return err
		}
		plan.generate = true
	}
	// Generation needs the dialect its .pw.sql sources are written in, and the
	// engine question is the only place a project states it.
	if edited, err = setProjectDatabase(edited, options.Engine); err != nil {
		return err
	}
	plan.edits["popcornwave.toml"] = edited
	// The entry point is application-owned, so the blank import that links the
	// engine is printed rather than injected.
	if engine.DriverImport != "" {
		plan.manual = append(plan.manual,
			"blank-import "+engine.DriverImport+" from the application entry point")
	}
	if engine.DevboxPackage != "" && state.devbox != "" {
		edited, err := addDevboxPackage(state.devbox, engine.DevboxPackage)
		if err != nil {
			return err
		}
		plan.edits["devbox.json"] = edited
	}
	plan.next = append(plan.next, "pw migrate up")
	return nil
}

// planDevbox writes the development environment, carrying the packages this
// project already needs: the toolchain it was scaffolded for, and Tailwind when
// it is enabled. Valkey arrives with its own capability.
func planDevbox(state projectState, plan *capabilityPlan) error {
	packages := []string{"go@latest"}
	if state.config.Toolchain == toolchainTinyGo {
		packages = append(packages, "tinygo@latest")
	}
	if state.config.Tailwind.Enabled {
		packages = append(packages, tailwindDevboxPackage)
	}
	plan.creates["devbox.json"] = devboxScaffold(packages)
	plan.creates["devbox.lock"] = "{}\n"
	plan.next = append(plan.next, "devbox shell")
	return nil
}

// planRedisValkey adds the development server to the Devbox environment.
func planRedisValkey(state projectState, plan *capabilityPlan) error {
	edited, err := addDevboxPackage(state.devbox, "valkey@latest")
	if err != nil {
		return err
	}
	plan.edits["devbox.json"] = edited
	plan.next = append(plan.next, "devbox shell")
	return nil
}

// planAuth installs a login. The framework serves the endpoints itself, so the
// application-side wiring is the account resolver and one call in main.
func planAuth(state projectState, options addOptions, plan *capabilityPlan) error {
	version := state.nextMigrationVersion()
	migrations := state.config.Migration.Dir
	// The project already chose its engine, and no engine reads another's DDL.
	dialect := engineDialect(state.config.Database)
	sessionMigration, err := sessionstore.MigrationSQL(dialect, "popcornwave_session")
	if err != nil {
		return err
	}
	authMigration, err := auth.MigrationSQL(dialect)
	if err != nil {
		return err
	}
	plan.creates[migrations+"/"+migrationFileName(version, sessionstore.MigrationName)] = sessionMigration
	plan.creates[migrations+"/"+migrationFileName(version+1, auth.MigrationName)] = authMigration

	// pw add installs the OIDC mode. A passkey mode additionally needs a
	// relying-party registration that depends on the origin this deployment is
	// reached on, which pw add cannot know, so it stays a pw init answer.
	scaffold := initOptions{
		Name: state.config.Name, Auth: authOIDC,
		// pw add installs the rdb backend, which is the one that fits a project
		// that already has a database. pw init offers the other two.
		Session: sessionRDB, AuthEmulator: options.AuthEmulator,
	}
	handlers := handlerPackageDirectory(state)
	resolver := handlers + "/accounts.go"
	plan.creates[resolver] = accountsScaffold(scaffold)

	section := authRuntimeConfig(scaffold)
	for _, name := range state.configFiles {
		plan.appends[name] = section
	}
	if options.AuthEmulator {
		plan.creates[defaultIdPConfig] = devIdPRoster()
		plan.appends["popcornwave.toml"] = devIdPProjectConfig(initOptions{AuthEmulator: true})
	}
	plan.manual = append(plan.manual,
		"call "+goPackageIdentifier(handlers)+".RegisterAccounts() in "+state.config.Main+" before pw.Run",
		// Storage is opt-in by blank import, so both stores this configuration
		// selects have to be imported by the application: the sessions and the
		// single-use login records.
		`add import _ "`+sessionBackendPlugin(sessionRDB, dialect)+`" to `+state.config.Main,
		`add import _ "github.com/shibukawa/popcornwave/authstate/`+dialect+`" to `+state.config.Main)
	plan.next = append(plan.next, "pw migrate up")
	plan.generate = true
	return nil
}

// planTailwind wires the pinned toolchain in. The stylesheet link belongs in
// the application-owned document shell, so it is printed rather than injected.
func planTailwind(state projectState, plan *capabilityPlan) error {
	plan.creates[defaultTailwindInput] = tailwindEntryScaffold(state.config.Generate)
	plan.creates[defaultTailwindOutput] = "/* Generated by Tailwind CSS. */\n"
	plan.appends["popcornwave.toml"] = tailwindProjectConfig()
	if state.devbox == "" {
		// A project without the Devbox environment installs the toolchain its
		// own way, so naming the version is all the framework can do.
		plan.manual = append(plan.manual, "install "+tailwindToolchainRequirement)
	} else {
		edited, err := addDevboxPackage(state.devbox, tailwindDevboxPackage)
		if err != nil {
			return err
		}
		plan.edits["devbox.json"] = edited
	}
	plan.manual = append(plan.manual,
		`add <link rel="stylesheet" href="/`+defaultTailwindOutput+`"> to the document shell`)
	plan.next = append(plan.next, "devbox shell")
	return nil
}

// planPages installs a page tree: the same starter tree api:cli-init writes,
// and the purpose that makes it generate. The two go together, because a tree
// no purpose lists is a directory nothing reads.
//
// The Register call is printed rather than injected, since the entry point is
// application-owned like every other main.go edit this command would rather not
// make.
func planPages(state projectState, plan *capabilityPlan) error {
	root := defaultDiscoveredDir
	for path, source := range pageTreeScaffold(initOptions{Name: state.config.Name}, root) {
		plan.creates[path] = source
	}
	edited, err := setPagesPurpose(state, []string{root})
	if err != nil {
		return err
	}
	plan.edits["popcornwave.toml"] = edited
	plan.manual = append(plan.manual,
		"import "+state.config.Name+"/"+root+" from "+state.config.Main+
			" and call "+goPackageIdentifier(root)+".Register on its mux")
	plan.generate = true
	return nil
}

// planHandlers installs the registered router into a project that started with
// the page tree alone: the package, its mux, and one route example.
//
// The template purpose gains the same directory, because a page template sits
// beside the handler that renders it, and the mounting is printed for the same
// reason a page tree's Register call is.
func planHandlers(state projectState, plan *capabilityPlan) error {
	directory := defaultRegisteredDir
	options := initOptions{
		Name:   state.config.Name,
		TinyGo: state.config.Toolchain == toolchainTinyGo,
		Auth:   authNone,
	}
	for path, source := range registeredRouterScaffold(options, directory) {
		plan.creates[path] = source
	}

	edited, err := setGeneratePurpose(state, "handlers", []string{directory})
	if err != nil {
		return err
	}
	templates := append([]string{directory}, state.config.Generate.Templates...)
	sort.Strings(templates)
	templates = slices.Compact(templates)
	if edited, err = setGeneratePurposeIn(edited, "templates", templates); err != nil {
		return err
	}
	plan.edits["popcornwave.toml"] = edited
	plan.manual = append(plan.manual,
		"import "+state.config.Name+"/"+directory+" from "+state.config.Main+" and serve its Handlers()")
	plan.generate = true
	return nil
}

// handlerPackageDirectory picks where an application-side source belongs. The
// handler purpose is the answer by construction: it is the list of directories
// api:cli-generate reads for routes.
func handlerPackageDirectory(state projectState) string {
	if len(state.config.Generate.Handlers) > 0 {
		return state.config.Generate.Handlers[0]
	}
	return "handlers"
}

// goPackageIdentifier names the package a directory holds, for a printed
// instruction that has to compile once the operator follows it.
func goPackageIdentifier(directory string) string {
	base := filepath.Base(filepath.FromSlash(directory))
	return strings.NewReplacer("-", "", ".", "").Replace(base)
}

func starterMigration() string {
	return `-- +goose Up
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE users;
`
}

func starterQuery() string {
	return `package queries

type User {
  id: int
  name: string
}

export statement FindUser(id: int): sql.one<User> {
SELECT id, name FROM users WHERE id = {id}
}
`
}
