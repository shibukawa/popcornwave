package pwcli

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shibukawa/popcornwave/internal/pwcheck"
	"github.com/shibukawa/popcornwave/internal/pwenv"
	"github.com/shibukawa/popcornwave/internal/pwmigrate"
)

// checkContext is everything one environment's checks may read. A check that
// needs an input this run could not build is skipped and reported as a limit,
// never guessed at.
type checkContext struct {
	Env     string
	Root    string
	Config  environmentConfig
	Graph   importGraph
	State   projectState
	Scan    *projectScan
	Online  bool
	HostEnv string
}

type checkRun struct {
	checkContext
	findings []doctorFinding
	// unresolved collects what the checks could not attribute, so the report
	// states it rather than reporting a gap it cannot back up.
	unresolved []string
}

// report records a finding, resolving the severity from the diagnosed token. A
// check whose scope excludes this environment is silently skipped, which is how
// one catalog serves dev and prod without a second severity system.
func (r *checkRun) report(id, message, evidence string) {
	check := pwcheck.MustLookup(id)
	if !check.AppliesTo(r.Env) {
		return
	}
	r.findings = append(r.findings, doctorFinding{
		Check:    check,
		Severity: check.SeverityFor(r.Env),
		Message:  message,
		Evidence: evidence,
		Remedy:   check.Remedy,
	})
}

// escalate records a finding at a severity the condition itself justifies,
// regardless of environment. It is used where one check covers a mild form and
// a severe one, such as a cookie that is merely insecure and one that is
// cross-site as well.
func (r *checkRun) escalate(id string, severity pwcheck.Severity, message, evidence string) {
	check := pwcheck.MustLookup(id)
	r.findings = append(r.findings, doctorFinding{
		Check: check, Severity: severity, Message: message, Evidence: evidence, Remedy: check.Remedy,
	})
}

func runChecks(ctx context.Context, context checkContext) ([]doctorFinding, []doctorLimit) {
	run := &checkRun{checkContext: context}
	run.checkProject()
	run.checkWiring()
	run.checkDependencies()
	run.checkSecrets()
	run.checkEnvironmentValues()
	run.checkIdentityProvider()
	run.checkStorage(ctx)
	run.checkReadiness()
	sortFindings(run.findings)
	var limits []doctorLimit
	if len(run.unresolved) > 0 {
		sortStrings(run.unresolved)
		limits = append(limits, doctorLimit{
			Subject: "configuration sections [" + strings.Join(run.unresolved, "], [") + "]",
			Reason:  "no binding this analysis knows declares them; a plugin outside the framework may own them",
			Effect:  "they were neither validated nor reported as unclaimed",
		})
	}
	return run.findings, limits
}

