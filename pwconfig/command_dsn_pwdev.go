//go:build pwdev

package pwconfig

// printDSNBuilt reports whether this binary carries --pw-print-dsn. See the
// !pwdev file for why it is a build tag rather than a runtime check.
const printDSNBuilt = true
