package pwcheck

// PW01xx: whether the project's declared shape, its toolchain, and its
// generated artifacts still agree with each other.
const (
	MainPackageMissing     = "PW0101"
	MigrationDirMissing    = "PW0102"
	OrphanGeneratedFile    = "PW0110"
	GeneratedOlderThanSrc  = "PW0111"
	GeneratedNotIgnored    = "PW0113"
	GoVersionMismatch      = "PW0120"
	TinyGoBaselineUnmet    = "PW0121"
	OutsideDevboxShell     = "PW0122"
	DeclaredServiceMissing = "PW0123"
	TailwindToolchain      = "PW0124"
	PortUnavailable        = "PW0125"
)

func init() {
	register(
		Check{
			ID: MainPackageMissing, Group: GroupProject,
			Title:    "project.main does not name a main package",
			Severity: Error, DevSeverity: Error, Scope: Every,
			Inputs: ProjectFiles, Phase: Doctor,
			Remedy: "point project.main at the directory holding func main, or scaffold one with pw new",
		},
		Check{
			ID: MigrationDirMissing, Group: GroupProject,
			Title:    "migration directory is missing",
			Severity: Error, DevSeverity: Error, Scope: Every,
			Inputs: ProjectFiles | Config, Phase: Doctor,
			Remedy: "run pw add database, which writes the directory and its starter schema",
		},
		Check{
			ID: OrphanGeneratedFile, Group: GroupProject,
			Title:    "generated file outlived its source",
			Severity: Error, DevSeverity: Error, Scope: Every,
			Inputs: ProjectFiles, Phase: Doctor,
			// The orphan compiles and its registrations still run, so a deleted
			// page keeps serving and a deleted query keeps building. Nothing
			// else in the toolchain reports it.
			Remedy: "delete the generated file",
		},
		Check{
			ID: GeneratedOlderThanSrc, Group: GroupProject,
			Title:    "generated file is older than its source",
			Severity: Warning, DevSeverity: Warning, Scope: Every,
			Inputs: ProjectFiles, Phase: Doctor,
			Remedy: "run pw generate",
		},
		Check{
			ID: GeneratedNotIgnored, Group: GroupProject,
			Title:    "generated sources are neither committed nor ignored",
			Severity: Note, DevSeverity: Note, Scope: Every,
			Inputs: ProjectFiles, Phase: Doctor,
			Remedy: "commit *_pw_gen.go or add it to .gitignore, so the project says once which it is",
		},
		Check{
			ID: GoVersionMismatch, Group: GroupProject,
			Title:    "devbox Go version disagrees with the go.mod directive",
			Severity: Warning, DevSeverity: Warning, Scope: Every,
			Inputs: ProjectFiles, Phase: Doctor,
			Remedy: "align devbox.json with the go directive, so the shell build and the CI build are one build",
		},
		Check{
			ID: TinyGoBaselineUnmet, Group: GroupProject,
			Title:    "pinned TinyGo is below the supported baseline",
			Severity: Error, DevSeverity: Error, Scope: Every,
			Inputs: ProjectFiles, Phase: Doctor,
			Remedy: "pin tinygo 0.42 or newer in devbox.json",
		},
		Check{
			ID: OutsideDevboxShell, Group: GroupProject,
			Title:    "running outside the devbox shell",
			Severity: Note, DevSeverity: Note, Scope: Every,
			Inputs: ProjectFiles | ProcessEnv, Phase: Doctor,
			Remedy: "run devbox shell, so the tools pw dev expects are on PATH",
		},
		Check{
			ID: DeclaredServiceMissing, Group: GroupProject,
			Title:    "configuration selects a service devbox does not declare",
			Severity: Warning, DevSeverity: Warning, Scope: Every,
			Inputs: ProjectFiles | Config, Phase: Doctor,
			Remedy: "run pw add redis-valkey",
		},
		Check{
			ID: TailwindToolchain, Group: GroupProject,
			Title:    "Tailwind is enabled without its toolchain or entry point",
			Severity: Error, DevSeverity: Error, Scope: Every,
			Inputs: ProjectFiles, Phase: Doctor,
			Remedy: "run pw add tailwind, or create the configured CSS entry point",
		},
		Check{
			ID: PortUnavailable, Group: GroupProject,
			Title:    "the configured port is already in use",
			Severity: Warning, DevSeverity: Warning, Scope: Development,
			Inputs: Config, Phase: Doctor,
			Remedy: "stop the process holding it, or set server.port",
		},
	)
}