func (r *checkRun) checkProject() {
	if !r.Scan.mainExists || !r.Scan.mainIsPackage {
		r.report(pwcheck.MainPackageMissing,
			fmt.Sprintf("project.main %q is not a directory holding package main", r.State.config.Main),
			"popcornwave.toml project.main")
	}
	if r.databaseConfigured() && r.State.config.Migration.Dir != "" {
		if _, err := os.Stat(filepath.Join(r.Root, filepath.FromSlash(r.State.config.Migration.Dir))); err != nil {
			r.report(pwcheck.MigrationDirMissing,
				fmt.Sprintf("the database is configured but %s does not exist", r.State.config.Migration.Dir),
				"popcornwave.toml migration.dir")
		}
	}
	for _, generated := range r.Scan.generated {
		if generated.Orphan {
			r.report(pwcheck.OrphanGeneratedFile,
				"the source of "+generated.Path+" no longer exists, and the generated registrations still run",
				generated.Path)
			continue
		}
		if generated.Stale {
			r.report(pwcheck.GeneratedOlderThanSrc,
				generated.Path+" is older than "+generated.Source,
				generated.Path)
		}
	}
	if r.Scan.inGitTree && len(r.Scan.generated) > 0 && !r.Scan.ignores("*"+generatedSuffix) {
		untracked := 0
		for _, generated := range r.Scan.generated {
			if !r.Scan.tracked(generated.Path) {
				untracked++
			}
		}
		if untracked > 0 && untracked < len(r.Scan.generated) {
			r.report(pwcheck.GeneratedNotIgnored,
				fmt.Sprintf("%d of %d generated sources are neither committed nor ignored", untracked, len(r.Scan.generated)),
				".gitignore")
		}
	}
	if pinned, ok := r.Scan.devboxVersions["go"]; ok && r.Scan.goDirective != "" && pinned != "latest" {
		if !sameMinorVersion(pinned, r.Scan.goDirective) {
			r.report(pwcheck.GoVersionMismatch,
				"devbox pins go "+pinned+" while go.mod asks for "+r.Scan.goDirective,
				"devbox.json, go.mod")
		}
	}
	if r.State.config.Toolchain == toolchainTinyGo {
		if pinned, ok := r.Scan.devboxVersions["tinygo"]; ok && pinned != "latest" && compareVersions(pinned, tinyGoBaseline) < 0 {
			r.report(pwcheck.TinyGoBaselineUnmet,
				"devbox pins tinygo "+pinned+", below the "+tinyGoBaseline+" baseline",
				"devbox.json")
		}
		if !r.Scan.netdevRegistered {
			r.report(pwcheck.MissingNetdev,
				"no tinygohelper.go blank import of the netdev package; the binary would exit with \"Netdev not set\"",
				"tinygohelper.go")
		}
	}
	if r.State.devbox != "" && os.Getenv("DEVBOX_SHELL_ENABLED") == "" {
		r.report(pwcheck.OutsideDevboxShell,
			"the project declares a devbox environment and this shell is not it",
			"devbox.json")
	}
	if r.Config.raw("session.backend") == "redis" && r.State.devbox != "" && !strings.Contains(r.State.devbox, "valkey@") {
		r.report(pwcheck.DeclaredServiceMissing,
			"session.backend is redis and devbox declares no Valkey service",
			"devbox.json, session.backend")
	}
	if r.State.config.Tailwind.Enabled {
		reasons := []string{}
		if _, ok := r.Scan.devboxVersions["tailwindcss"]; !ok && r.State.devbox != "" {
			reasons = append(reasons, "devbox pins no tailwindcss")
		}
		if input := r.State.config.Tailwind.Input; input != "" {
			if _, err := os.Stat(filepath.Join(r.Root, filepath.FromSlash(input))); err != nil {
				reasons = append(reasons, input+" does not exist")
			}
		}
		if len(reasons) > 0 {
			r.report(pwcheck.TailwindToolchain, strings.Join(reasons, ", "), "popcornwave.toml assets.tailwind")
		}
	}
	if r.Env == pwenv.Development {
		if port := r.Config.raw("server.port"); port != "" {
			if listener, err := net.Listen("tcp", "127.0.0.1:"+port); err != nil {
				r.report(pwcheck.PortUnavailable, "port "+port+" is already bound on loopback", "server.port")
			} else {
				_ = listener.Close()
			}
		}
	}
}

// tinyGoBaseline is the oldest TinyGo the framework is verified against.
const tinyGoBaseline = "0.42"

func (r *checkRun) checkWiring() {
	if !r.Graph.available() {
		return
	}
	if r.Graph.linksPrefix(devIdPPackagePrefix) {
		r.report(pwcheck.DevIdPLinked,
			"the application imports the development identity provider, which authenticates nobody",
			devIdPPackagePrefix)
	}
	if r.Config.enabled("session.enabled") {
		backend := r.Config.raw("session.backend")
		switch pkg, known := sessionBackendPackages[backend]; {
		case known && !r.Graph.links(pkg):
			r.report(pwcheck.MissingSessionPlugin,
				"session.backend is "+backend+" and the application links no plugin registering it",
				"add: import _ \""+pkg+"\"")
		case !known && backend != "":
			r.report(pwcheck.MissingSessionPlugin,
				"session.backend is "+backend+", which no linked plugin registers",
				"session.backend")
		}
	}
	for _, connection := range r.Config.databaseDSNs() {
		scheme := connection.scheme()
		if scheme == "" {
			continue
		}
		pkg, known := driverPackages[scheme]
		switch {
		case !known:
			r.report(pwcheck.MissingSQLDriver,
				"no known driver package answers the "+scheme+" scheme of connection "+connection.Label,
				connection.Key)
		case scheme == "sqlite" && r.Graph.links(frameworkPackage):
			// pw links the sqlite driver itself, so a sqlite DSN is answered by
			// the framework import every application already has.
		case !r.Graph.links(pkg):
			r.report(pwcheck.MissingSQLDriver,
				"connection "+connection.Label+" uses the "+scheme+" scheme and the application links no driver for it",
				"add: import _ \""+pkg+"\"")
		}
	}
	for _, section := range r.Config.Sections {
		owner, known := configPrefixOwners[section]
		switch {
		case !known && !containsString(frameworkPrefixes, section):
			// A section this analysis cannot attribute may belong to a
			// third-party plugin, so it is a limit rather than a gap.
			r.unresolved = append(r.unresolved, section)
		case known && !r.Graph.links(owner):
			r.report(pwcheck.UnclaimedConfigKey,
				"the ["+section+"] section is configured and the application links no binding for it",
				"add: import _ \""+owner+"\"")
		}
	}
	if r.Graph.links(authPluginPackage) && !r.Config.enabled("auth.enabled") {
		r.report(pwcheck.UnusedLinkedPlugin,
			"the auth plugin is linked and auth.enabled is false",
			authPluginPackage)
	}
}

