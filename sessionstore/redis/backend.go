package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/popcornwave/session"
)

// Importing this package registers the redis session backend:
//
//	import _ "github.com/shibukawa/popcornwave/sessionstore/redis"
//
// Registration opens no connection. The client is dialed when session.backend
// selects this backend at startup.
func init() {
	pw.RegisterSessionBackend(pw.SessionBackendRedis, open)
}

// defaultConnectTimeout bounds the startup ping and the per-command deadlines
// when session.redis.connect_timeout is unset.
const defaultConnectTimeout = 5 * time.Second

// open dials the configured server and refuses to start against one it cannot
// reach, rather than answering the first login with a backend failure.
func open(ctx context.Context, config pw.SessionConfig, _ pw.SessionResources) (session.Backend, error) {
	dsn := strings.TrimSpace(config.Redis.DSN)
	if dsn == "" {
		return session.Backend{}, errors.New(`session.backend = "redis" requires session.redis.dsn`)
	}
	options, err := goredis.ParseURL(dsn)
	if err != nil {
		// Client text would repeat the URL, which can carry a password.
		return session.Backend{}, errors.New("session.redis.dsn is not a redis:// or rediss:// URL")
	}
	timeout := config.Redis.ConnectTimeout
	if timeout <= 0 {
		timeout = defaultConnectTimeout
	}
	// RESP2 and explicit deadlines are what requirement:contrib-redis-valkey
	// verified against both servers.
	options.Protocol = 2
	options.ContextTimeoutEnabled = true
	options.DialTimeout = timeout
	options.ReadTimeout = timeout
	options.WriteTimeout = timeout
	client := goredis.NewClient(options)
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return session.Backend{}, fmt.Errorf("session.redis: server did not answer within %s", timeout)
	}
	store, err := NewStore(client, Options{KeyPrefix: config.Redis.KeyPrefix})
	if err != nil {
		_ = client.Close()
		return session.Backend{}, fmt.Errorf("session.redis: %w", err)
	}
	// This backend opened the client, so it hands back the close. The server
	// expires records on its own, so it hands back no sweep.
	return session.Backend{
		Store: store,
		Close: func(context.Context) error { return client.Close() },
	}, nil
}
