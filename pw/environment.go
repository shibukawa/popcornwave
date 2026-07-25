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
	value string
}

// Env returns the resolved runtime environment token.
//
// The value comes from APP_ENV and falls back to DefaultEnv. An invalid token
// resolves to DefaultEnv here and fails ParseConfig before requests are served.
func Env() string {
	envState.RLock()
	value := envState.value
	envState.RUnlock()
	if value != "" {
		return value
	}
	resolved, err := pwenv.Resolve(nil)
	if err != nil {
		return DefaultEnv
	}
	return resolved
}

func setEnv(value string) {
	envState.Lock()
	envState.value = value
	envState.Unlock()
}