func (r *checkRun) checkDependencies() {
	authEnabled := r.Config.enabled("auth.enabled")
	sessionEnabled := r.Config.enabled("session.enabled")
	if authEnabled && !sessionEnabled {
		r.report(pwcheck.AuthWithoutSession,
			"auth.enabled is true and session.enabled is false",
			"session.enabled")
	}
	if sessionEnabled && r.Config.raw("session.backend") == "rdb" &&
		r.Config.raw("session.rdb.source") == "middleware" && !r.Config.enabled("middleware.rdb.enabled") {
		r.report(pwcheck.SessionRDBWithoutMW,
			"the session store reuses middleware.rdb and middleware.rdb.enabled is false",
			"middleware.rdb.enabled")
	}
	if !r.Config.enabled("middleware.rdb.enabled") {
		return
	}
	writable := map[string]bool{}
	groups := map[string]bool{}
	for index := 0; ; index++ {
		group, ok := r.Config.value(fmt.Sprintf("middleware.rdb.connections[%d].group", index))
		if !ok {
			if _, dsnPresent := r.Config.value(fmt.Sprintf("middleware.rdb.connections[%d].dsn", index)); !dsnPresent {
				break
			}
			group.Raw = r.Config.raw("middleware.rdb.default_group")
		}
		name := group.Raw
		if name == "" {
			name = "default"
		}
		groups[name] = true
		readonly, _ := strconv.ParseBool(r.Config.raw(fmt.Sprintf("middleware.rdb.connections[%d].readonly", index)))
		if !readonly {
			writable[name] = true
		}
	}
	if len(groups) == 0 {
		return
	}
	writeGroup := r.Config.raw("middleware.rdb.write_group")
	switch {
	case writeGroup != "" && !writable[writeGroup]:
		r.report(pwcheck.ReadonlyFrameworkDB,
			"middleware.rdb.write_group names "+writeGroup+", which holds no writable connection",
			"middleware.rdb.write_group")
	case writeGroup == "" && len(groups) > 1:
		r.report(pwcheck.ReadonlyFrameworkDB,
			"more than one connection group exists and middleware.rdb.write_group names none of them",
			"middleware.rdb.write_group")
	}
}

// placeholderSecrets are the values a scaffold or a hurried edit leaves behind.
// A scaffold value is published in the framework source, so it is a known
// credential rather than a weak one.
var placeholderSecrets = map[string]bool{
	"changeme": true, "change-me": true, "change_me": true, "secret": true,
	"password": true, "todo": true, "xxx": true, "replace-me": true, "example": true,
}

