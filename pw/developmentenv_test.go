package pw

import "testing"

// restoreEnvState puts the process-wide environment answer back, so one test
// cannot decide another one's relaxations.
func restoreEnvState(t *testing.T) {
	t.Helper()
	envState.RLock()
	value, declared, known := envState.value, envState.declared, envState.known
	envState.RUnlock()
	t.Cleanup(func() {
		envState.Lock()
		envState.value, envState.declared, envState.known = value, declared, known
		envState.Unlock()
	})
}

// An unset APP_ENV is development. Running with no environment set is what
// working on an application looks like, and refusing it would fail the case the
// default exists to serve.
func TestAnUnnamedEnvironmentIsDevelopment(t *testing.T) {
	restoreEnvState(t)
	setEnv(DefaultEnv, false)

	if !Development() {
		t.Error("an unset APP_ENV should keep the development relaxations")
	}
	if EnvironmentDeclared() {
		t.Error("an unset APP_ENV should not count as declared")
	}
}

// A named development environment is the same answer, arrived at deliberately.
func TestANamedDevelopmentEnvironmentIsDevelopment(t *testing.T) {
	restoreEnvState(t)
	setEnv(EnvDevelopment, true)

	if !Development() {
		t.Error("APP_ENV=dev should keep the development relaxations")
	}
	if !EnvironmentDeclared() {
		t.Error("APP_ENV=dev should count as declared")
	}
}

// Every other named environment loses them. The check used to be a list of the
// environments it refused — "stg", "prod", "production" — so "staging", "prd",
// "live" and every other spelling walked past a lock built to stop exactly them.
func TestANamedNonDevelopmentEnvironmentLosesTheRelaxations(t *testing.T) {
	for _, environment := range []string{
		EnvStaging, EnvProduction, "staging", "production", "prd", "live", "uat", "canary", "prod-eu",
	} {
		t.Run(environment, func(t *testing.T) {
			restoreEnvState(t)
			setEnv(environment, true)

			if Development() {
				t.Errorf("APP_ENV=%q kept the development relaxations", environment)
			}
			if !EnvironmentDeclared() {
				t.Errorf("APP_ENV=%q should count as declared", environment)
			}
		})
	}
}

// The relaxations that follow the answer: an insecure session cookie is a
// development-only exception, and query bind values are logged only there.
func TestTheRelaxationsFollowTheEnvironment(t *testing.T) {
	insecure := SessionConfig{Enabled: true, Cookie: SessionCookieConfig{SameSite: "lax"}}

	if err := validateSessionConfig(insecure, EnvDevelopment, true); err != nil {
		t.Errorf("development refused the loopback cookie exception: %v", err)
	}
	if err := validateSessionConfig(insecure, EnvStaging, false); err == nil {
		t.Error("staging started with a session cookie that has no Secure")
	}

	if resolveQueryDiagnostics(queryConfig(nil), true) == nil {
		t.Error("auto should log queries in development")
	}
	if resolveQueryDiagnostics(queryConfig(nil), false) != nil {
		t.Error("auto should stay off outside development")
	}
}
