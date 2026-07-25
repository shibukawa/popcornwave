package pw

import (
	"fmt"
	"os"
	"testing"

	"github.com/shibukawa/tinybind-go/configbind"
)

type scaffoldCommandTest struct {
	Format string
}

func TestSubCommandParsesAndExposesSelectedCommand(t *testing.T) {
	const (
		name = "pw-test-generate-config"
		help = "write test configuration scaffolds"
	)
	configbind.RegisterSubCommand[scaffoldCommandTest](configbind.SubCommandDefinition{
		TypeName: "github.com/shibukawa/popcornwave/pw.scaffoldCommandTest",
		Name:     name,
		Help:     help,
		Positionals: []configbind.Positional{{
			ConfigKey: "format",
			Name:      "format",
			Help:      "output format",
			Role:      configbind.PositionalRequired,
		}},
		Apply: func(dst any, overlay *configbind.Overlay) error {
			command, ok := dst.(*scaffoldCommandTest)
			if !ok || command == nil {
				return fmt.Errorf("bad command destination")
			}
			command.Format, _ = overlay.GetString("format")
			return nil
		},
	})

	originalArgs := os.Args
	os.Args = []string{originalArgs[0], name, "toml"}
	t.Cleanup(func() { os.Args = originalArgs })

	SubCommand[scaffoldCommandTest](name, help)
	if _, err := configbind.Load(configbind.LoadOptions{
		Vendor:   "popcornwave-test",
		Tool:     "pw-subcommand-test",
		FileName: "missing.toml",
		Args:     []string{name, "toml"},
		Environ:  []string{},
	}); err != nil {
		t.Fatal(err)
	}

	command, ok := Command[scaffoldCommandTest]()
	if !ok {
		t.Fatal("selected command was not exposed")
	}
	if command.Format != "toml" {
		t.Fatalf("Format = %q", command.Format)
	}
}
