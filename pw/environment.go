package pw

import (
	"sync"

	"github.com/shibukawa/popcornwave/internal/pwenv"
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

var envState struct {
	sync.RWMutex
	value    string
	declared bool
	known    bool
}

// Env returns the resolved runtime environment token.
//
// The value comes from APP_ENV and falls back to DefaultEnv. An invalid token
// resolves to DefaultEnv here and fails ParseConfig before requests are served.
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

// reportEnvironment says at startup that nobody named this process's
// environment, and what that silence bought.
//
// An unset APP_ENV resolves to development, and development is not a neutral
// default: query records may carry bind values, the session cookie may travel
// without Secure, and an error page carries the whole problem rather than its
// status. Those are the right answers for someone working on the application and
// the wrong ones for a deployment that simply forgot the variable.
//
// So the framework keeps the convenient behaviour and stops being quiet about
// it. This is a warning rather than a refusal because refusing would fail the
// case it is trying to help — the developer who just cloned the project and ran
// it — and because a deployment that reads its own startup log has everything it
// needs here: the variable to set, and what setting it changes.
func reportEnvironment() {
	if EnvironmentDeclared() {
		return
	}
	processLogger().Warn("popcornwave is running as development because no environment was named",
		String("variable", EnvVar),
		String("environment", Env()),
		String("effect", "query bind values may be logged, session.cookie.secure may be false, and error pages carry their detail"),
		String("action", "set "+EnvVar+" to the environment this process is actually running in"),
	)
}
