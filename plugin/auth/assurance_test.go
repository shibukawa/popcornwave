package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNamedPolicyResolvesItsWindow(t *testing.T) {
	config := baseConfig(ModeOIDCOnly)
	config.Assurance = AssuranceConfig{Policy: []AssurancePolicy{
		{Name: "admin", MaxAge: 15 * time.Minute},
		{Name: "danger", MaxAge: 0},
	}}
	request := httptest.NewRequest("GET", "/admin", nil)
	for name, want := range map[string]time.Duration{"admin": 15 * time.Minute, "danger": 0} {
		got, ok := Policy(name).maxAge(request, config)
		if !ok || got != want {
			t.Fatalf("policy %q = %v, %v; want %v", name, got, ok, want)
		}
	}
	if _, ok := Policy("absent").maxAge(request, config); ok {
		t.Fatal("an undefined policy resolved")
	}
}

// An undefined name must fail at startup rather than at the request that needed
// it, because a guard that cannot resolve its own requirement is a route with no
// guard at all.
func TestAnUndefinedPolicyNameIsRefusedAtStartup(t *testing.T) {
	config := baseConfig(ModeOIDCOnly)
	config.Assurance = AssuranceConfig{Policy: []AssurancePolicy{
		{Name: "admin", MaxAge: time.Minute},
		{Name: "admin", MaxAge: time.Hour},
	}}
	err := config.validateShape()
	if err == nil || !strings.Contains(err.Error(), "declared twice") {
		t.Fatalf("duplicate policy name = %v", err)
	}

	config.Assurance = AssuranceConfig{Policy: []AssurancePolicy{{MaxAge: time.Minute}}}
	if err := config.validateShape(); err == nil || !strings.Contains(err.Error(), "name must be set") {
		t.Fatalf("unnamed policy = %v", err)
	}

	config.Assurance = AssuranceConfig{Policy: []AssurancePolicy{{Name: "x", MaxAge: -time.Second}}}
	if err := config.validateShape(); err == nil || !strings.Contains(err.Error(), "must not be negative") {
		t.Fatalf("negative window = %v", err)
	}
}

func TestDefaultRequirementReadsRecentAuthMaxAge(t *testing.T) {
	config := baseConfig(ModeOIDCOnly)
	config.RecentAuthMaxAge = 90 * time.Second
	got, ok := Default().maxAge(httptest.NewRequest("GET", "/", nil), config)
	if !ok || got != 90*time.Second {
		t.Fatalf("default = %v, %v", got, ok)
	}
}

// A wall-clock deadline is not a fixed duration, so the window is computed per
// request. This is the shape an internal system re-confirming after every
// midnight needs: the proof must postdate the boundary, so the window is the
// time elapsed since it.
func TestDynamicRequirementComputesItsWindow(t *testing.T) {
	config := baseConfig(ModeOIDCOnly)
	request := httptest.NewRequest("GET", "/", nil)
	sinceMidnight := func(*http.Request) time.Duration {
		now := time.Date(2026, 8, 1, 9, 30, 0, 0, time.UTC)
		midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return now.Sub(midnight)
	}
	got, ok := Dynamic(sinceMidnight).maxAge(request, config)
	if !ok || got != 9*time.Hour+30*time.Minute {
		t.Fatalf("dynamic window = %v, %v", got, ok)
	}
	if _, ok := Dynamic(nil).maxAge(request, config); ok {
		t.Fatal("a nil resolver produced a window")
	}
	if _, ok := Dynamic(func(*http.Request) time.Duration { return -time.Second }).maxAge(request, config); ok {
		t.Fatal("a negative window was accepted")
	}
}

// stepUpAdmitted is what a zero window rests on, because elapsed time can never
// satisfy one: the redirect to the provider and back always costs more than
// zero seconds.
func TestAZeroWindowIsSatisfiedOnlyByACompletedStepUp(t *testing.T) {
	if stepUpAdmitted(SessionData{}) {
		t.Fatal("a session with no step-up was admitted")
	}
	if !stepUpAdmitted(SessionData{StepUpAt: time.Now().Unix()}) {
		t.Fatal("a just-completed step-up was refused, which would loop forever")
	}
	stale := time.Now().Add(-stepUpAdmissionWindow - time.Second).Unix()
	if stepUpAdmitted(SessionData{StepUpAt: stale}) {
		t.Fatal("a step-up older than the admission window was admitted")
	}
}

// Freshness is measured from what the provider reported, not from when the
// login landed: a provider may satisfy an authorization request from a single
// sign-on session established much earlier.
func TestFreshnessPrefersTheProviderProofTime(t *testing.T) {
	arrival := time.Now()
	proved := arrival.Add(-8 * time.Hour)
	data := SessionData{ProviderAuthTime: proved.Unix()}
	if got := data.provenAt(arrival); got.Unix() != proved.Unix() {
		t.Fatalf("provenAt = %v, want the reported %v", got, proved)
	}
	if got := (SessionData{}).provenAt(arrival); !got.Equal(arrival) {
		t.Fatalf("provenAt with no provider time = %v, want the fallback %v", got, arrival)
	}
}
