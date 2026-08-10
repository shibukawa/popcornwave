package pwruntime

import "sync/atomic"

// BotSettings is the resolved bot-detection configuration in a form that names
// no transport, so one resolution serves both runtimes.
//
// It travels the way the update settings do, and for the same reason: a
// configuration file is not a transport concern, and the runtime that read it
// and the runtime that serves the request need not be the same one.
type BotSettings struct {
	// Enabled is html.bot_detection. It is off by default, which is why an
	// unpublished value is a usable answer rather than a missing one: a runtime
	// that finds nothing behaves as a deployment that never turned it on.
	Enabled bool
	// UserAgents are the additional tokens from html.bot_user_agents, already
	// lowercased, so classification stays a plain scan.
	UserAgents []string
}

var botSettingsState atomic.Pointer[BotSettings]

// PublishBotSettings records the resolved configuration for whichever runtime
// reads it.
func PublishBotSettings(settings BotSettings) {
	botSettingsState.Store(&settings)
}

// ResolvedBotSettings returns the published configuration, or the zero value
// where nothing published one.
//
// It reports no second value, unlike the update settings. Those have no safe
// default and a runtime that finds none must decline; this one does, because
// detection off is both the configured default and the conservative branch —
// every client gets the streamed render, which is correct for a browser and
// merely slower for a crawler.
func ResolvedBotSettings() BotSettings {
	if settings := botSettingsState.Load(); settings != nil {
		return *settings
	}
	return BotSettings{}
}
