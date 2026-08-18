package pwconfig

import (
	"sync"

	"github.com/shibukawa/popcornweb/internal/pwenv"
)

// EnvVar names the environment variable that selects the runtime environment.
const EnvVar = pwenv.Var

// Well-known runtime environments. Any other lowercase token is also accepted.
const (
	EnvDevelopment = pwenv.Development
	EnvStaging     = pwenv.Staging
	EnvProduction  = pwenv.Production
)

// DefaultEnv is used when EnvVar is unset or empty.
const DefaultEnv = pwenv.Default

// The environment lives beside the registry rather than beside the runtime
// because it is an input to the load: it selects which configuration files are
// searched, and it is resolved once, by whoever parses. Everything that reads
// it afterwards — a relaxation, a warning, a default — reads what the parse
// settled.
var envState struct {
	sync.RWMutex
	value    string
	declared bool
	known    bool
}

// Env returns the resolved runtime environment token.
//
// The value comes from APP_ENV and falls back to DefaultEnv. An invalid token
// resolves to DefaultEnv here and fails Parse before requests are served.
func Env() string {
	envState.RLock()
	value, known := envState.value, envState.known
	envState.RUnlock()
	if known && value != "" {
		return value
	}
	resolved, err := pwenv.Resolve(nil)
	if err != nil {
		return DefaultEnv
	}
	return resolved
}

// Development reports whether the development relaxations apply.
//
// An unset APP_ENV counts, because it resolves to development and running the
// framework with no environment set is what working on an application looks
// like. What does not count is any other named environment: the relaxations used
// to be refused by a list of the environments that must not have them — "stg",
// "prod", "production" — which meant "staging", "prd", "live", "uat" and every
// other spelling walked past a lock built to stop exactly them.
//
// The cost of admitting the unset case is that a deployment which forgot the
// variable gets development behaviour. That is a real exposure — query records
// carry bind values there, and the session cookie may travel without Secure — so
// it is not silent: see EnvironmentDeclared and the startup warning it drives.
func Development() bool {
	return Env() == EnvDevelopment
}

// EnvironmentDeclared reports whether APP_ENV named this process's environment,
// as opposed to the process defaulting to development because nothing set it.
//
// It exists for the startup warning. Nothing decides a relaxation on it: the
// decision is Development, and this only says whether anyone asked for the
// answer that came back.
func EnvironmentDeclared() bool {
	envState.RLock()
	declared, known := envState.declared, envState.known
	envState.RUnlock()
	if known {
		return declared
	}
	_, declared, err := pwenv.ResolveDeclared(nil)
	return err == nil && declared
}

func setEnv(value string, declared bool) {
	envState.Lock()
	envState.value = value
	envState.declared = declared
	envState.known = true
	envState.Unlock()
}

// SwapEnv replaces the resolved environment and returns the restore, on the
// same terms as Swap: the environment is process state, and a test that needs
// production behaviour has to be able to say so without setting a variable the
// rest of the run would then see.
func SwapEnv(value string, declared bool) func() {
	envState.RLock()
	previousValue, previousDeclared, previousKnown := envState.value, envState.declared, envState.known
	envState.RUnlock()
	setEnv(value, declared)
	return func() {
		envState.Lock()
		envState.value, envState.declared, envState.known = previousValue, previousDeclared, previousKnown
		envState.Unlock()
	}
}
