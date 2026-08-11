package pwconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/shibukawa/popcornwave/internal/pwenv"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinybind-go/configbind"
)

type configEntry struct {
	prefix string
	ptr    any
}

var configState = struct {
	sync.RWMutex
	entries  map[reflect.Type]configEntry
	parsed   bool
	parseErr error
	options  configbind.LoadOptions
	hooks    Hooks
}{
	entries: make(map[reflect.Type]configEntry),
	options: configbind.LoadOptions{
		Vendor:   "popcornwave",
		FileName: "config.toml",
	},
}

// Hooks are what a runtime layers on top of the load without this package
// having to know which runtime it is.
//
// Both are optional, and a runtime that sets neither still gets a complete
// load: the settings are the point, and the two things below are what a
// particular runtime does around them.
type Hooks struct {
	// Args filters the command arguments before the load, which is how a
	// runtime takes its own subcommands off the line before configbind sees
	// them. Returning an error fails the parse.
	//
	// A nil hook does not mean no filtering: the framework's own arguments are
	// this package's and are taken off the line regardless, through
	// ParseFrameworkAction. A runtime sets this only to add to that.
	Args func([]string) ([]string, error)
	// Loaded receives the load result, which is what a startup summary reads.
	// It runs only on a successful load.
	Loaded func(*configbind.LoadResult)
}

// SetHooks installs the hooks of the runtime performing startup. It must be
// called before Parse.
func SetHooks(hooks Hooks) {
	configState.Lock()
	defer configState.Unlock()
	if configState.parsed {
		panic("popcornwave: configuration hooks set after Parse")
	}
	configState.hooks = hooks
}

// Register registers one generated configbind target without parsing it.
func Register[T any](prefix string) {
	if strings.TrimSpace(prefix) == "" {
		panic("popcornwave: empty configuration prefix")
	}
	typ := reflect.TypeFor[T]()
	configState.Lock()
	defer configState.Unlock()
	if configState.parsed {
		panic("popcornwave: configuration registered after ParseConfig")
	}
	if existing, ok := configState.entries[typ]; ok {
		if existing.prefix != prefix {
			panic(fmt.Sprintf("popcornwave: %v already registered with prefix %q", typ, existing.prefix))
		}
		return
	}
	configState.entries[typ] = configEntry{prefix: prefix, ptr: configbind.Bind[T](prefix)}
}

// SetLoadOptions customizes configbind loading before Parse.
func SetLoadOptions(options configbind.LoadOptions) {
	configState.Lock()
	defer configState.Unlock()
	if configState.parsed {
		panic("popcornwave: config options changed after ParseConfig")
	}
	configState.options = options
}

// Parse loads every registered binding, once. Repeated calls return the first
// answer rather than reloading, because a process has one configuration.
func Parse() error {
	// Publishing takes the read lock through the lookup, so it happens after
	// this one is released rather than inside it.
	publish := false
	defer func() {
		if publish {
			PublishChainSettings()
			PublishUpdateSettings()
		}
	}()
	configState.Lock()
	defer configState.Unlock()
	if configState.parsed {
		return configState.parseErr
	}
	configState.parsed = true
	options, env, declared, envErr := resolveLoadOptions(configState.options)
	if envErr != nil {
		configState.parseErr = envErr
		return envErr
	}
	setEnv(env, declared)
	// The framework's own arguments are this package's, so they are filtered off
	// the line whether or not a runtime installed a hook. A runtime that wants
	// to add to that installs one; a build that installs none still answers
	// --generate-config and the health probe, which is what makes the two
	// builds of one application understand the same words.
	filter := configState.hooks.Args
	if filter == nil {
		filter = ParseFrameworkAction
	}
	var actionErr error
	if options.Args, actionErr = filter(commandArgs(options.Args)); actionErr != nil {
		configState.parseErr = actionErr
		return actionErr
	}
	result, err := configbind.Load(options)
	configState.parseErr = err
	if err != nil {
		return err
	}
	DeriveExportEnabled(result, boundConfig[ObservabilityConfig]())
	if loaded := configState.hooks.Loaded; loaded != nil {
		loaded(result)
	}
	publish = true
	return nil
}

// Parsed reports whether Parse has run, which is what a runtime checks before
// deciding whether it is the one that should perform startup.
func Parsed() bool {
	configState.RLock()
	defer configState.RUnlock()
	return configState.parsed
}

// Snapshot copies every registered binding by value, which is what a request
// capsule carries.
func Snapshot() map[reflect.Type]any {
	configState.RLock()
	defer configState.RUnlock()
	configs := make(map[reflect.Type]any, len(configState.entries))
	for typ, entry := range configState.entries {
		value := reflect.ValueOf(entry.ptr)
		if value.Kind() == reflect.Pointer && !value.IsNil() {
			configs[typ] = value.Elem().Interface()
		}
	}
	return configs
}

// Seed gives a registered binding its documented values before anything parses
// a configuration source.
//
// A reader reports a registered target even while it is unparsed, so a zero
// value there reads as an explicit setting rather than as "not configured yet".
// For HTMLConfig that distinction matters: a zero Streaming would turn
// progressive rendering off in every test and in every embedding that never
// parses, which is the opposite of the documented default.
func Seed[T any](value T) {
	configState.Lock()
	defer configState.Unlock()
	entry, ok := configState.entries[reflect.TypeFor[T]()]
	if !ok {
		return
	}
	if ptr, ok := entry.ptr.(*T); ok && ptr != nil {
		*ptr = value
	}
}

