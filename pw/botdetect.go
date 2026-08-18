package pw

import (
	"net/http"

	"github.com/shibukawa/popcornweb/internal/botdetect"
)

// botUserAgents is the shared token list, named here because this package's
// tests assert against it and because a reader looking for the list should find
// it from the entry that uses it.
var botUserAgents = botdetect.UserAgents

// IsBot reports whether the request came from a client that will not run the
// boundary runtime, so [WriteHTMLChain] should render the settled document
// rather than committing fallbacks it can never replace.
//
// The answer comes from the User-Agent header alone. Nothing verifies it, which
// is acceptable precisely because the only thing it decides is which render
// branch runs: both branches render one chain with one set of data, so a forged
// header buys a slower first byte and nothing else. For the same reason this
// must never become an input to an access decision, and a handler must not use
// it to change what a page says — differing content by User-Agent is cloaking,
// while differing delivery for the same content is not.
func IsBot(r *http.Request) bool {
	if r == nil {
		return false
	}
	return isBotRequest(r, Config[HTMLConfig](requestContext(r)))
}

func isBotRequest(r *http.Request, config HTMLConfig) bool {
	if !config.BotDetection {
		return false
	}
	return classifyUserAgent(r.Header.Get("User-Agent"), config.BotUserAgents)
}

// classifyUserAgent answers from the shared classifier, so the other runtime
// reaches the same verdict from the same list rather than from a copy of it.
func classifyUserAgent(agent string, extra []string) bool {
	return botdetect.Classify(agent, extra)
}

// normalizeBotUserAgents lowercases the configured additions once, so
// classification stays a plain scan.
func normalizeBotUserAgents(values []string) []string {
	return botdetect.Normalize(values)
}
