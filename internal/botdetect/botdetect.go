// Package botdetect classifies a User-Agent as a client that will not run the
// boundary runtime.
//
// It is here rather than in pw because both runtimes ask the same question and
// there is only one right answer to it. A second copy of the token list would
// drift, and the way it would drift is silent: a crawler added on one transport
// and not the other changes which render branch runs, and both branches produce
// a correct-looking page. Nothing would fail, and the difference would show up
// as a search result rather than as a test.
//
// Nothing here touches a request. The caller reads the header, on whichever
// transport it has, and hands over the string.
package botdetect

import "strings"

// UserAgents are the clients that present a browser-shaped User-Agent and still
// run no script. Everything that names itself honestly — curl, Wget,
// python-requests, Go-http-client — is caught by the Mozilla prefix rule in
// Classify instead, so this list only has to carry the bots that disguise
// themselves.
//
// Every token here must be absent from every shipping browser User-Agent.
// That is why the bare substring "bot" is not a member: an Android phone
// reporting CUBOT would become a crawler. The browser list in pw's tests guards
// the rule.
var UserAgents = []string{
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

// Classify reports whether the agent belongs to a non-interactive client. The
// two tests answer different populations: the token scan catches bots wearing a
// browser costume, and the prefix rule catches every CLI and client library at
// once, including ones released after this list was written.
func Classify(agent string, extra []string) bool {
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
	for _, token := range UserAgents {
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

// Normalize lowercases the configured additions once, so classification stays a
// plain scan.
func Normalize(values []string) []string {
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