func (r *checkRun) checkSecrets() {
	fromFile := 0
	for _, key := range r.Config.secretKeys() {
		raw := r.Config.raw(key)
		if !carriesCredential(key, raw) {
			// The classification is by field name, so a DSN naming a local file
			// is secret-classified while carrying no credential to disclose.
			continue
		}
		if placeholderSecrets[strings.ToLower(strings.TrimSpace(raw))] {
			// The value is named only as a class, never printed.
			r.report(pwcheck.PlaceholderSecret, key+" still holds a placeholder value", key)
		}
		if !r.Config.fromFile(key) {
			continue
		}
		if strings.Contains(raw, "${") {
			// An expansion is a reference to the environment, not a literal.
			continue
		}
		fromFile++
		evidence := key + " in " + r.Config.ConfigPath
		if r.Scan.tracked(r.Config.ConfigPath) {
			evidence += " (tracked by git)"
		}
		r.report(pwcheck.LiteralSecretInFile, key+" is set from the configuration file", evidence)
	}
	if fromFile == 0 || r.Config.ConfigPath == "" {
		return
	}
	if r.Scan.inGitTree && r.Scan.tracked(r.Config.ConfigPath) {
		r.report(pwcheck.SecretFileNotIgnored,
			r.Config.ConfigPath+" holds a secret and is tracked by version control",
			r.Config.ConfigPath)
	}
	if mode, ok := r.Scan.configFileModes[r.Config.ConfigPath]; ok && mode&0o077 != 0 {
		r.report(pwcheck.SecretFilePerms,
			fmt.Sprintf("%s holds a secret and its mode is %04o", r.Config.ConfigPath, mode),
			r.Config.ConfigPath)
	}
}

func (r *checkRun) checkEnvironmentValues() {
	if r.Config.enabled("session.enabled") {
		secure, resolved := r.Config.boolValue("session.cookie.secure")
		sameSite := strings.ToLower(r.Config.raw("session.cookie.same_site"))
		switch {
		case resolved && !secure && sameSite == "none":
			r.escalate(pwcheck.InsecureSessionCookie, pwcheck.Error,
				"session.cookie.same_site is none without session.cookie.secure, which no browser accepts as a cross-site cookie",
				"session.cookie.secure")
		case resolved && !secure:
			r.report(pwcheck.InsecureSessionCookie,
				"session.cookie.secure is false, so the session cookie travels over plain http",
				"session.cookie.secure")
		}
	}
	if enabled, resolved := r.Config.boolValue("security.headers.enabled"); resolved && !enabled {
		r.report(pwcheck.ResponseHeadersWeak, "security.headers.enabled is false", "security.headers.enabled")
	}
	if r.Config.raw("observability.query.enabled") == "on" {
		r.report(pwcheck.QueryDiagnosticsOn,
			"observability.query.enabled is on, with slow_threshold "+r.Config.raw("observability.query.slow_threshold"),
			"observability.query.enabled")
	}
	if r.Config.raw("observability.query.bind_values") == "on" {
		r.report(pwcheck.BindValuesOn,
			"observability.query.bind_values is on, so application row data enters the SQL records",
			"observability.query.bind_values")
	}
	switch strings.ToLower(r.Config.raw("observability.minimum_level")) {
	case "trace", "debug":
		r.report(pwcheck.VerboseLogLevel,
			"observability.minimum_level is "+r.Config.raw("observability.minimum_level"),
			"observability.minimum_level")
	}
	if strings.EqualFold(r.Config.raw("observability.stdout_format"), "plaintext") {
		r.report(pwcheck.PlaintextLogs, "observability.stdout_format is plaintext", "observability.stdout_format")
	}
	if r.databaseConfigured() {
		for _, connection := range r.Config.databaseDSNs() {
			if strings.Contains(connection.DSN, ":memory:") {
				r.report(pwcheck.MemoryDatabase,
					"connection "+connection.Label+" is an in-memory database, so the schema and every row are lost at restart",
					connection.Key)
			}
		}
	}
	if enabled, resolved := r.Config.boolValue("server.public.read_local"); resolved && enabled {
		r.report(pwcheck.LocalPublicRead, "server.public.read_local is true", "server.public.read_local")
	}
	if enabled, resolved := r.Config.boolValue("observability.otel.enabled"); resolved && !enabled {
		r.report(pwcheck.TelemetryDisabled, "observability.otel.enabled is false", "observability.otel.enabled")
	}
}

