package pwruntime

// NormalizeHTTPMethod maps a request method onto the OpenTelemetry HTTP
// semantic-convention verb set, collapsing anything outside it to "_OTHER".
//
// A metric label must not carry an unbounded value. Both net/http and fasthttp
// accept any RFC 7230 token as a method, and the in-process aggregator never
// evicts a series, so a client sending a stream of distinct arbitrary method
// tokens would otherwise add a permanent time series per token until the
// process runs out of memory. The original method still belongs on a span as
// http.request.method_original; it never belongs on a metric label.
func NormalizeHTTPMethod(method string) string {
	switch method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "CONNECT", "OPTIONS", "TRACE":
		return method
	}
	return "_OTHER"
}

// NormalizeScheme bounds a request scheme to the two an HTTP server metric can
// carry. An absolute-form target or a crafted URI could otherwise put an
// arbitrary scheme on the url.scheme label; anything that is not https is
// reported as http so the label stays two-valued.
func NormalizeScheme(scheme string) string {
	if scheme == "https" {
		return "https"
	}
	return "http"
}
