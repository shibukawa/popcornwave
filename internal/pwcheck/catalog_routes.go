package pwcheck

// PW02xx: what the route table says about paths that collide and paths nothing
// serves.
//
// These declare the RouteTable input, which api:cli-generate writes as
// data:route-table. A runner that cannot build an input skips the checks that
// need it and reports the gap, so a project that has not generated is told the
// routes were not examined instead of being shown a clean route section
// nothing backs up.
const (
	DuplicateRoutePattern  = "PW0201"
	FrameworkMountClash    = "PW0202"
	UnresolvedRegistration = "PW0203"
)

func init() {
	register(
		Check{
			ID: DuplicateRoutePattern, Group: GroupRoutes,
			Title:    "two registrations share one route pattern",
			Severity: Error, DevSeverity: Error, Scope: Every,
			Inputs: RouteTable, Phase: Doctor,
			// net/http panics at registration, so this is a startup crash that
			// can be found without starting.
			Remedy: "remove one registration",
		},
		Check{
			ID: UnresolvedRegistration, Group: GroupRoutes,
			Title:    "a route registration the analysis could not read a pattern from",
			Severity: Note, DevSeverity: Note, Scope: Every,
			Inputs: RouteTable, Phase: Doctor,
			// The route runs; what is missing is knowing about it. Every other
			// route check is silent about that path, and the OpenAPI document
			// omits it, so the note is what keeps a clean report honest.
			Remedy: "register a literal method-and-path pattern, per rule:static-route-discovery",
		},
		Check{
			ID: FrameworkMountClash, Group: GroupRoutes,
			Title:    "an application route collides with an enabled framework mount",
			Severity: Error, DevSeverity: Error, Scope: Every,
			Inputs: RouteTable | Config, Phase: Doctor,
			Remedy: "move the route, or disable the mount with its configuration key",
		},
	)
}
