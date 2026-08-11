//go:build !pwdev

package pwconfig

// printDSNBuilt reports whether this binary carries --pw-print-dsn.
//
// It does not. The flag prints the effective database DSN, password included,
// to stdout, and it was reachable from any build: nothing but the argument being
// present stood between a principal who could execute the binary and the
// database credential. A sidecar sharing a process namespace, a CI job reusing
// the image, or a foothold that can exec but not read the mounted secret all had
// it for the asking.
//
// It exists for `pw migrate`, which asks the application for its own DSN rather
// than reimplementing the precedence of TOML, environment, and flags. That is a
// development-toolchain conversation, and the toolchain compiles from source, so
// it can afford to ask for the build that carries the flag.
const printDSNBuilt = false