func (r *checkRun) checkIdentityProvider() {
	if r.State.config.IdP.Enabled && r.Env != pwenv.Development {
		r.report(pwcheck.DevIdPEnabled, "dev.idp.enabled is true in popcornwave.toml", "popcornwave.toml dev.idp")
	}
	if !r.Config.enabled("auth.enabled") {
		return
	}
	issuer := r.Config.raw("auth.oidc.issuer")
	allowLoopback := r.Config.enabled("auth.oidc.allow_loopback_http")
	// The redirect target is checked in every environment: the provider would
	// send the browser to a path the application does not serve, and a loopback
	// redirect that happens to work locally hides it.
	if redirect := r.Config.raw("auth.oidc.redirect_url"); redirect != "" {
		callback := r.Config.raw("auth.callback_path")
		if parsed, err := url.Parse(redirect); err == nil && callback != "" && parsed.Path != callback {
			r.report(pwcheck.RedirectDisagreement,
				"auth.oidc.redirect_url ends at "+parsed.Path+" while auth.callback_path is "+callback,
				"auth.oidc.redirect_url")
		}
	}
	// In dev the provider values are expected to be absent from the file:
	// pw dev runs the development identity provider and injects them. Nothing
	// below has anything to say about that arrangement working.
	if r.Env == pwenv.Development {
		return
	}
	if issuer != "" {
		if parsed, err := url.Parse(issuer); err == nil {
			if isLoopbackHost(parsed.Hostname()) {
				r.report(pwcheck.DevelopmentIssuer,
					"auth.oidc.issuer is a loopback address, which is a development provider",
					"auth.oidc.issuer")
			}
			if parsed.Scheme == "http" {
				r.report(pwcheck.InsecureIssuer,
					"auth.oidc.issuer uses http; the discovery document and every token would travel in the clear",
					"auth.oidc.issuer")
			}
		}
	}
	if allowLoopback {
		r.report(pwcheck.InsecureIssuer,
			"auth.oidc.allow_loopback_http is true, which is a development-only exception",
			"auth.oidc.allow_loopback_http")
		if secure, resolved := r.Config.boolValue("session.cookie.secure"); resolved && !secure {
			r.report(pwcheck.LoopbackPairing,
				"allow_loopback_http and an insecure session cookie are the development pairing",
				"auth.oidc.allow_loopback_http, session.cookie.secure")
		}
	}
	var missing []string
	for key, variable := range map[string]string{
		"auth.oidc.issuer":        "AUTH_OIDC_ISSUER",
		"auth.oidc.client_id":     "AUTH_OIDC_CLIENT_ID",
		"auth.oidc.client_secret": "AUTH_OIDC_CLIENT_SECRET",
	} {
		if strings.TrimSpace(r.Config.raw(key)) == "" {
			missing = append(missing, variable)
		}
	}
	if len(missing) > 0 {
		sortStrings(missing)
		// Absent here is either platform injection or a real gap, and this host
		// cannot tell which; naming what must be set is the useful half.
		r.report(pwcheck.ProviderNotDeclared,
			"authentication is enabled and no provider values are declared for "+r.Env,
			"the deployment must set "+strings.Join(missing, ", "))
	}
}

func (r *checkRun) checkStorage(ctx context.Context) {
	if !r.databaseConfigured() {
		return
	}
	versions := map[int]string{}
	var ordered []int
	for _, name := range r.State.migrations {
		version, _, found := strings.Cut(name, "_")
		number, err := strconv.Atoi(version)
		if !found || err != nil {
			continue
		}
		if existing, clash := versions[number]; clash {
			r.report(pwcheck.DuplicateMigration,
				"migrations "+existing+" and "+name+" share version "+version,
				r.State.config.Migration.Dir)
			continue
		}
		versions[number] = name
		ordered = append(ordered, number)
	}
	sortInts(ordered)
	for index := 1; index < len(ordered); index++ {
		if ordered[index] != ordered[index-1]+1 {
			r.report(pwcheck.MigrationVersionGap,
				fmt.Sprintf("the migration sequence jumps from %05d to %05d", ordered[index-1], ordered[index]),
				r.State.config.Migration.Dir)
			break
		}
	}
	for _, connection := range r.Config.databaseDSNs() {
		if connection.scheme() != "sqlite" {
			continue
		}
		path := strings.TrimPrefix(connection.DSN, "sqlite://")
		if path == "" || strings.HasPrefix(path, ":memory:") {
			continue
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(r.Root, filepath.FromSlash(path))
		}
		directory := filepath.Dir(path)
		if _, err := os.Stat(directory); err != nil {
			r.report(pwcheck.DatabasePathUnwrite,
				"connection "+connection.Label+" would open "+relativeToRoot(r.Root, path)+" and its directory does not exist",
				connection.Key)
		}
	}
	if !r.Online {
		r.report(pwcheck.AppliedStateUnknown,
			"the applied migration state was not read, so the pending count is unknown",
			"--online")
		return
	}
	r.checkDatabaseOnline(ctx)
}

