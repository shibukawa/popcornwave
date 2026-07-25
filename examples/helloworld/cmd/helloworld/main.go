package main

import (
	"context"
	"log"

	"helloworld/handlers"

	"github.com/shibukawa/popcornwave/pw"
)

func main() {
	handlers.RegisterConfig()
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
