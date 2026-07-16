package authstate_test

import (
	"context"
	"fmt"
	"time"

	"github.com/shibukawa/petitweb-go/contrib/authstate"
)

func ExampleNewMemoryStore() {
	store, err := authstate.NewMemoryStore[string](authstate.Options{})
	if err != nil {
		panic(err)
	}
	expiresAt := time.Now().Add(time.Minute)
	if err := store.Put(context.Background(), "login-state", "opaque-correlation", expiresAt); err != nil {
		panic(err)
	}
	value, err := store.Take(context.Background(), "login-state")
	if err != nil {
		panic(err)
	}
	fmt.Println(value)
	// Output: opaque-correlation
}