// Bound returns the pointer the registry holds for T, which is the value
// configbind writes into.
//
// Nothing on the serving path uses it: a reader takes a copy through
// pwruntime.ResolveConfig, because a pointer into the registry is a value
// anything could change under a request that already read it. It exists for a
// caller that drives a real load and then has to put back what the process had,
// which is a test and only a test.
func Bound[T any]() (*T, bool) {
	configState.RLock()
	defer configState.RUnlock()
	bound := boundConfig[T]()
	return bound, bound != nil
}

// boundConfig returns the registered binding for T, or nil when none is
// registered. Callers hold configState.
func boundConfig[T any]() *T {
	entry, ok := configState.entries[reflect.TypeFor[T]()]
	if !ok {
		return nil
	}
	bound, _ := entry.ptr.(*T)
	return bound
}

// The resolved configuration is published for every runtime, none of which
// binds its own. It is an init rather than a step inside Parse because the
// closure reads the registry when it is called rather than capturing it, so
// publishing it before anything is parsed is both correct and one less ordering
// constraint.
func init() {
	pwruntime.PublishConfigLookup(func(target reflect.Type) (any, bool) {
		configState.RLock()
		defer configState.RUnlock()
		entry, ok := configState.entries[target]
		if !ok {
			return nil, false
		}
		return entry.ptr, true
	})
}

// DeriveExportEnabled turns observability.otel.enabled on whenever an endpoint
// arrived from any source.
//
// Naming an endpoint is the OTLP convention for asking for export, and it is how
// api:cli-dev points an application at its viewer. But every other otel key
// declares enabled as its dependon parent, so leaving the switch off would hide
// the very address that was just configured, and the summary would describe a
// process that does not exist.
//
// The value is written into the overlay as well as the bound struct, because the
// startup summary reads provenance rather than the struct. It is recorded at the
// place the endpoint came from, so the summary says who asked for export instead
// of claiming a default did.
//
// bound is passed in rather than looked up so that the function carries no
// locking contract and a test can drive it against its own value.
func DeriveExportEnabled(result *configbind.LoadResult, bound *ObservabilityConfig) {
	if result == nil || result.Overlay == nil {
		return
	}
	endpoint, ok := result.Overlay.Get("observability.otel.endpoint")
	if !ok || strings.TrimSpace(endpoint.Raw) == "" {
		return
	}
	result.Overlay.Set("observability.otel.enabled", "true", endpoint.Place)
	if bound != nil {
		bound.Otel.Enabled = true
	}
}

// resolveLoadOptions completes options for the active runtime environment.
// Project-local candidates are environment-specific and searched in the working
// directory before its config/ directory; the user and system configuration
// directories keep the environment-neutral file name.
func resolveLoadOptions(options configbind.LoadOptions) (configbind.LoadOptions, string, bool, error) {
	if options.Tool == "" {
		options.Tool = ExecutableName()
	}
	env, declared, err := pwenv.ResolveDeclared(options.Environ)
	if err != nil {
		return options, "", false, err
	}
	if options.FileName == "" {
		options.FileName = pwenv.NeutralFileName
	}
	if options.ExtraConfigReadPaths == nil {
		options.ExtraConfigReadPaths = pwenv.ReadPaths(env)
	}
	return options, env, declared, nil
}

// ExecutableName is the name a configuration search falls back to when the
// caller named no tool.
func ExecutableName() string {
	name := filepath.Base(os.Args[0])
	if name == "" || name == "." {
		return "app"
	}
	return name
}

// commandArgs is the command line as configbind should see it.
//
// A test binary's own flags are dropped, because go test puts them on the same
// line and a configuration parser has no way to know they are not its own.
func commandArgs(configured []string) []string {
	if configured != nil {
		return configured
	}
	args := os.Args[1:]
	if !strings.HasSuffix(os.Args[0], ".test") {
		return args
	}
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-test.") {
			filtered = append(filtered, arg)
		}
	}
	return filtered
}

// Swap replaces one registered binding and returns the pointer it installed
// together with the restore.
//
// It exists because the framework's configuration is process state, and the
// tests that need one setting to differ would otherwise reach into this
// package's map from outside it — which is what they did while the registry
// was in pw and they were in the same package. A seam that restores what it
// replaced is the smaller of the two evils; the alternative is an exported map.
//
// The pointer is handed back because a test that installs a configuration
// often has to adjust one field of it partway through, and reinstalling the
// whole value to change one would be a second seam for the same reason.
func Swap[T any](value T) (*T, func()) {
	configState.Lock()
	defer configState.Unlock()
	typ := reflect.TypeFor[T]()
	previous, existed := configState.entries[typ]
	installed := &value
	configState.entries[typ] = configEntry{prefix: previous.prefix, ptr: installed}
	return installed, func() {
		configState.Lock()
		defer configState.Unlock()
		if existed {
			configState.entries[typ] = previous
			return
		}
		delete(configState.entries, typ)
	}
}
