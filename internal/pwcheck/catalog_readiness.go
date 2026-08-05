package pwcheck

// PW05xx: the pre-launch checklist as something that runs, so the list cannot
// go stale the way a documentation page does. Readiness here means what the
// framework configured; it says nothing about capacity, backups, or
// infrastructure. A condition another group owns is cited there rather than
// restated here, so one setting still produces one finding.
const (
	APIDocExposed      = "PW0501"
	TailwindMinifyOff  = "PW0502"
	StylesheetStale    = "PW0503"
	PrecompressionOld  = "PW0504"
	ImageToolMissing   = "PW0505"
	UnrevocableSession = "PW0506"
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
			Title:    "the built asset tree is older than what it was built from",
			Severity: Warning, DevSeverity: Note, Scope: Deployed,
			Inputs: ProjectFiles, Phase: Doctor,
			Remedy: "run pw build, which rebuilds dist/public and its manifest",
		},
		Check{
			ID: ImageToolMissing, Group: GroupReadiness,
			// A missing encoder costs bytes rather than correctness: the
			// conversion declines and the authored image is served as it was
			// written, so the build succeeds and the page works. That is a
			// warning about a lost optimization, not a broken deployment.
			Title:    "image conversion is enabled and no encoder is installed",
			Severity: Warning, DevSeverity: Note, Scope: Deployed,
			Inputs: ProjectFiles, Phase: Doctor,
			Remedy: "run pw add images, or install the encoders it pins",
		},
		Check{
			ID: UnrevocableSession, Group: GroupReadiness,
			// The cookie backend cannot take back a record it already wrote, so
			// a login kept there outlives the logout that was meant to end it.
			// That is the right trade in development, where the point of the
			// backend is a login needing no infrastructure, and Deployed scope
			// is what keeps this silent there.
			Title:    "the login session is stored where it cannot be revoked",
			Severity: Warning, DevSeverity: Note, Scope: Deployed,
			Inputs: Config, Phase: Doctor,
			Remedy: "set session.backend to rdb, redis, or dynamo where a logout must end a session",
		},
	)
}
