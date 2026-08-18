//go:build pwdev

package pw

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/shibukawa/popcornweb/internal/devconsole"
)

// The announcement crosses a package boundary as a path, a header name, and a
// body shape, all of them written down twice. The console the loop actually runs
// is what this checks them against, so a rename on one side cannot pass.
func TestTheConsoleTakesTheAnnouncedAddressAndLinksIt(t *testing.T) {
	console, err := devconsole.New("127.0.0.1:0", devconsole.Project{
		Name: "app", Environment: "dev", ApplicationURL: "http://localhost:8080",
	}, nil, devconsole.NewAttachment("secret"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(console.Close)
	t.Setenv(DevConsoleURLVar, console.URL())
	t.Setenv(DevAttachTokenVar, "secret")

	announceDevelopmentListener("http://localhost:8081")

	response, err := http.Get(console.URL() + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "http://localhost:8081") {
		t.Errorf("the console never took the announced address:\n%s", body)
	}
}

func TestAnAnnouncementGoesOnlyWhereAConsoleIsRunning(t *testing.T) {
	var announced atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/listening" && r.Header.Get("X-Pw-Attach-Token") == "secret" {
			announced.Add(1)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	t.Setenv(DevAttachTokenVar, "secret")

	t.Setenv(DevConsoleURLVar, server.URL)
	announceDevelopmentListener("http://localhost:8081")
	if got := announced.Load(); got != 1 {
		t.Fatalf("the console was told %d times, want once", got)
	}

	// A pwdev binary started by hand has no console to talk to, which is an
	// ordinary way to run one rather than something to report.
	t.Setenv(DevConsoleURLVar, "")
	announceDevelopmentListener("http://localhost:8081")
	if got := announced.Load(); got != 1 {
		t.Errorf("an announcement was sent with no console configured (%d in total)", got)
	}
}
