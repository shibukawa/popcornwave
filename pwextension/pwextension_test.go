package pwextension

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// A runtime that assembles its chain from arguments can still run the startup
// half of what a blank import brought in — a storage integration opens its
// client and installs nothing — so a plugin means the same thing in both
// builds.
func TestSetupProcessRunsTheStartupOfFrameLessExtensions(t *testing.T) {
	opened, closed := 0, 0
	Register(Extension{
		Name:  "test.frameless",
		Slot:  Slot(11),
		Setup: func(context.Context) (Middleware, error) { opened++; return nil, nil },
		Close: func(context.Context) error { closed++; return nil },
	})

	shutdown, err := SetupProcess(context.Background())
	if err != nil {
		t.Fatalf("SetupProcess: %v", err)
	}
	if opened != 1 {
		t.Errorf("the extension was set up %d times", opened)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown: %v", err)
	}
	if closed != 1 {
		t.Errorf("the extension was closed %d times", closed)
	}
}

// An extension that installs a frame and says where the other transport gets it
// is left alone. Its startup belongs to that package, and running it here as
// well would build the same runtime twice.
func TestSetupProcessLeavesAnExtensionServedElsewhereAlone(t *testing.T) {
	ran := 0
	Register(Extension{
		Name: "test.bothtransports",
		Slot: Slot(13),
		Setup: func(context.Context) (Middleware, error) {
			ran++
			return func(next http.Handler) http.Handler { return next }, nil
		},
		SecondTransport: "example.com/fw/fasthalf",
	})
	shutdown, err := SetupProcess(context.Background())
	if err != nil {
		t.Fatalf("SetupProcess refused an extension that named its other half: %v", err)
	}
	if ran != 0 {
		t.Errorf("the extension was set up %d times on a transport it does not serve", ran)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown: %v", err)
	}
	// It is still in the registry, because the net/http chain installs it from
	// there.
	found := false
	for _, extension := range Registered() {
		if extension.Name == "test.bothtransports" {
			found = true
		}
	}
	if !found {
		t.Error("the extension was removed from the net/http chain's registry")
	}
}

// An extension that installs a frame and names no other half is refused by
// name. Its middleware is net/http's and there is nowhere on the other
// transport to put it, so dropping it silently would leave a build with the
// extension linked, its configuration bound, its startup done, and its
// behaviour absent.
//
// It is last in this file on purpose: the registry is the process's, a
// registration cannot be taken back, and once this one is in every later call
// to SetupProcess fails on it.
func TestSetupProcessRefusesAnExtensionThatInstallsAFrame(t *testing.T) {
	Register(Extension{
		Name: "test.frame",
		Slot: Slot(12),
		Setup: func(context.Context) (Middleware, error) {
			return func(next http.Handler) http.Handler { return next }, nil
		},
	})
	shutdown, err := SetupProcess(context.Background())
	if shutdown != nil {
		_ = shutdown(context.Background())
	}
	if err == nil {
		t.Fatal("an extension installing a net/http frame was accepted")
	}
	if !strings.Contains(err.Error(), "test.frame") {
		t.Errorf("the refusal does not name the extension: %v", err)
	}
}
