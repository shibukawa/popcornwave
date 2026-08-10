package pwruntime

import (
	"sync/atomic"
	"time"
)

// ChainSettings is what building the request chain needs from configuration,
// in a form that names no transport.
//
// It is the fourth thing to travel this way, after the update settings, the bot
// settings and the configuration lookup, and for the same reason each time: a
// configuration file is not a transport concern, and the runtime that read it
// and the runtime that serves the request need not be the same one.
//
// It is a flat value rather than the configuration structs themselves because
// those carry the binder's tags, the scaffold help text and the defaults, none
// of which a chain builder reads. What a chain builder reads is this.
type ChainSettings struct {
	// The three frames a deployment turns on and off.
	RequestID bool
	AccessLog bool
	Recovery  bool
	// RequestTimeout and MaxRequestBody install their frames when positive.
	RequestTimeout time.Duration
	MaxRequestBody int64
	// SecurityHeaders is installed when Enabled says so, which is the switch
	// the configuration keeps beside the values rather than a separate one.
	SecurityHeaders SecurityHeadersConfig
	// TrustedProxies are the networks whose forwarding headers this deployment
	// reads, as configured. They are strings rather than parsed networks
	// because that is what the configuration carries and parsing them is the
	// caller's, which already has to report a bad one against a config key.
	TrustedProxies []string
}

var chainSettingsState atomic.Pointer[ChainSettings]

// PublishChainSettings records the resolved configuration for whichever runtime
// builds a chain from it.
func PublishChainSettings(settings ChainSettings) {
	chainSettingsState.Store(&settings)
}

// ResolvedChainSettings returns the published configuration, and whether
// anything published one.
//
// A runtime that finds none has no configuration to build a chain from and says
// so, rather than composing a chain out of zero values. Zero here would mean no
// recovery frame, no request ID, no security headers — a chain that serves
// requests and looks like a chain, with the frames a deployment configured
// silently missing.
func ResolvedChainSettings() (ChainSettings, bool) {
	if settings := chainSettingsState.Load(); settings != nil {
		return *settings, true
	}
	return ChainSettings{}, false
}
