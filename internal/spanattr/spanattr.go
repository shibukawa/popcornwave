// Package spanattr holds the parts of request span construction that are the
// same whichever transport served the request.
//
// Only one thing is in here so far, and it is the one that matters: what a span
// is allowed to say about a query string. A trace backend is retained longer
// and read more widely than the application database, so getting this wrong on
// one transport and right on the other would publish secrets from half a
// deployment.
package spanattr

import "strings"

// QueryValueMask replaces every query parameter value in an exported span.
const QueryValueMask = "REDACTED"

// redactedQuery keeps the shape of a query string and drops its values.
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
func RedactQuery(raw string) string {
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
