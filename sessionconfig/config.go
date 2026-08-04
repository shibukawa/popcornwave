// Package sessionconfig holds the configuration bindings of per-browser state.
//
// The structs live here rather than in pw so that both sides of the boundary
// can name them without importing each other: pw installs the session
// middleware and popcornwave/plugin/auth supplies the lifetime, and neither may
// depend on the other. pw re-exports every type below as a true alias, so an
// application writes pw.SessionConfig and the configuration registry, which is
// keyed by reflect.Type, resolves the two names to one entry.
//
// Nothing here imports anything of the framework, which is what keeps it usable
// from either side.
package sessionconfig

import "time"

// Session storage backends.
const (
	// SessionBackendRDB keeps records in a database through
	// sessionstore/sqlite. It is the backend a deployment that must revoke a
	// session, or that outgrows a cookie, uses.
	SessionBackendRDB = "rdb"
	// SessionBackendCookie keeps records in a sealed browser cookie. It needs
	// no storage at all, and it cannot revoke a record it already wrote.
	SessionBackendCookie = "cookie"
	// SessionBackendRedis keeps records in Redis or Valkey through
	// sessionstore/redis, where the server owns expiry and no sweep runs.
	SessionBackendRedis = "redis"
	// SessionBackendDynamo keeps records in DynamoDB through
	// sessionstore/dynamo, for a deployment with no relational database at
	// all. TTL on the deployed table removes dead records, and nothing sweeps.
	SessionBackendDynamo = "dynamo"
)

// SessionConfig selects where per-browser state lives and how its token cookie
// travels. It declares no duration at all: an expiry states how long a proof of
// identity stays good, so every session lifetime is declared under [auth].
//
// The session token is opaque in every backend; only SessionBackendCookie keeps
// the record on the client for the whole of a session, and every backend seals
// one into the browser while the session is still anonymous.
type SessionConfig struct {
	Enabled bool `default:"false"`
	// Backend selects the storage plugin: rdb, cookie, redis, or dynamo. Every
	// backend but cookie reaches the binary through its own blank import. It
	// names which server backend a server-placed slot uses, never whether a
	// slot is server-placed, which RegisterSessionStore states instead.
	Backend string `default:"rdb" dependon:".enabled" help:"session storage backend: rdb, cookie, redis, or dynamo"`
	// Retention bounds how long the store may hold one record.
	//
	// It is not the session lifetime, which [auth] declares: an expiry states
	// how long a proof of identity stays good, and this states how long bytes
	// may sit in a table. They bound different things, so the effective record
	// lifetime is whichever is shorter.
	//
	// A server backend needs it whatever authentication says. A record with no
	// deadline is one the expiry sweep has no cutoff for, and the sweep is what
	// keeps a table bounded when sessions are abandoned rather than ended.
	Retention   time.Duration            `default:"720h" dependon:".enabled" help:"how long the store may hold one record; the session lifetime under [auth] narrows it"`
	Cookie      SessionCookieConfig      `dependon:".enabled"`
	RDB         SessionRDBConfig         `dependon:".enabled"`
	Redis       SessionRedisConfig       `dependon:".enabled"`
	CookieStore SessionCookieStoreConfig `dependon:".enabled"`
	Keyring     SessionKeyringConfig     `dependon:".enabled"`
	Dynamo      SessionDynamoConfig      `dependon:".enabled"`
}

// SessionDynamoConfig configures the DynamoDB session store. It carries no
// endpoint and no credential: middleware.dynamo already opens the client this
// backend borrows.
type SessionDynamoConfig struct {
	// Table is the declared table name, which rule:dynamodb-table-naming maps
	// onto the deployed one.
	Table string `default:"popcornwave_session" help:"declared session table name"`
	// ConsistentRead makes the first read strongly consistent and removes the
	// retry a miss otherwise pays. It costs twice the read capacity.
	ConsistentRead bool `default:"false" help:"read sessions with strong consistency"`
}

