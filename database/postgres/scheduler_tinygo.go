//go:build tinygo && !scheduler.threads

package postgres

// Under the cooperative scheduler a blocking socket call holds the whole
// runtime, so the driver's cancellation watcher never runs: a query outlives
// its context deadline and returns a nil error, with nothing logged. A 5s
// server-side sleep under a 500ms deadline returned after the full 5s.
//
// TinyGo derives a scheduler.threads build tag from -scheduler, which is what
// lets that mistake be a compile error here instead of a silence at run time.
// The guard is keyed on the import graph, so it fires for exactly the programs
// that link this engine, however the build was invoked.
//
// The identifier below does not exist. Its name is the diagnostic.
var _ = build_this_program_with_tinygo_scheduler_threads
