package pwcli

import (
	tea "github.com/charmbracelet/bubbletea"
)

// initWizardSteps builds the question list, seeding every answer from the
// shortcut flags that were already supplied on the command line.
func initWizardSteps(defaults initOptions) []wizardStep[initOptions] {
	return []wizardStep[initOptions]{
		newTextStep(
			"Project name",
			"Creates ./<name> holding a Go module of the same name.",
			defaults.Name,
			"myapp",
			validateProjectName,
			func(target *initOptions, value string) { target.Name = value },
		),
		newChoiceStep(
			"TinyGo support",
			"TinyGo produces much smaller binaries and has the more complete wasm target. "+
				"pw add cannot change this later; switching is a manual edit, see the pw init docs.",
			yesNoCursor(defaults.TinyGo),
			wizardChoice[initOptions]{
				name:        "Yes",
				description: "pw.ServeMux routing plus the TinyGo toolchain in devbox.json",
				apply:       func(target *initOptions) { target.TinyGo = true },
			},
			wizardChoice[initOptions]{
				name:        "No",
				description: "net/http.ServeMux routing, host Go toolchain only",
				apply:       func(target *initOptions) { target.TinyGo = false },
			},
		),
		newChoiceStep(
			"Tailwind CSS",
			"Wires the pinned Tailwind toolchain into the project and generates public/generated/app.css.",
			yesNoCursor(defaults.Tailwind),
			wizardChoice[initOptions]{
				name:        "Yes",
				description: "assets/app.css entry point and the Tailwind build step",
				apply:       func(target *initOptions) { target.Tailwind = true },
			},
			wizardChoice[initOptions]{
				name:        "No",
				description: "plain CSS owned by the application; pw add tailwind enables it later",
				apply:       func(target *initOptions) { target.Tailwind = false },
			},
		),
		newChoiceStep(
			"Database",
			"Adds the rdb configuration, the migration directory, and a typed SQL example. Authentication needs it.",
			yesNoCursor(defaults.Database),
			wizardChoice[initOptions]{
				name:        "Yes",
				description: "[middleware.rdb], migrations/00001_init.sql, and a typed SQL example",
				apply:       setDatabase(true),
			},
			wizardChoice[initOptions]{
				name:        "No",
				description: "no database, no SQL example, and no migrations; pw add database enables it later",
				apply:       setDatabase(false),
			},
		),
		when(func(options initOptions) bool { return options.Database },
			newChoiceStep(
				"Database engine",
				"Decides the DSN, the dialect of the starter schema and migrations, and the development server. "+
					"Changing it later means rewriting both, so pw add cannot do it for you.",
				engineCursor(defaults.Engine),
				engineChoices()...,
			),
		),
		when(func(options initOptions) bool { return options.Database },
			newChoiceStep(
				"Authentication",
				"Selects the login model. The framework writes the [auth] configuration; handlers stay yours.",
				authCursor(defaults.Auth),
				wizardChoice[initOptions]{
					name:        "None",
					description: "no authentication configuration; pw add auth enables it later",
					apply:       setAuth(authNone),
				},
				wizardChoice[initOptions]{
					name:        "OIDC",
					description: "log in against an OpenID Provider",
					apply:       setAuth(authOIDC),
				},
				wizardChoice[initOptions]{
					name:        "Passkey only",
					description: "not implemented yet; the choice is recorded, disabled",
					apply:       setAuth(authPasskey),
				},
			),
		),
		when(func(options initOptions) bool { return usesOIDC(options.Auth) },
			newChoiceStep(
				"OIDC provider",
				"The local emulator signs you in by picking a user from a list, so login works before a real IdP exists.",
				yesNoCursor(defaults.AuthEmulator),
				wizardChoice[initOptions]{
					name:        "Local emulator",
					description: "pw dev runs it and injects the issuer and client credentials",
					apply:       func(target *initOptions) { target.AuthEmulator = true },
				},
				wizardChoice[initOptions]{
					name:        "External provider",
					description: "fill in auth.oidc yourself, or supply it through the environment",
					apply:       func(target *initOptions) { target.AuthEmulator = false },
				},
			),
		),
		newChoiceStep(
			"Devbox environment",
			"How this machine gets the toolchain and the services. Only the answer changes; the project is the same either way.",
			yesNoCursor(defaults.Devbox),
			wizardChoice[initOptions]{
				name:        "Yes",
				description: "devbox.json pins the versions, and pw dev starts the services it declares",
				apply:       func(target *initOptions) { target.Devbox = true },
			},
			wizardChoice[initOptions]{
				name:        "No",
				description: "keep your own setup — mise, Docker Compose, Nix, Homebrew, Scoop; pw add devbox enables it later",
				apply:       setDevbox(false),
			},
		),
		when(func(options initOptions) bool { return options.Devbox },
			newChoiceStep(
				"Redis or Valkey",
				"Adds the Valkey development server to devbox.json for session, token, and counter state.",
				yesNoCursor(defaults.Redis),
				wizardChoice[initOptions]{
					name:        "Yes",
					description: "pw dev starts Valkey beside the application",
					apply:       func(target *initOptions) { target.Redis = true },
				},
				wizardChoice[initOptions]{
					name:        "No",
					description: "a smaller development environment; pw add redis-valkey enables it later",
					apply:       func(target *initOptions) { target.Redis = false },
				},
			),
		),
	}
}

