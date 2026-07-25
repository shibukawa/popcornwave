package pw

import (
	"io"

	"github.com/shibukawa/tinybind-go/configbind"
)

func ScaffoldTOML() (string, error) { return configbind.ScaffoldTOML() }

func ScaffoldEnv() (string, error) { return configbind.ScaffoldEnv() }

func WriteScaffoldTOML(w io.Writer) error { return configbind.WriteScaffoldTOML(w) }

func WriteScaffoldEnv(w io.Writer) error { return configbind.WriteScaffoldEnv(w) }
