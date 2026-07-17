# contrib/authstate/redis

This package adapts a pinned `go-redis` client to `authstate.Store[T]` for
Redis and Valkey. It uses `SET NX` with expiry and `GETDEL`; it is not a general
Redis client. Connect to a loopback or same-Pod proxy when upstream TLS is
required. The caller owns and closes the supplied client.

```go
client := goredis.NewUniversalClient(&goredis.UniversalOptions{
	Addrs: []string{"127.0.0.1:6379"},
	Protocol: 2,
})
store, err := redisstore.NewStore[oauth.Transaction](
	client,
	oauth.TransactionCodec{},
	redisstore.Options{Prefix: "petitweb:", Namespace: "oauth"},
)
```

`Prefix` and `Namespace` are required. Direct Redis Cluster redirection is not
supported through a single TCP TLS tunnel; use a stable proxy endpoint.