// engineChoices renders the engine table as wizard choices, in catalog order,
// so adding an engine costs one table entry rather than a step edit as well.
func engineChoices() []wizardChoice[initOptions] {
	choices := make([]wizardChoice[initOptions], 0, len(engineOrder))
	for _, name := range engineOrder {
		engine := databaseEngines[name]
		choices = append(choices, wizardChoice[initOptions]{
			name:        engine.Label,
			description: engine.Summary,
			apply:       setEngine(name),
		})
	}
	return choices
}

// setEngine records the engine answer.
func setEngine(name string) func(*initOptions) {
	return func(target *initOptions) { target.Engine = name }
}

// setDevbox records the answer and clears what depends on it. The Valkey
// question only ever writes a Devbox package, so a declined environment takes
// that step out of the wizard rather than leaving an answer nothing applies.
func setDevbox(enabled bool) func(*initOptions) {
	return func(target *initOptions) {
		target.Devbox = enabled
		if !enabled {
			target.Redis = false
		}
	}
}

// setDatabase records the answer and clears what depends on it. A declined
// database takes the authentication step out of the wizard, so an answer
// seeded by --auth must not survive as an unreachable one.
func setDatabase(enabled bool) func(*initOptions) {
	return func(target *initOptions) {
		target.Database = enabled
		if !enabled {
			// The engine step goes with it. Its answer applies only inside a
			// project that has a database, so leaving one behind would be an
			// answer to a question this project never reached.
			target.Engine = engineSQLite
			target.Auth = authNone
			target.AuthEmulator = false
		}
	}
}

// setAuth records the mode and clears the emulator answer for a mode that has
// no provider, so a stray --devidp flag cannot survive the choice.
func setAuth(mode string) func(*initOptions) {
	return func(target *initOptions) {
		target.Auth = mode
		if !usesOIDC(mode) {
			target.AuthEmulator = false
		}
	}
}

// authCursor maps an authentication mode onto its position in the choice list.
func authCursor(mode string) int {
	switch mode {
	case authOIDC:
		return 1
	case authPasskey:
		return 2
	default:
		return 0
	}
}

// yesNoCursor maps a boolean default onto a leading yes, trailing no choice list.
func yesNoCursor(enabled bool) int {
	if enabled {
		return 0
	}
	return 1
}

// runInitWizard asks every question and returns the confirmed options.
func runInitWizard(defaults initOptions, programOptions ...tea.ProgramOption) (initOptions, error) {
	return runWizard(newInitWizard(defaults), programOptions...)
}

// newInitWizard builds the model. Unlike api:cli-add and api:cli-new, init
// reviews answers rather than files: nothing it writes can collide with work
// that already exists.
func newInitWizard(defaults initOptions) wizardModel[initOptions] {
	return wizardModel[initOptions]{
		steps:    initWizardSteps(defaults),
		defaults: defaults,
		title:    "Popcorn Wave  new project",
		confirm:  "create",
		theme:    newWizardTheme(),
	}
}
