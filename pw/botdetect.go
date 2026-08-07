package pw

import (
	"net/http"
	"strings"
)

// botUserAgents are the clients that present a browser-shaped User-Agent and
// still run no script. Everything that names itself honestly — curl, Wget,
// python-requests, Go-http-client — is caught by the Mozilla prefix rule in
// classifyUserAgent instead, so this list only has to carry the bots that
// disguise themselves.
//
// Every token here must be absent from every shipping browser User-Agent.
// That is why the bare substring "bot" is not a member: an Android phone
// reporting CUBOT would become a crawler. browserUserAgents guards the rule.
var botUserAgents = []string{
	// Search crawlers.
	"googlebot", "google-inspectiontool", "storebot-google", "adsbot-google",
	"mediapartners-google", "bingbot", "bingpreview", "msnbot", "slurp",
	"duckduckbot", "duckassistbot", "baiduspider", "yandexbot",
	"yandexrenderresourcesbot", "sogou", "seznambot", "qwantbot", "petalbot",
	"applebot", "naver", "yeti/",

	// AI crawlers and assistants.
	"gptbot", "oai-searchbot", "chatgpt-user", "claudebot", "claude-web",
	"claude-user", "claude-searchbot", "anthropic-ai", "perplexitybot",
	"perplexity-user", "ccbot", "bytespider", "amazonbot",
	"meta-externalagent", "cohere-ai", "diffbot", "youbot", "timpibot",
	"omgili",

	// Link preview and OGP spiders.
	"facebookexternalhit", "facebookcatalog", "meta-externalfetcher",
	"twitterbot", "slackbot", "slack-imgproxy", "discordbot", "telegrambot",
	"whatsapp", "linkedinbot", "pinterest", "redditbot", "skypeuripreview",
	"embedly", "iframely", "vkshare", "mastodon", "bitlybot", "nuzzel",
	"quora link preview",

	// SEO tooling and monitoring.
	"ahrefsbot", "semrushbot", "mj12bot", "dotbot", "dataforseobot",
	"screaming frog", "w3c_validator", "pingdom", "uptimerobot", "statuscake",

	// Windows and Office HTTP stacks, which are the notable client libraries
	// that do claim Mozilla — an inherited habit from the Internet Explorer
	// engine they were built beside — so the prefix rule cannot reach them.
	"winhttp", "wininet", "ms-office", "microsoft office", "microsoft outlook",

	// Self-naming agents that follow no single vendor convention. These are the
	// widest tokens, and "+http" is the contact URL a well-behaved crawler
	// carries — no browser emits one.
	"+http", "crawler", "spider", "feedfetcher", "archive.org_bot",
}

// Google-Extended and Applebot-Extended are deliberately absent: they are
// robots.txt control tokens that never appear as a request header, so an entry
// for either could only ever be dead weight.
//
// HeadlessChrome, Chrome-Lighthouse, Puppeteer, and Playwright are absent for
// the opposite reason: they execute the boundary runtime, so streaming already
// serves them correctly. An application whose capture tool photographs the page
// before the boundaries settle adds the token through html.bot_user_agents.

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

// classifyUserAgent reports whether the agent belongs to a non-interactive
// client. The two tests answer different populations: the token scan catches
// bots wearing a browser costume, and the prefix rule catches every CLI and
// client library at once, including ones released after this list was written.
func classifyUserAgent(agent string, extra []string) bool {
	agent = strings.TrimSpace(agent)
	if agent == "" {
		// A browser always sends one. An absent header is a script that never
		// bothered to set it.
		return true
	}
	// Every mainstream browser still claims Mozilla/5.0 for reasons that stopped
	// being technical decades ago, and effectively no CLI or client library
	// copies the habit. A scraper that does copy it is indistinguishable from a
	// browser and gets treated as one, which is the ceiling of any header-based
	// method rather than a gap in this list.
	//
	// It is tested first because an agent without the prefix is a bot whatever
	// the token scan would say, so only Mozilla-claiming agents pay for the
	// lowercase copy and the scan that catches bots wearing a browser costume.
	if len(agent) < len("mozilla/") || !strings.EqualFold(agent[:len("mozilla/")], "mozilla/") {
		return true
	}
	agent = strings.ToLower(agent)
	for _, token := range botUserAgents {
		if strings.Contains(agent, token) {
			return true
		}
	}
	for _, token := range extra {
		if token != "" && strings.Contains(agent, token) {
			return true
		}
	}
	return false
}

// normalizeBotUserAgents lowercases the configured additions once, so
// classification stays a plain scan.
func normalizeBotUserAgents(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.ToLower(strings.TrimSpace(value)); value != "" {
			normalized = append(normalized, value)
		}
	}
	return normalized
}