// checkDatabaseOnline connects to the configured database. It runs only under
// --online, uses the goose linkage pw migrate already carries, and applies
// nothing.
func (r *checkRun) checkDatabaseOnline(ctx context.Context) {
	connections := r.Config.databaseDSNs()
	if len(connections) == 0 {
		return
	}
	for _, connection := range connections {
		if absent, path := absentLocalDatabase(r.Root, connection); absent {
			// Opening a sqlite DSN creates the file, and a diagnosis that
			// writes is not one. The absence is reported instead.
			r.report(pwcheck.ConnectionFailed,
				"connection "+connection.Label+" points at "+path+", which does not exist yet",
				connection.Key)
			continue
		}
		target, err := pwmigrate.Open(connection.DSN)
		if err != nil {
			r.report(pwcheck.ConnectionFailed,
				"connection "+connection.Label+" could not be opened: "+err.Error(),
				connection.Key)
			continue
		}
		if err := target.DB.PingContext(ctx); err != nil {
			_ = target.Close()
			r.report(pwcheck.ConnectionFailed,
				"connection "+connection.Label+" did not answer: "+err.Error(),
				connection.Key)
			continue
		}
		_ = target.Close()
	}
	migrationDSN := r.migrationDSN(connections)
	if migrationDSN == "" || r.State.config.Migration.Dir == "" {
		return
	}
	if absent, _ := absentLocalDatabase(r.Root, labeledDSN{DSN: migrationDSN}); absent {
		return
	}
	sources, err := pwmigrate.Sources(filepath.Join(r.Root, filepath.FromSlash(r.State.config.Migration.Dir)))
	if err != nil {
		return
	}
	target, err := pwmigrate.Open(migrationDSN)
	if err != nil {
		return
	}
	defer func() { _ = target.Close() }()
	statuses, err := pwmigrate.Statuses(ctx, target, sources)
	if err != nil {
		r.report(pwcheck.ConnectionFailed,
			"the applied migration state could not be read: "+err.Error(),
			"middleware.rdb.migration_group")
		return
	}
	pending := 0
	for _, status := range statuses {
		if !status.Applied {
			pending++
		}
	}
	if pending > 0 {
		message := fmt.Sprintf("%d migration(s) are not applied", pending)
		if !r.State.config.Migration.Auto {
			message += "; migration.auto is false, so pw dev does not apply them either"
		}
		r.report(pwcheck.PendingMigrations, message, r.State.config.Migration.Dir)
	}
}

// migrationDSN picks the connection migrations run against, following the
// configured migration group and then the write group.
func (r *checkRun) migrationDSN(connections []labeledDSN) string {
	group := r.Config.raw("middleware.rdb.migration_group")
	if group == "" {
		group = r.Config.raw("middleware.rdb.write_group")
	}
	if group == "" {
		return connections[0].DSN
	}
	for index, connection := range connections {
		name := r.Config.raw(fmt.Sprintf("middleware.rdb.connections[%d].group", index))
		if name == group {
			return connection.DSN
		}
	}
	return connections[0].DSN
}

func (r *checkRun) checkReadiness() {
	// server.api_doc names a renderer rather than a flag, so anything but the
	// empty or off value serves the page.
	switch renderer := strings.ToLower(strings.TrimSpace(r.Config.raw("server.api_doc"))); renderer {
	case "", "off", "none", "false":
	default:
		r.report(pwcheck.APIDocExposed,
			"server.api_doc is "+renderer+", so "+r.Config.raw("server.api_doc_path")+" is served",
			"server.api_doc")
	}
	tailwind := r.State.config.Tailwind
	if !tailwind.Enabled {
		return
	}
	if !tailwind.Minify {
		r.report(pwcheck.TailwindMinifyOff, "assets.tailwind.minify is false", "popcornwave.toml assets.tailwind.minify")
	}
	if tailwind.Output != "" && tailwind.Input != "" {
		output := filepath.Join(r.Root, filepath.FromSlash(tailwind.Output))
		if olderThan(output, []string{filepath.Join(r.Root, filepath.FromSlash(tailwind.Input))}) {
			r.report(pwcheck.StylesheetStale,
				tailwind.Output+" is older than "+tailwind.Input,
				tailwind.Output)
		}
	}
	r.checkPrecompression()
}

