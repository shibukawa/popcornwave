package pwcheck

// PW05xx: the pre-launch checklist as something that runs, so the list cannot
// go stale the way a documentation page does. Readiness here means what the
// framework configured; it says nothing about capacity, backups, or
// infrastructure. A condition another group owns is cited there rather than
// restated here, so one setting still produces one finding.
const (
	APIDocExposed     = "PW0501"
	TailwindMinifyOff = "PW0502"
	StylesheetStale   = "PW0503"
	PrecompressionOld = "PW0504"
)

func init() {
	register(
		Check{
			ID: APIDocExposed, Group: GroupReadiness,
			Title:    "the API documentation endpoint is exposed",
			Severity: Warning, DevSeverity: Note, Scope: Deployed,
			Inputs: Config, Phase: Doctor,
			Remedy: "clear server.api_doc unless the exposure is intended",
		},
		Check{
			ID: TailwindMinifyOff, Group: GroupReadiness,
			Title:    "Tailwind output is not minified",
			Severity: Warning, DevSeverity: Note, Scope: Deployed,
			Inputs: ProjectFiles, Phase: Doctor,
			Remedy: "set assets.tailwind.minify",
		},
		Check{
			ID: StylesheetStale, Group: GroupReadiness,
			Title:    "the generated stylesheet is older than its sources",
			Severity: Error, DevSeverity: Warning, Scope: Deployed,
			Inputs: ProjectFiles, Phase: Doctor,
			Remedy: "run pw build, which rebuilds the stylesheet",
		},
		Check{
			ID: PrecompressionOld, Group: GroupReadiness,
			Title:    "a public asset is newer than its compressed sidecar",
			Severity: Warning, DevSeverity: Note, Scope: Deployed,
			Inputs: ProjectFiles, Phase: Doctor,
			Remedy: "run pw build, which writes the sidecars",
		},
	)
}
