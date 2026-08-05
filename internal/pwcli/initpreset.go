package pwcli

import (
	"errors"
	"fmt"
	"strings"
)

// The preset names --preset accepts. They are the vocabulary of
// requirement:init-presets, and the wizard, the flag, and the documentation all
// use them.
const (
	presetWebsiteLogin      = "website-login"
	presetWebsiteAWS        = "website-aws"
	presetWebsiteDiscovered = "website-discovered"
	presetWebsiteRegistered = "website-registered"
	presetAPIServer         = "api-server"
	presetPackage           = "package"
	presetManual            = "manual"
)

// initPreset is one named answer set. A preset answers the capability
// questions so that a project name is the only one left, which is the whole
// point of decision:preset-first-bootstrap: the questions stay, and a preset
// supplies answers to them rather than replacing them.
type initPreset struct {
	name string
	// label and summary are what the first wizard screen shows. The summary
	// names the answers that distinguish this preset from its neighbours
	// rather than every answer it gives, because a list where every line says
	// the same six things distinguishes nothing.
	label   string
	summary string
	// kind is the project.kind this preset produces. A preset that sets one
	// removes questions rather than answering them, since the questions
	// describe an application and its project is not one.
	kind string
	// apply writes the answers over the defaults. It is nil for manual, which
	// answers nothing and opens the hub on the defaults instead.
	apply func(*initOptions)
}

// initPresetCatalog is the list, in the order the wizard shows it: the project
// shapes first, and Manual last.
var initPresetCatalog = []initPreset{
	{
		name:    presetWebsiteLogin,
		label:   "Web site with login",
		summary: "pages, OIDC login, Redis sessions, SQLite, Tailwind",
		apply: func(target *initOptions) {
			target.Router = routerDiscovered
			target.Tailwind = true
			target.Auth = authOIDC
			// The local emulator, so a login works before a real provider
			// exists. A preset that scaffolded an external provider would
			// produce a project whose first run cannot sign anyone in.
			target.AuthEmulator = true
			target.AuthStore = engineSQLite
			target.Database = true
			target.Engine = engineSQLite
			target.Session = sessionRedis
			target.Redis = true
		},
	},
	{
		name:    presetWebsiteAWS,
		label:   "Web site on AWS",
		summary: "pages, OIDC login, DynamoDB, Tailwind",
		apply: func(target *initOptions) {
			target.Router = routerDiscovered
			target.Tailwind = true
			target.Auth = authOIDC
			target.AuthEmulator = true
			// Every store this project has is DynamoDB. The four tables
			// plugin/auth owns moved onto it, so the login no longer drags a
			// SQL engine behind it.
			target.AuthStore = dynamoStore
			target.Dynamo = true
			target.Database = false
			target.Session = sessionDynamo
			target.Redis = false
		},
	},
	{
		name:    presetWebsiteDiscovered,
		label:   "Simple website",
		summary: "pages, no login, no database",
		apply: func(target *initOptions) {
			target.Router = routerDiscovered
			simpleWebsite(target)
		},
	},
	{
		name:    presetWebsiteRegistered,
		label:   "Simple website, handlers",
		summary: "handlers, no login, no database",
		apply: func(target *initOptions) {
			target.Router = routerRegistered
			simpleWebsite(target)
		},
	},
	{
		name:    presetAPIServer,
		label:   "API Server",
		summary: "handlers, JWT verification, no browser login",
		apply: func(target *initOptions) {
			target.Router = routerRegistered
			// The mode the authentication question deliberately does not
			// offer. Reaching it by naming the project shape it belongs to is
			// the whole of decision:jwt-only-preset-scaffolding.
			target.Auth = authJWTOnly
			target.AuthEmulator = false
			// An API serves no CSS and renders no page, so neither the
			// stylesheet toolchain nor a store is scaffolded. The database
			// arrives with the registered admission mode or with revocation,
			// which are the two answers this scaffold does not take.
			target.Tailwind = false
			target.Database = false
			target.Dynamo = false
			target.Session = sessionCookie
			target.Redis = false
		},
	},
	{
		name:    presetPackage,
		label:   "Package",
		summary: "a publishable module, no application of its own",
		kind:    kindPackage,
		apply: func(target *initOptions) {
			// A package compiles into somebody else's binary, so every answer
			// below describes a project this one is not. They are cleared
			// rather than left at their defaults, because a default that
			// reaches the scaffold writes a file the package must not carry.
			target.Router = ""
			target.Tailwind = false
			target.Auth = authNone
			target.AuthEmulator = false
			target.AuthStore = ""
			target.Database = false
			target.Dynamo = false
			target.Session = ""
			target.Redis = false
			target.Devbox = false
			target.Images = false
		},
	},
	{
		name:    presetManual,
		label:   "Manual",
		summary: "choose every answer",
	},
}