// SessionRedisConfig configures the Redis-compatible session store. The server
// owns record expiry, so nothing here schedules a sweep.
type SessionRedisConfig struct {
	// DSN is a redis:// or rediss:// URL. Keep credentials out of the file
	// itself with a ${NAME} reference or SESSION_REDIS_DSN.
	DSN string `secret:"mask" env:"SESSION_REDIS_DSN" help:"redis:// or rediss:// session server"`
	// KeyPrefix isolates session keys from every other user of the server.
	KeyPrefix string `default:"pw:session:" help:"key space owned by the session store"`
	// ConnectTimeout bounds the startup ping and the per-command deadlines.
	ConnectTimeout time.Duration `default:"5s" help:"startup ping and per-command deadline"`
}

// SessionCookieConfig is the browser cookie policy of the session middleware.
type SessionCookieConfig struct {
	Name     string `default:"pw_session"`
	Path     string `default:"/"`
	Domain   string
	Secure   bool   `default:"true" help:"disable only for loopback development"`
	HTTPOnly bool   `default:"true"`
	SameSite string `default:"lax"`
}

// SessionRDBConfig configures the database-backed session store. The middleware
// source reuses the pool owned by middleware.rdb; the dedicated source opens
// its own pool from DSN.
type SessionRDBConfig struct {
	Source string `default:"middleware" help:"middleware reuses middleware.rdb; dedicated opens rdb.dsn"`
	// Group names the middleware-source connection group holding the session
	// table. Empty resolves to middleware.rdb.write_group.
	Group string `help:"connection group holding the session table"`
	DSN   string `secret:"mask" help:"dedicated session database DSN"`
	Table string `default:"popcornwave_session"`
}

// SessionCookieStoreConfig configures the client-side session backend. The
// record cookie follows the [session.cookie] policy, so both cookies of a
// session expire and travel under the same rules; only the name is separate.
type SessionCookieStoreConfig struct {
	// Name holds the sealed record beside the token cookie.
	Name string `default:"pw_session_data" help:"cookie holding the sealed record"`
}

// SessionKeyringConfig holds the secret that protects everything the browser
// carries.
//
// It is not the cookie backend's setting, which is why it is not under
// [session.cookie_store]: one secret serves both protections a slot can carry,
// because a session.ReadOnly slot signs and a session.Private slot seals, and
// session.Keyring derives a purpose-separated subkey per mode from it. A
// deployment on rdb, redis, or dynamo needs it exactly as much as one on
// cookie, because the anonymous phase of a private slot is a sealed cookie
// whatever the backend is.
//
// It is therefore required unless every declared slot is session.Shared, which
// is the only placement that protects nothing.
type SessionKeyringConfig struct {
	// Secret is 32 or more random bytes in base64, generated with
	// `openssl rand -base64 32`. Keep it out of the file itself outside
	// development: write "${SESSION_KEYRING_SECRET}" or set the environment
	// variable. `pw init` generates one into config.dev.toml so a scaffolded
	// project runs without an authored secret, and `pw doctor` reports a
	// literal in any other environment as an error.
	Secret string `secret:"mask" env:"SESSION_KEYRING_SECRET" help:"base64 secret signing and sealing everything the browser carries"`
	// PreviousSecrets keep values written before a rotation readable. They
	// never write.
	PreviousSecrets []string `secret:"mask" help:"retired secrets kept readable during a rotation"`
}

// SessionLifetimeConfig bounds how long a session stays valid.
//
// These durations used to live under [session]. They belong here because an
// absolute expiry, an idle expiry, and a re-proof window are three answers to
// one question, how long a proof of identity stays good, and splitting them
// across two bindings split one policy across two files. A deployment reasons
// about them together, and a TTL shorter than a guard window is a
// misconfiguration only this package can detect.
//
// The session package enforces whatever deadline it is handed and forms no
// opinion about the number, which is what lets one store hold a shopping cart
// and a login.
type SessionLifetimeConfig struct {
	// TTL is the absolute session lifetime.
	TTL time.Duration `default:"24h" help:"absolute session lifetime"`
	// IdleTimeout expires a session that stops being used. Zero disables it.
	IdleTimeout time.Duration `default:"0s" help:"inactivity expiry; zero disables it"`
	// RenewalInterval bounds how often an active request renews idle expiry.
	RenewalInterval time.Duration `default:"0s" help:"minimum interval between idle expiry renewals"`
}
