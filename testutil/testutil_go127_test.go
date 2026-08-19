//go:build go1.27

package testutil

import (
	"testing"

	"github.com/shibukawa/popcornweb/internal/pwtestbridge"
)

// The methods delegate, so each assertion crosses the two spellings: what a
// method wrote, a function reads, and the reverse. A method wired to the wrong
// function would otherwise pass every test written in one spelling alone.
func TestTheConfigMethodsReachTheSameValuesTheFunctionsDo(t *testing.T) {
	config := &Config{values: pwtestbridge.Configs{}}

	config.Set(fixtureConfig{Name: "set", Labels: []string{"a"}})
	if got := Get[fixtureConfig](config).Name; got != "set" {
		t.Errorf("Get = %q, want set; the method wrote somewhere the function does not read", got)
	}

	config.Update(func(value *fixtureConfig) { value.Name = "updated" })
	if got := config.Get[fixtureConfig]().Name; got != "updated" {
		t.Errorf("Get = %q, want updated", got)
	}
	if got := config.Get[fixtureConfig]().Labels; len(got) != 1 || got[0] != "a" {
		t.Errorf("Labels = %v, want [a]; Update replaced the value rather than editing it", got)
	}

	Set(config, fixtureConfig{Name: "function"})
	if got := config.Get[fixtureConfig]().Name; got != "function" {
		t.Errorf("Get = %q, want function; the method does not read what the function wrote", got)
	}
}

// Every operation copies, which is what keeps one test's edit out of the next
// test's configuration. The methods have to inherit that rather than hand out
// the stored value.
func TestTheConfigMethodsHandBackACopy(t *testing.T) {
	config := &Config{values: pwtestbridge.Configs{}}
	config.Set(fixtureConfig{Name: "stored", Labels: []string{"a"}})

	got := config.Get[fixtureConfig]()
	got.Labels[0] = "edited"

	if stored := config.Get[fixtureConfig]().Labels[0]; stored != "a" {
		t.Errorf("the stored value became %q; Get handed back the slice it holds", stored)
	}
}

// A nil configuration is what a caller has before TestRun builds one. The
// function answers a zero value rather than panicking, and the method is called
// on the same nil pointer.
func TestTheConfigGetMethodAnswersOnANilConfig(t *testing.T) {
	var config *Config
	if got := config.Get[fixtureConfig]().Name; got != "" {
		t.Errorf("Get = %q, want the zero value", got)
	}
}