// checkPrecompression reports a public asset whose compressed sidecar is older
// than it is. pw build writes the sidecars, so this fires on a tree that was
// assembled by hand.
func (r *checkRun) checkPrecompression() {
	public := filepath.Join(r.Root, "public")
	entries, err := os.ReadDir(public)
	if err != nil {
		return
	}
	var stale []string
	var walk func(directory string, items []os.DirEntry)
	walk = func(directory string, items []os.DirEntry) {
		for _, item := range items {
			path := filepath.Join(directory, item.Name())
			if item.IsDir() {
				if children, readErr := os.ReadDir(path); readErr == nil {
					walk(path, children)
				}
				continue
			}
			if strings.HasSuffix(path, ".zstd") {
				continue
			}
			if _, sidecarErr := os.Stat(path + ".zstd"); sidecarErr != nil {
				continue
			}
			if olderThan(path+".zstd", []string{path}) {
				stale = append(stale, relativeToRoot(r.Root, path))
			}
		}
	}
	walk(public, entries)
	if len(stale) > 0 {
		sortStrings(stale)
		r.report(pwcheck.PrecompressionOld,
			fmt.Sprintf("%d public asset(s) are newer than their .zstd sidecar", len(stale)),
			strings.Join(stale, ", "))
	}
}

func (r *checkRun) databaseConfigured() bool {
	return r.Config.enabled("middleware.rdb.enabled")
}

// absentLocalDatabase reports a sqlite file DSN whose file is not there yet,
// with the path as the report would show it. An in-memory DSN is never absent:
// it exists for the length of the connection that opens it.
func absentLocalDatabase(root string, connection labeledDSN) (bool, string) {
	if connection.scheme() != "sqlite" {
		return false, ""
	}
	path := strings.TrimPrefix(strings.TrimSpace(connection.DSN), "sqlite://")
	if path == "" || strings.HasPrefix(path, ":memory:") || strings.Contains(path, "mode=memory") {
		return false, ""
	}
	resolved := path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(root, filepath.FromSlash(resolved))
	}
	if _, err := os.Stat(resolved); err == nil {
		return false, ""
	}
	return true, relativeToRoot(root, resolved)
}

// carriesCredential reports whether a secret-classified value actually holds
// something worth disclosing. Classification is by field name, so it marks
// every DSN; a sqlite path or a credential-free URL has nothing to leak, and
// reporting it would train a reader to ignore the finding that matters.
func carriesCredential(key, raw string) bool {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false
	}
	if !strings.HasSuffix(key, ".dsn") && !strings.Contains(key, "dsn") {
		return true
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return true
	}
	if parsed.User != nil {
		if _, hasPassword := parsed.User.Password(); hasPassword || parsed.User.Username() != "" {
			return true
		}
	}
	for _, name := range []string{"password", "pass", "token", "secret", "auth"} {
		if parsed.Query().Get(name) != "" {
			return true
		}
	}
	return false
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	if address := net.ParseIP(host); address != nil {
		return address.IsLoopback()
	}
	return false
}

// sameMinorVersion compares the first two components, which is the granularity
// a toolchain pin and a go directive agree at.
func sameMinorVersion(a, b string) bool {
	trim := func(value string) string {
		parts := strings.Split(strings.TrimPrefix(value, "v"), ".")
		if len(parts) > 2 {
			parts = parts[:2]
		}
		return strings.Join(parts, ".")
	}
	return trim(a) == trim(b)
}

func sortInts(values []int) {
	for index := 1; index < len(values); index++ {
		for position := index; position > 0 && values[position] < values[position-1]; position-- {
			values[position], values[position-1] = values[position-1], values[position]
		}
	}
}
