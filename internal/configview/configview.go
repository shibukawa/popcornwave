// Package configview decides how a resolved configuration value is shown.
//
// The startup summary and pw doctor render the same loaded configuration for
// the same reader, so what a value looks like is decided once, here, rather
// than twice in two packages that would drift.
package configview

import (
	"strconv"
	"strings"

	"github.com/shibukawa/tinybind-go/configbind"
)

// Redacted is the mark a hidden value leaves behind. It matches what configbind
// writes, so one idea never shows up under two marks.
const Redacted = "*****"

// Raw recovers the unmasked value behind one provenance entry.
//
// Provenance masks a secret-classified value, which is right for anything
// printed as-is. A caller that has to read the value — to render part of it, or
// to judge whether it discloses anything — asks the overlay instead. An array
// element is keyed by its array there rather than by the expanded key
// provenance reports, which is why the indexed lookup is a second step.
func Raw(overlay *configbind.Overlay, entry configbind.ProvenanceEntry) (string, bool) {
	if overlay == nil {
		return "", false
	}
	if raw, ok := overlay.GetString(entry.Key); ok {
		return raw, true
	}
	if entry.ArrayKey == "" {
		return "", false
	}
	tables, ok := overlay.GetTables(entry.ArrayKey)
	if !ok || entry.Index < 0 || entry.Index >= len(tables) {
		return "", false
	}
	field, found := strings.CutPrefix(entry.Key, entry.ArrayKey+"["+strconv.Itoa(entry.Index)+"].")
	if !found {
		return "", false
	}
	return tables[entry.Index].GetString(field)
}

// IsDSNKey reports whether a key holds a data source name, which is the one
// secret-classified value with a public half worth showing.
func IsDSNKey(key string) bool {
	return strings.HasSuffix(strings.ToLower(key), ".dsn")
}

// DSN renders a data source name with its credential removed and the rest kept.
//
// Hiding the whole value costs the operator the question a summary is read to
// answer: which database is this process talking to. The parts that answer it —
// the scheme, the host, the port, and the database or file the DSN ends in —
// disclose nothing on their own, so they stay. What is removed is the userinfo,
// which is the credential, and the query string, which is the one place an
// unrecognized secret parameter could hide. A value this cannot take apart is
// hidden whole, because a half-parsed DSN is not worth the risk of printing.
//
//	postgres://app:s3cret@db.internal:5432/app?sslmode=verify-full
//	  -> postgres://*****@db.internal:5432/app
//	mysql://app:s3cret@tcp(db.internal:3306)/app?parseTime=true
//	  -> mysql://*****@db.internal:3306/app
//	sqlite://./app.db  -> sqlite://./app.db
//	sqlite://:memory:  -> sqlite://:memory:
func DSN(value string) string {
	dsn := strings.TrimSpace(value)
	if dsn == "" {
		return ""
	}
	scheme, rest, found := strings.Cut(dsn, "://")
	if !found || scheme == "" || rest == "" || strings.ContainsAny(scheme, " \t") {
		return Redacted
	}
	// A parameter this does not recognize could carry a password, so the whole
	// query goes rather than a list of names that would need maintaining.
	rest, _, _ = strings.Cut(rest, "?")
	credential := false
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		rest, credential = rest[at+1:], true
	}
	// A go-sql-driver address is protocol(host:port), which names the same host
	// and port every other engine writes plainly.
	if open := strings.Index(rest, "("); open >= 0 {
		close := strings.Index(rest[open:], ")")
		if close < 0 {
			return Redacted
		}
		rest = rest[open+1:open+close] + rest[open+close+1:]
	}
	if rest == "" {
		return Redacted
	}
	if credential {
		return scheme + "://" + Redacted + "@" + rest
	}
	return scheme + "://" + rest
}
