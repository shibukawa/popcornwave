package pwfast

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/popcornweb/pwconfig"
	"github.com/shibukawa/popcornweb/pwratelimit"
	"github.com/shibukawa/tinybind-go/configbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// A process has one configuration, and the layer says so: the load options
// cannot change after a parse and a second parse returns the first answer. So
// the settings file is written once for the package, and a test that needs
// different values swaps the binding it cares about — which is the same seam an
// application's own tests use.
var parseOnce sync.Once

func parsed(t *testing.T) {
	t.Helper()
	parseOnce.Do(func() {
		directory, err := os.MkdirTemp("", "pwfast-run")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(directory, "popcornweb.toml")
		if err := os.WriteFile(path, []byte("[server]\nport = 0\nhealth = \"/healthz\"\n\n"+
			"[middleware]\nrequest_id = true\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		pwconfig.SetLoadOptions(configbind.LoadOptions{
			Vendor:             "popcornweb-pwfast-test",
			Tool:               "pwfast-test",
			ExplicitConfigPath: path,
			Args:               []string{},
			Environ:            []string{"APP_ENV=dev"},
		})
	})
	if err := pwconfig.Parse(); err != nil {
		t.Fatalf("configuration: %v", err)
	}
}

// limitedTo swaps in a limiter configuration for one test.
//
// The chain settings are published again after the swap, because a parse
// publishes them once and a chain builder reads what was published rather than
// the registry. Swapping a binding without republishing changes what a request
// reads and not what the chain was built from, which is exactly the mistake
// this helper exists so a test does not make twice.
func limitedTo(t *testing.T, config pwconfig.RateLimitConfig) {
	t.Helper()
	_, restore := pwconfig.Swap(config)
	pwconfig.PublishChainSettings()
	t.Cleanup(func() {
		restore()
		pwconfig.PublishChainSettings()
	})
}

func hello() fasthttp.RequestHandler {
	mux := NewServeMux()
	mux.HandleFunc("GET /hello", func(r *fasthttp.RequestCtx) { _, _ = r.WriteString("hello") })
	return mux.Handler
}

// Startup is the whole of what an application could not do before: parse the
// settings, open what they name, build the chain, and hand back the shutdown
// that releases it.
//
// The probe is the assertion rather than the handler, because a chain that
// answers its own operational endpoint is one the framework actually built —
// a bare handler would answer /hello whether or not any of this ran.
func TestStartBuildsAChainThatServes(t *testing.T) {
	parsed(t)

	chain, shutdown, err := Start(context.Background(), hello())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		if err := shutdown(context.Background()); err != nil {
			t.Errorf("shutdown: %v", err)
		}
	})

	if status, _, body := serve(t, chain, "/hello"); status != fasthttp.StatusOK || body != "hello" {
		t.Errorf("the application route answered %d %q", status, body)
	}
	status, header, _ := serve(t, chain, "/healthz")
	if status != fasthttp.StatusOK {
		t.Errorf("the health probe answered %d; the framework chain was not built", status)
	}
	if !strings.Contains(strings.ToLower(header), "x-request-id:") {
		t.Errorf("the configured request id frame is missing:\n%s", header)
	}
}

// The limiter is installed from the configuration rather than from an argument,
// which is what makes ratelimit.enabled mean something on this transport.
func TestStartInstallsTheConfiguredRateLimiter(t *testing.T) {
	parsed(t)
	limits := pwratelimit.DefaultConfig()
	limits.Enabled, limits.PerAddress, limits.PerSubject = true, 2, 0
	limitedTo(t, limits)

	chain, shutdown, err := Start(context.Background(), hello())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })

	got := statusesFor(t, chain, "/hello", 3)
	if got[0] != fasthttp.StatusOK || got[1] != fasthttp.StatusOK {
		t.Fatalf("the first two arrivals were %v, want 200", got[:2])
	}
	if got[2] != fasthttp.StatusTooManyRequests {
		t.Errorf("the third arrival = %d; the configured limiter was not installed", got[2])
	}
}

// A configuration that turns the limiter on and names a backend nothing
// registered is refused before a port is bound, rather than served without one.
// The remedy is a blank import, so the message has to name what was asked for.
func TestStartRefusesAnUnreachableRateLimitBackend(t *testing.T) {
	parsed(t)
	limits := pwratelimit.DefaultConfig()
	limits.Enabled, limits.Backend = true, "elasticsearch"
	limitedTo(t, limits)

	_, shutdown, err := Start(context.Background(), hello())
	if err == nil {
		_ = shutdown(context.Background())
		t.Fatal("a limiter with no registered backend started")
	}
	if !strings.Contains(err.Error(), "elasticsearch") {
		t.Errorf("the refusal does not name the backend: %v", err)
	}
}

// Run owns the port, so what it must get right is the part Start cannot be
// asked about: binding, serving, and stopping when the context ends without
// cutting a response in half.
func TestRunServesAndShutsDownOnCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	parsed(t)
	server := pwconfig.Value[pwconfig.ServerConfig]()
	server.Port = port
	_, restore := pwconfig.Swap(server)
	t.Cleanup(restore)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, hello()) }()

	base := "http://127.0.0.1:" + itoa(port)
	if err := waitFor(base + "/hello"); err != nil {
		cancel()
		t.Fatalf("the server never answered: %v", err)
	}
	response, err := http.Get(base + "/hello")
	if err != nil {
		cancel()
		t.Fatalf("get: %v", err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if string(body) != "hello" {
		t.Errorf("body = %q", body)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after its context was cancelled")
	}

	// The port is released, which is what makes a restart possible.
	again, err := net.Listen("tcp", "127.0.0.1:"+itoa(port))
	if err != nil {
		t.Errorf("the listener was not closed: %v", err)
	} else {
		_ = again.Close()
	}
}

func waitFor(url string) error {
	var last error
	for i := 0; i < 100; i++ {
		response, err := http.Get(url)
		if err == nil {
			_ = response.Body.Close()
			return nil
		}
		last = err
		time.Sleep(20 * time.Millisecond)
	}
	return last
}
