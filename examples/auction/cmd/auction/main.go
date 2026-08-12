package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"auction/handlers"
	"auction/pages"
	"github.com/shibukawa/popcornwave/pw"
	// Registers the engine the configured DSN names.
	_ "github.com/shibukawa/popcornwave/database/sqlite"
	// session.backend = "redis" is served by this import; storage is opt-in.
	_ "github.com/shibukawa/popcornwave/sessionstore/redis"
	// The single-use login records this engine stores.
	_ "github.com/shibukawa/popcornwave/authstate/sqlite"
)

func main() {
	for _, name := range []string{"SESSION_KEYRING_SECRET", "AUTH_OIDC_CLIENT_SECRET"} {
		if err := loadSecretEnv(name); err != nil {
			log.Fatal(err)
		}
	}

	// Names the API document served at server.openapi_path and shown by the
	// reference UI at /docs. Without it both fall back to "Application API".
	if err := pw.SetOpenAPIInfo(pw.OpenAPIInfo{Title: "auction", Version: "0.1.0"}); err != nil {
		log.Fatal(err)
	}

	// Installed before Run: the framework calls these while it serves a login.
	handlers.RegisterAccounts()
	// The page routes join the handler mux. Registration order does not
	// matter; a duplicate pattern would panic here rather than shadow.
	mux := handlers.Handlers()
	pages.Register(mux)
	if err := pw.Run(context.Background(), mux); err != nil {
		log.Fatal(err)
	}
}

// loadSecretEnv implements the conventional NAME_FILE deployment binding.
// It keeps mounted Docker secrets out of the container environment until this
// process needs to hand them to Popcorn Wave's normal configuration loader.
func loadSecretEnv(name string) error {
	if os.Getenv(name) != "" {
		return nil
	}
	path := os.Getenv(name + "_FILE")
	if path == "" {
		return nil
	}
	value, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s_FILE: %w", name, err)
	}
	trimmed := strings.TrimRight(string(value), "\r\n")
	if trimmed == "" {
		return fmt.Errorf("read %s_FILE: secret is empty", name)
	}
	return os.Setenv(name, trimmed)
}
