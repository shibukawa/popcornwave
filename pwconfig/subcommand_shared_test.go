package pwconfig_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/pwconfig"
)

// The health probe token is the framework's, and a registration under it could
// never run: the framework consumes the word before application commands are
// parsed, and a HEALTHCHECK already written into a Dockerfile must keep meaning
// the probe. It is refused where it is written rather than shadowed at dispatch.
func TestTheHealthProbeNameIsReserved(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("a subcommand was registered under the reserved probe name")
		}
		if !strings.Contains(recovered.(string), "healthcheck") {
			t.Errorf("the refusal does not name the token: %v", recovered)
		}
	}()
	pwconfig.RegisterSubCommand[struct{}]("healthcheck", "shadow the probe")
}
