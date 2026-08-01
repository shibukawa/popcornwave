// Package passkeye2e holds the end-to-end test of the passkey ceremony
// endpoints. It carries no source of its own.
//
// It is a package rather than another file in plugin/auth because framework
// configuration is parsed once per process: pw.SetConfigLoadOptions panics when
// it is called again afterwards, so one test binary can build exactly one
// deployment. plugin/auth already spends its build on the oidc_only login test,
// and an oidc_passkey deployment needs a different one.
package passkeye2e
