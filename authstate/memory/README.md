# authstate/memory

`memory` provides a bounded, expiry-aware, race-safe implementation of
`authstate.Store[T]`. It is process-local and intended for tests, development,
and single-process deployments.

```go
store, err := memory.NewStore[oauth.Transaction](memory.Options{})
```

`Store.Take` deletes a value before returning it. Nil contexts and nil or zero
store receivers return `authstate.ErrInvalidOptions` instead of panicking.
The defaults allow 4096 entries and 256-byte keys; configuration cannot exceed
65536 entries or 4096 bytes per key.
