package main

import (
	"context"
	"log"

	"helloworld/handlers"

	// Importing the package registers the authentication endpoints. The
	// framework then serves auth.login_path, auth.callback_path, and
	// auth.logout_path from config.{APP_ENV}.toml; this application registers
	// no route and writes no OIDC code.
	_ "github.com/shibukawa/popcornwave/auth"
	"github.com/shibukawa/popcornwave/pw"
)

func main() {
	handlers.RegisterConfig()
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