// simpleWebsite is the answer set the two simple presets share. They differ by
// the router and by nothing else, because decision:page-router-scaffold-choice
// is the one bootstrap answer with no wrong choice and the list names both
// rather than picking for the reader.
func simpleWebsite(target *initOptions) {
	// Every preset that produces a website styles it. What distinguishes these
	// two from the ones above is the login and the store, and a starter page
	// that arrives unstyled would be a fourth difference nobody asked for.
	target.Tailwind = true
	target.Auth = authNone
	target.AuthEmulator = false
	target.Database = false
	target.Dynamo = false
	target.Session = sessionCookie
	target.Redis = false
}

// presetChoices renders the catalog as wizard choices, in catalog order, so
// adding a preset costs one table entry rather than a step edit as well.
func presetChoices() []wizardChoice[initOptions] {
	choices := make([]wizardChoice[initOptions], 0, len(initPresetCatalog))
	for _, preset := range initPresetCatalog {
		choices = append(choices, wizardChoice[initOptions]{
			name:        preset.label,
			description: preset.summary,
			apply:       applyPresetChoice(preset),
		})
	}
	return choices
}

// applyPresetChoice records the preset and its answers. Every question below
// reads them as its preselection, which is what makes the review a hub seeded
// by the preset rather than a screen that hides what it chose.
func applyPresetChoice(preset initPreset) func(*initOptions) {
	return func(target *initOptions) {
		target.Preset = preset.name
		if preset.apply == nil {
			// Manual answers nothing, so the questions below keep whatever
			// they already carried: the defaults on a fresh run, and the
			// answers already given on a return visit.
			target.Kind = ""
			return
		}
		name := target.Name
		*target = applyPreset(preset, *target)
		target.Name = name
		target.Preset = preset.name
	}
}

// presetCursor preselects the preset a seeded answer already names, defaulting
// to the first entry of the catalog.
func presetCursor(name string) int {
	for index, preset := range initPresetCatalog {
		if preset.name == name {
			return index
		}
	}
	return 0
}

// validateModulePath checks the path a package publishes under. It is a module
// path rather than a directory name, so it is checked as one, and the
// destination it creates is its last element.
func validateModulePath(path string) error {
	if path == "" {
		return errors.New("a module path is required, such as github.com/you/mycomponent")
	}
	if strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return errors.New("a module path has no leading or trailing slash")
	}
	if strings.ContainsAny(path, " \t\\") {
		return errors.New("a module path has no spaces or backslashes")
	}
	for _, element := range strings.Split(path, "/") {
		if element == "" {
			return errors.New("a module path has no empty element")
		}
	}
	if !strings.Contains(path, "/") {
		// A single element is a valid module path to the Go tool and is not a
		// publishable one, which is the only kind this project is for.
		return errors.New("a published module path names a host and a repository, such as github.com/you/mycomponent")
	}
	_, err := initDestination(moduleDirectory(path))
	return err
}

// moduleDirectory is the directory a module path is created in: its last
// element, which is what a checkout of that repository is called.
func moduleDirectory(path string) string {
	if index := strings.LastIndex(path, "/"); index >= 0 {
		return path[index+1:]
	}
	return path
}

// lookupPreset finds a preset by the name --preset was given.
func lookupPreset(name string) (initPreset, bool) {
	for _, preset := range initPresetCatalog {
		if preset.name == name {
			return preset, true
		}
	}
	return initPreset{}, false
}

