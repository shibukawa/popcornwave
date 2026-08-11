package otel

import "strings"

// QueryValueMask replaces every query parameter value in an exported span.
const QueryValueMask = "REDACTED"

// RedactedQuery keeps the shape of a query string and drops its values.
//
// A trace backend is retained longer and read more widely than the application
// database, and a query string is where a password-reset token, an OAuth code,
// and a presigned signature all travel. Exporting it verbatim published them.
//
// The names survive because they are what the attribute is for: knowing that a
// request carried "page" and "token" is the whole diagnostic value, and the
// values were never part of it. The access log settled the same question the
// same way — it records the path and not the query — and the two agreeing is
// worth as much as either answer.
//
// It lives here rather than beside the server middleware because the outbound
// transport asks the same question about the URL it is about to request, and a
// second copy of this rule would be a second chance to answer it differently.
func RedactedQuery(raw string) string {
	var out strings.Builder
	out.Grow(len(raw))
	for remainder := raw; remainder != ""; {
		var pair string
		pair, remainder, _ = strings.Cut(remainder, "&")
		if pair == "" {
			continue
		}
		if out.Len() > 0 {
			out.WriteByte('&')
		}
		name, _, hasValue := strings.Cut(pair, "=")
		out.WriteString(name)
		if hasValue {
			// A valueless parameter is a flag rather than a carrier, so it keeps
			// its exact shape and gains no "=".
			out.WriteByte('=')
			out.WriteString(QueryValueMask)
		}
	}
	return out.String()
}
