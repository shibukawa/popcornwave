package pwcli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func runSchemaInit(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("schema-init: unexpected arguments")
	}
	root, err := projectRoot(".")
	if err != nil {
		return err
	}
	config, err := loadProjectConfig(root)
	if err != nil {
		return err
	}
	schemaDir := filepath.Join(root, "dbschema")
	if _, err := os.Stat(schemaDir); err != nil {
		return fmt.Errorf("schema-init: %w", err)
	}
	command := exec.CommandContext(ctx, "go", "run", config.Main, "--pw-schema-init="+schemaDir)
	command.Dir = root
	command.Stdout = stdout
	command.Stderr = stderr
	command.Env = os.Environ()
	if err := command.Run(); err != nil {
		return fmt.Errorf("schema-init: %w", err)
	}
	return nil
}
