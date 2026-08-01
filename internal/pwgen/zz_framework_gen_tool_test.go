package pwgen

import (
	"os"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// TestGenerateFrameworkConfigBind regenerates the configbind definition of one
// framework package. It is a tool rather than a test: set PWGEN_DIR to run it.
func TestGenerateFrameworkConfigBind(t *testing.T) {
	dir := os.Getenv("PWGEN_DIR")
	if dir == "" {
		t.Skip("PWGEN_DIR not set")
	}
	options, err := Options("sqlite")
	if err != nil {
		t.Fatal(err)
	}
	path, err := generator.New(options).GenerateConfigBind(dir, dir, "configbind_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("wrote", path)
}
