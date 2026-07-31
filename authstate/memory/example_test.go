package memory_test

import (
	"context"
	"fmt"
	"time"

	"github.com/shibukawa/popcornwave/authstate/memory"
)

func ExampleNewStore() {
	store, err := memory.NewStore[string](memory.Options{})
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
