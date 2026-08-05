package pwcli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/shibukawa/popcornwave/internal/pwenv"
	"github.com/shibukawa/popcornwave/plugin/auth"
)

// Capability names one framework feature a project can carry. api:cli-init
// offers the same set as questions, so a choice declined at bootstrap is not a
// decision the project is stuck with.
const (
	capabilityDevbox   = "devbox"
	capabilityDatabase = "database"
	capabilityRedis    = "redis-valkey"
	capabilityAuth     = "auth"
	capabilityTailwind = "tailwind"
	// capabilityImages installs the image encoders and turns the conversion on.
	// It is a capability rather than a plain configuration key because the
	// encoders are host tools: switching it on without them converts nothing.
	capabilityImages = "images"
	// capabilityDynamo is a second kind of store rather than a fourth SQL
	// engine, so it depends on nothing and combines with any database answer
	// including none.
	capabilityDynamo = "dynamo"
	// capabilityFirestore is the same kind of answer as capabilityDynamo, in
	// Datastore mode on Google Cloud. The two are independent capabilities: a
	// project may install either, both, or neither.
	capabilityFirestore = "firestore"
	// capabilityRegistered and capabilityDiscovered are the two routers the one
	// question of decision:page-router-scaffold-choice selects between, so
	// either can be installed into a project that started with the other. They
	// are named after the router rather than after the directory it reads,
	// because the directory is a data:project-config value.
	capabilityRegistered = routerRegistered
	capabilityDiscovered = routerDiscovered
)

// capabilityOrder lists the catalog in the order the wizard offers it, which
// puts a capability before the ones that depend on it.
var capabilityOrder = []string{
	capabilityRegistered, capabilityDiscovered,
	capabilityDevbox, capabilityDatabase, capabilityDynamo, capabilityFirestore, capabilityRedis, capabilityAuth, capabilityTailwind,
	capabilityImages,
}

// capabilitySummary is the one-line description shown beside each choice.
var capabilitySummary = map[string]string{
	capabilityDevbox:     "the reproducible development environment and its toolchain",
	capabilityDatabase:   "rdb configuration, the migration directory, and a typed SQL example",
	capabilityRedis:      "the Valkey development server in devbox.json",
	capabilityAuth:       "login sessions, the framework tables, and the account resolver",
	capabilityTailwind:   "the pinned Tailwind toolchain and its CSS entry point",
	capabilityImages:     "build-time image conversion and the encoders it runs",
	capabilityDynamo:     "the DynamoDB client, its typed records, and a local development server",
	capabilityFirestore:  "the Firestore client, in Datastore mode, against the local emulator",
	capabilityRegistered: "the registered router: a route is a registration written in Go",
	capabilityDiscovered: "the discovered router: a directory holding a page template is a route",
}

// capabilityRequires records the capabilities that cannot stand alone.
var capabilityRequires = map[string]string{
	// Login sessions are stored by the rdb backend of data:session-runtime-config.
	capabilityAuth: capabilityDatabase,
	// The Valkey answer writes nothing but a Devbox package, so without the
	// environment it has nothing to install.
	capabilityRedis: capabilityDevbox,
}

// projectState is a loaded project plus the files api:cli-add has to read to
// decide what a capability would change.
type projectState struct {
	root        string
	config      projectConfig
	configFiles []string
	migrations  []string
	devbox      string
}

// loadProject reads everything the capability probes and plans need.
func loadProjectState(root string) (projectState, error) {
	config, err := loadProjectConfig(root)
	if err != nil {
		return projectState{}, err
	}
	loaded := projectState{root: root, config: config}
	if loaded.configFiles, err = environmentConfigFiles(root); err != nil {
		return projectState{}, err
	}
	if len(loaded.configFiles) == 0 {
		return projectState{}, fmt.Errorf("no %s found; a capability writes its runtime configuration into the environment files",
			pwenv.FileName(pwenv.Development))
	}
	if loaded.migrations, err = migrationFiles(root, config.Migration.Dir); err != nil {
		return projectState{}, err
	}
	devbox, err := os.ReadFile(filepath.Join(root, "devbox.json"))
	if err != nil && !os.IsNotExist(err) {
		return projectState{}, err
	}
	loaded.devbox = string(devbox)
	return loaded, nil
}

// environmentConfigFiles lists the project-local runtime configuration files,
// in both locations policy:config-file-resolution reads.
func environmentConfigFiles(root string) ([]string, error) {
	var found []string
	for _, directory := range []string{".", "config"} {
		entries, err := os.ReadDir(filepath.Join(root, directory))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || !pwenv.IsFileName(entry.Name()) {
				continue
			}
			found = append(found, filepath.ToSlash(filepath.Join(directory, entry.Name())))
		}
	}
	sort.Strings(found)
	return found, nil
}

// migrationFiles lists the migration file names, which carry both the version
// and the rule:framework-owned-tables name stem.
func migrationFiles(root, directory string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(directory)))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var found []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			found = append(found, entry.Name())
		}
	}
	sort.Strings(found)
	return found, nil
}

// nextMigrationVersion returns the next free version. A capability added later
// takes whatever is free at that point, because renumbering a migration a
// project may already have applied is never safe.
func (p projectState) nextMigrationVersion() int {
	highest := 0
	for _, name := range p.migrations {
		version, _, found := strings.Cut(name, "_")
		if !found {
			continue
		}
		number, err := strconv.Atoi(version)
		if err != nil {
			continue
		}
		highest = max(highest, number)
	}
	return highest + 1
}

// migrationFileName renders a version and stem the way the scaffold does.
func migrationFileName(version int, stem string) string {
	return fmt.Sprintf("%05d_%s.sql", version, stem)
}

