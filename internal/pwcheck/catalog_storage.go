package pwcheck

// PW03xx: whether the migration sources are well-formed, and, when contacting
// the database is permitted, whether the database still matches them.
const (
	DuplicateMigration  = "PW0301"
	MigrationVersionGap = "PW0302"
	DatabasePathUnwrite = "PW0304"
	AppliedStateUnknown = "PW0310"
	ConnectionFailed    = "PW0311"
	PendingMigrations   = "PW0312"
)

func init() {
	register(
		Check{
			ID: DuplicateMigration, Group: GroupStorage,
			Title:    "two migrations share one version",
			Severity: Error, DevSeverity: Error, Scope: Every,
			Inputs: ProjectFiles, Phase: Doctor,
			Remedy: "renumber the newer file; never renumber one already applied",
		},
		Check{
			ID: MigrationVersionGap, Group: GroupStorage,
			Title:    "the migration version sequence has a gap",
			Severity: Note, DevSeverity: Note, Scope: Every,
			Inputs: ProjectFiles, Phase: Doctor,
			// goose applies lexically and tolerates gaps, so this is a merge
			// artifact worth seeing rather than a fault.
			Remedy: "no action required",
		},
		Check{
			ID: DatabasePathUnwrite, Group: GroupStorage,
			Title:    "the local database path is not writable",
			Severity: Error, DevSeverity: Error, Scope: Every,
			Inputs: Config | ProjectFiles, Phase: Doctor,
			Remedy: "create the parent directory, or point the DSN elsewhere",
		},
		Check{
			ID: AppliedStateUnknown, Group: GroupStorage,
			Title:    "the applied migration state was not read",
			Severity: Note, DevSeverity: Note, Scope: Every,
			Inputs: Config, Phase: Doctor,
			Remedy: "run pw doctor --online, or pw migrate status",
		},
		Check{
			ID: ConnectionFailed, Group: GroupStorage,
			Title:    "a configured database connection was refused",
			Severity: Error, DevSeverity: Error, Scope: Every,
			Inputs: Config | Network, Phase: Doctor,
			Remedy: "check the DSN, the server, and the network path",
		},
		Check{
			ID: PendingMigrations, Group: GroupStorage,
			Title:    "the database is behind the migration sources",
			Severity: Warning, DevSeverity: Warning, Scope: Every,
			Inputs: Config | Network, Phase: Doctor,
			Remedy: "run pw migrate up",
		},
	)
}
