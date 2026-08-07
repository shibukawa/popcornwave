//go:build !tinygo

// net/http/pprof reaches for http.NewResponseController, which TinyGo does not
// have, so the profiler is a host-Go-only tool. Excluding it here keeps the
// TinyGo build of these examples measurable.
package main

import (
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
)

// servePprof exposes the profiler on its own loopback listener when PPROF_ADDR
// is set, so that profiling never changes the mux being measured. It is off
// unless asked for, and it is not part of either service's request path.
func servePprof() {
	addr := os.Getenv("PPROF_ADDR")
	if addr == "" {
		return
	}
	// A CPU profile samples only goroutines that are running, so a goroutine
	// parked waiting for a mutex contributes nothing to it. Contention is
	// therefore invisible in the CPU profile and needs these two, which are off
	// by default because both cost something to collect.
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(10000) // one sample per 10µs blocked

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
	mux.Handle("/debug/pprof/block", pprof.Handler("block"))
	go func() {
		log.Printf("pprof on %s", addr)
		_ = http.ListenAndServe(addr, mux)
	}()
}