// hasMigrationStem reports whether a capability already reached the database,
// detected by name stem rather than by version.
func (p projectState) hasMigrationStem(stem string) (string, bool) {
	for _, name := range p.migrations {
		if _, rest, found := strings.Cut(name, "_"); found && rest == stem+".sql" {
			return filepath.ToSlash(filepath.Join(p.config.Migration.Dir, name)), true
		}
	}
	return "", false
}

// readConfigFiles returns the contents of every environment configuration file.
func (p projectState) readConfigFiles() (map[string]string, error) {
	contents := make(map[string]string, len(p.configFiles))
	for _, name := range p.configFiles {
		source, err := os.ReadFile(filepath.Join(p.root, filepath.FromSlash(name)))
		if err != nil {
			return nil, err
		}
		contents[name] = string(source)
	}
	return contents, nil
}

// carries reports whether the project already has a capability, and names the
// file that proves it. Presence is derived from the files that carry it, never
// from a list in data:project-config, so a hand-edited project cannot disagree
// with its own manifest.
func (p projectState) carries(name string) (string, bool, error) {
	switch name {
	case capabilityDevbox:
		if p.devbox != "" {
			return "devbox.json", true, nil
		}
		return "", false, nil
	case capabilityDatabase:
		return p.configSectionEvidence("[middleware.rdb]")
	case capabilityDynamo:
		return p.configSectionEvidence("[middleware.dynamo]")
	case capabilityFirestore:
		return p.configSectionEvidence("[middleware.firestore]")
	case capabilityRedis:
		if strings.Contains(p.devbox, "valkey@") {
			return "devbox.json", true, nil
		}
		return "", false, nil
	case capabilityAuth:
		if path, ok := p.hasMigrationStem(auth.MigrationName); ok {
			return path, true, nil
		}
		return p.configSectionEvidence("[auth]")
	case capabilityTailwind:
		if p.config.Tailwind.Enabled {
			return "popcornwave.toml", true, nil
		}
		return "", false, nil
	case capabilityImages:
		if p.config.Assets.Images {
			return "popcornwave.toml", true, nil
		}
		return "", false, nil
	case capabilityDiscovered:
		// The purpose is the capability: a tree nothing generates from is not
		// installed, whatever the directory holds.
		if len(p.config.Generate.Pages) > 0 {
			return "popcornwave.toml", true, nil
		}
		return "", false, nil
	case capabilityRegistered:
		if len(p.config.Generate.Handlers) > 0 {
			return "popcornwave.toml", true, nil
		}
		return "", false, nil
	}
	return "", false, fmt.Errorf("unknown capability %q", name)
}

// configSectionEvidence finds the first environment file carrying a section.
func (p projectState) configSectionEvidence(section string) (string, bool, error) {
	contents, err := p.readConfigFiles()
	if err != nil {
		return "", false, err
	}
	for _, name := range p.configFiles {
		if containsSection(contents[name], section) {
			return name, true, nil
		}
	}
	return "", false, nil
}

// containsSection reports whether a TOML document declares a section, ignoring
// the commented-out form api:cli-init writes for an unimplemented mode.
func containsSection(document, section string) bool {
	for line := range strings.Lines(document) {
		if strings.TrimSpace(line) == section {
			return true
		}
	}
	return false
}

// carriedCapabilities lists what the project already has, in catalog order.
// api:cli-doctor reports the same probes api:cli-add offers from, so a gap the
// report names can be answered with pw add rather than by hand.
func (p projectState) carriedCapabilities() ([]string, error) {
	var carried []string
	for _, name := range capabilityOrder {
		_, present, err := p.carries(name)
		if err != nil {
			return nil, err
		}
		if present {
			carried = append(carried, name)
		}
	}
	return carried, nil
}

// scaffoldOptionsOf reconstructs the pw init answers a project carries, so a
// scaffold this command writes describes the project it is writing into rather
// than a default one. The capabilities come from the probes rather than from a
// manifest, for the same reason detection does: a manifest would disagree with
// a project someone edited by hand.
func scaffoldOptionsOf(state projectState) (initOptions, error) {
	carried, err := state.carriedCapabilities()
	if err != nil {
		return initOptions{}, err
	}
	options := initOptions{
		Name:   state.config.Name,
		TinyGo: state.config.Toolchain != toolchainGo,
		Engine: state.config.Database,
		Auth:   authNone,
	}
	for _, capability := range carried {
		switch capability {
		case capabilityDevbox:
			options.Devbox = true
		case capabilityDatabase:
			options.Database = true
		case capabilityRedis:
			options.Redis = true
		case capabilityTailwind:
			options.Tailwind = true
		case capabilityImages:
			options.Images = true
		case capabilityDynamo:
			options.Dynamo = true
		case capabilityAuth:
			// The probe reports that a login is served, not which mode serves
			// it. The starter page only asks the first question.
			options.Auth = authOIDC
		case capabilityRegistered:
			options.Router = withRouter(options.Router, routerRegistered)
		case capabilityDiscovered:
			options.Router = withRouter(options.Router, routerDiscovered)
		}
	}
	return options, nil
}

// withRouter adds a tree to a router answer. Both trees present is the one
// answer that names neither of them.
func withRouter(current, added string) string {
	if current == "" || current == added {
		return added
	}
	return routerBoth
}

// missingCapabilities lists what the project can still take, in catalog order.
func (p projectState) missingCapabilities() ([]string, error) {
	var missing []string
	for _, name := range capabilityOrder {
		_, present, err := p.carries(name)
		if err != nil {
			return nil, err
		}
		if !present {
			missing = append(missing, name)
		}
	}
	return missing, nil
}
