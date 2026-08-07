//go:build tinygo && !scheduler.threads

package mysql

// This engine speaks a network protocol, so it needs -scheduler=threads for
// the reason recorded in the postgres package: under the cooperative scheduler
// a blocking socket call holds the runtime and the cancellation watcher never
// runs, so a query outlives its deadline and reports no error.
//
// The identifier below does not exist. Its name is the diagnostic.
var _ = build_this_program_with_tinygo_scheduler_threads