// presetNames lists the accepted --preset values for an error message.
func presetNames() string {
	names := make([]string, 0, len(initPresetCatalog))
	for _, preset := range initPresetCatalog {
		names = append(names, preset.name)
	}
	return strings.Join(names, ", ")
}

// applyPreset writes a preset's answers over the options a flag run started
// from. The project name survives, because it is the one question a preset
// still asks and a positional argument has already answered it.
func applyPreset(preset initPreset, options initOptions) initOptions {
	name := options.Name
	yes := options.Yes
	applied := defaultInitOptions()
	applied.Name = name
	applied.Yes = yes
	applied.Kind = preset.kind
	if preset.apply != nil {
		preset.apply(&applied)
	}
	return applied
}

// presetConflict names a capability flag given beside --preset. Both answer the
// same questions and neither is obviously the winner, so the run stops before
// anything is written rather than silently letting one of them lose.
func presetConflict(args []string) string {
	for _, arg := range args {
		name, _, _ := strings.Cut(arg, "=")
		switch name {
		case "--tailwind", "--no-tailwind", "--tinygo", "--no-tinygo",
			"--devbox", "--no-devbox", "--database", "--no-database",
			"--dynamo", "--no-dynamo", "--redis", "--no-redis",
			"--devidp", "--no-devidp", "--session", "--db", "--router", "--auth":
			return name
		}
	}
	return ""
}

// presetConflictError is the message a conflicting run stops with.
func presetConflictError(flag string) error {
	return fmt.Errorf("init: --preset answers the question %s answers; drop one of them", flag)
}

// parsePresetArgs reads --preset and --kind and settles what they mean
// together, before any other flag is looked at.
//
// --kind is the spelling that names a project kind directly, and --preset is
// the one that names it through the list a reader is already looking at.
// --kind=package and --preset=package are the same run, so they agree rather
// than conflict; any other pairing is a contradiction stated in one command.
func parsePresetArgs(args []string) (initPreset, bool, error) {
	var (
		preset      initPreset
		presetGiven bool
		kind        string
		kindGiven   bool
	)
	for _, arg := range args {
		if name, ok := strings.CutPrefix(arg, "--preset="); ok {
			found, exists := lookupPreset(name)
			if !exists {
				return initPreset{}, false, fmt.Errorf("init: --preset must be one of %s", presetNames())
			}
			preset, presetGiven = found, true
			continue
		}
		if value, ok := strings.CutPrefix(arg, "--kind="); ok {
			if value != kindApplication && value != kindPackage {
				return initPreset{}, false, fmt.Errorf("init: --kind must be %q or %q", kindApplication, kindPackage)
			}
			kind, kindGiven = value, true
		}
	}
	if presetGiven {
		if kindGiven && kind != presetKind(preset) {
			return initPreset{}, false, fmt.Errorf("init: --preset=%s creates a %s project, so --kind=%s contradicts it",
				preset.name, presetKind(preset), kind)
		}
		if conflict := presetConflict(args); conflict != "" {
			return initPreset{}, false, presetConflictError(conflict)
		}
		return preset, true, nil
	}
	if kindGiven && kind == kindPackage {
		// The same scaffold the package preset reaches, named the other way.
		// A caller who knows the flag gets the project without having to know
		// that the list is where it now lives.
		found, _ := lookupPreset(presetPackage)
		return found, true, nil
	}
	return initPreset{}, false, nil
}

// presetKind names the project kind a preset produces, reading an empty kind
// as the application every other preset creates.
func presetKind(preset initPreset) string {
	if preset.kind == "" {
		return kindApplication
	}
	return preset.kind
}

// applyPresetArgs applies a preset to a flag run and validates what it
// produced. Manual answers nothing, so it leaves the defaults for the hub to
// edit rather than writing a project shape of its own.
func applyPresetArgs(preset initPreset, options initOptions) (initOptions, error) {
	if preset.name == presetManual {
		options.Preset = presetManual
		return options, nil
	}
	applied := applyPreset(preset, options)
	applied.Preset = preset.name
	return normalizeSession(applied), nil
}
