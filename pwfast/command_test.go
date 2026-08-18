package pwfast

import (
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/pwconfig"
)

// The second build used to take no command line at all: every flag was an
// unknown flag, --generate-config did nothing, and an application's own
// subcommand could not be reached. The parsing is the shared layer's now, so
// what this holds is that this transport asks it.
//
// It is the args hook rather than a spawned binary, because the words are the
// same words by construction — the two builds call one parser — and what a
// runtime can still get wrong is failing to consult it.
func TestTheFrameworkArgumentsAreTakenOffTheCommandLine(t *testing.T) {
	remaining, err := pwconfig.ParseFrameworkAction([]string{"--app-name", "x", "--generate-config", "toml"})
	if err != nil {
		t.Fatalf("ParseFrameworkAction: %v", err)
	}
	// The application's own arguments survive; the framework's are consumed.
	if len(remaining) != 2 || remaining[0] != "--app-name" {
		t.Errorf("the application's arguments did not survive: %v", remaining)
	}
	for _, arg := range remaining {
		if strings.HasPrefix(arg, "--generate-config") {
			t.Errorf("a framework argument reached the application's parser: %v", remaining)
		}
	}
	// Selected rather than merely removed, which is what Run answers before it
	// starts a server.
	if err := pwconfig.RefusePendingFrameworkAction(); err == nil {
		t.Error("a pending action was not reported to a caller that cannot answer it")
	} else if !strings.Contains(err.Error(), "generate-config") {
		t.Errorf("the refusal does not name the action: %v", err)
	}
	// Answered, so nothing is left pending for the next test in this package.
	if handled, err := pwconfig.RunFrameworkAction(); !handled {
		t.Errorf("the selected action was not answered: %v", err)
	}
}
