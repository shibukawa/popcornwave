package auth

import (
	"strings"
	"testing"
	"time"
)

func presenceConfig() PresenceConfig {
	return PresenceConfig{Enabled: true, Interval: time.Minute, AbsentAfter: 30 * time.Minute}
}

// A single late report must not end a session: a slow network produces one as
// readily as an empty chair, and the two are not the same claim.
func TestPresenceRefusesAWindowOneMissedTickWouldClose(t *testing.T) {
	config := baseConfig(ModeOIDCOnly)
	config.Assurance.Presence = presenceConfig()
	config.Assurance.Presence.AbsentAfter = time.Minute
	if err := config.validateShape(); err == nil || !strings.Contains(err.Error(), "absent_after") {
		t.Fatalf("absent_after equal to interval = %v", err)
	}

	config.Assurance.Presence.AbsentAfter = 30 * time.Minute
	if err := config.validateShape(); err != nil {
		t.Fatalf("valid presence settings = %v", err)
	}
}

func TestPresenceRefusesNonPositiveDurations(t *testing.T) {
	for name, mutate := range map[string]func(*PresenceConfig){
		"zero interval":     func(p *PresenceConfig) { p.Interval = 0 },
		"zero absent_after": func(p *PresenceConfig) { p.AbsentAfter = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			config := baseConfig(ModeOIDCOnly)
			config.Assurance.Presence = presenceConfig()
			mutate(&config.Assurance.Presence)
			if err := config.validateShape(); err == nil {
				t.Fatal("a non-positive duration was accepted")
			}
		})
	}
}

// Presence off is the default and validates nothing, so a deployment that never
// considered the signal keeps the request-driven idle expiry it already had.
func TestPresenceOffValidatesNothing(t *testing.T) {
	config := baseConfig(ModeOIDCOnly)
	config.Assurance.Presence = PresenceConfig{Interval: -time.Second, AbsentAfter: -time.Second}
	if err := config.validateShape(); err != nil {
		t.Fatalf("disabled presence = %v", err)
	}
}

// A wall-clock gap far larger than the tick interval is how a machine waking
// from sleep is inferred, because nothing reports a wake directly. It counts as
// absence: nobody was there for the gap.
func TestALargeClockGapCountsAsAbsence(t *testing.T) {
	config := presenceConfig()
	cases := map[string]struct {
		report presenceReport
		absent bool
	}{
		"interaction":            {presenceReport{Active: true}, false},
		"no interaction":         {presenceReport{Active: false}, true},
		"slept past the window":  {presenceReport{Active: true, Gap: int64((31 * time.Minute).Seconds())}, true},
		"brief gap while active": {presenceReport{Active: true, Gap: 5}, false},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			absent := !testCase.report.Active
			if config.AbsentAfter > 0 && testCase.report.Gap > 0 &&
				time.Duration(testCase.report.Gap)*time.Second >= config.AbsentAfter {
				absent = true
			}
			if absent != testCase.absent {
				t.Fatalf("absent = %v, want %v", absent, testCase.absent)
			}
		})
	}
}
