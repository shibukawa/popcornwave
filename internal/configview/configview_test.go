package configview

import "testing"

// A summary is read to answer which database this process talks to, so the
// answer stays and only the credential goes.
func TestDSNKeepsTheAddressAndDropsTheCredential(t *testing.T) {
	for dsn, want := range map[string]string{
		"postgres://app:s3cret@db.internal:5432/app?sslmode=verify-full": "postgres://" + Redacted + "@db.internal:5432/app",
		"postgres://db.internal:5432/app":                                "postgres://db.internal:5432/app",
		"postgresql://app@db.internal/app":                               "postgresql://" + Redacted + "@db.internal/app",
		"mysql://app:s3cret@tcp(db.internal:3306)/app?parseTime=true":    "mysql://" + Redacted + "@db.internal:3306/app",
		"sqlite://./app.db":                                              "sqlite://./app.db",
		"sqlite://:memory:":                                              "sqlite://:memory:",
		"redis://:s3cret@cache.internal:6379/0":                          "redis://" + Redacted + "@cache.internal:6379/0",
		"rediss://cache.internal:6379/0":                                 "rediss://cache.internal:6379/0",
		// The file layer expands ${NAME} before this, so an unexpanded one
		// arrived verbatim from the environment and names a variable rather
		// than holding a secret. It sits where the credential does either way.
		"postgres://app:${DB_PASSWORD}@db.internal:5432/app": "postgres://" + Redacted + "@db.internal:5432/app",
	} {
		if got := DSN(dsn); got != want {
			t.Errorf("DSN(%q) = %q, want %q", dsn, got, want)
		}
	}
}

// Anything this cannot take apart is hidden whole: a half-parsed DSN is not
// worth the risk of printing.
func TestDSNHidesWhatItCannotParse(t *testing.T) {
	for _, dsn := range []string{
		"app.db",
		"postgres://",
		"://db.internal/app",
		"mysql://app:s3cret@tcp(db.internal:3306/app",
		"postgres://app:s3cret@",
	} {
		if got := DSN(dsn); got != Redacted {
			t.Errorf("DSN(%q) = %q, want it hidden", dsn, got)
		}
	}
	if got := DSN("   "); got != "" {
		t.Errorf("DSN of an empty value = %q, want empty", got)
	}
}

func TestIsDSNKey(t *testing.T) {
	for key, want := range map[string]bool{
		"middleware.rdb.connections[0].dsn": true,
		"session.redis.dsn":                 true,
		"session.rdb.dsn":                   true,
		"auth.oidc.client_secret":           false,
		"observability.otel.headers":        false,
	} {
		if got := IsDSNKey(key); got != want {
			t.Errorf("IsDSNKey(%q) = %v, want %v", key, got, want)
		}
	}
}
