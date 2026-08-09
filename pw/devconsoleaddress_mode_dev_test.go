//go:build pwdev

package pw

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// TestMain gives the package a console address for the whole run.
//
// The framework's module set is resolved once per process, so a test cannot
// hand itself an address after something else has already asked for the set:
// whichever test ran first would decide what every later one sees. Setting it
// here is what makes the development module, its mark, and the revision they
// land on the ones this build actually serves under pw dev.
//
// The address is a stub server rather than a made-up one because a pwdev
// application announces its listener to whatever the variable names. Pointing
// at something that answers keeps an announcement from spending its timeout and
// writing to stderr in the middle of an unrelated test.
func TestMain(m *testing.M) {
	os.Exit(runWithStubConsole(m))
}

func runWithStubConsole(m *testing.M) int {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer stub.Close()
	os.Setenv(DevConsoleURLVar, stub.URL)
	return m.Run()
}
