package pwconfig

import "github.com/shibukawa/popcornwave/pwruntime"

// PublishUpdateSettings records what a browser-facing response needs to know
// about the update surface, and what tells a bot apart from a reader.
//
// It is here for the same reason PublishChainSettings is: a parse publishes what
// it resolved, so a runtime that binds no configuration of its own still serves
// with it. Without it the whole update surface is inert on the second transport
// — no live subscription, no redraw, no partial update — and inert is the one
// state that looks like working, because a page still renders.
//
// The values that are not configuration are the framework's own contracts with
// the runtime it ships to the browser, and come from the shared leaf so both
// halves describe one document.
func PublishUpdateSettings() {
	config := Value[HTMLConfig]()
	pwruntime.PublishUpdateSettings(pwruntime.UpdateSettings{
		Enabled: config.Update.Enabled,
		// Live and streaming together, which is the same rule the first
		// transport applies: a buffered document settles its live boundaries in
		// place and holds no placeholder a delivery could replace.
		Live:                config.Live && config.Streaming,
		ValidatorKey:        config.Update.ValidatorKey,
		HeaderPrefix:        pwruntime.UpdateHeaderPrefix,
		DataAttributePrefix: pwruntime.UpdateAttributePrefix,
		GlobalName:          pwruntime.UpdateGlobalName,
		PathPrefix:          pwruntime.UpdatePathPrefix,
		BuildID:             pwruntime.UpdateBuildID(),
		MaxManifestBytes:    config.Update.MaxManifestBytes,
		CSRFHeaderName:      pwruntime.CSRFHeaderName,
		// The merged asset of the unified update runtime is this framework's,
		// so the module serves none and emits no tag of its own.
		CallerOwnsRuntime:  true,
		AsyncTimeout:       config.AsyncTimeout,
		AsyncConcurrency:   config.AsyncConcurrency,
		LiveMaxResponses:   config.LiveMaxResponses,
		LiveMaxBoundaries:  config.LiveMaxBoundaries,
		LiveMaxDuration:    config.LiveMaxDuration,
		LiveDurationJitter: config.LiveDurationJitter,
		LiveIdleTimeout:    config.LiveIdleTimeout,
	})
	// Bot detection travels with them. It is not an update concern, but this is
	// where a resolved HTMLConfig arrives, and both runtimes need the same two
	// values to answer IsBot the same way.
	pwruntime.PublishBotSettings(pwruntime.BotSettings{
		Enabled:    config.BotDetection,
		UserAgents: config.BotUserAgents,
	})
}
