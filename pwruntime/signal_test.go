package pwruntime_test

import (
	"bytes"
	"errors"
	"iter"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/pwruntime"
)

// A live source is written once and compiled into both builds, so everything it
// needs has to be reachable without naming a transport. That is the whole reason
// the constructors live in this package, and it is a property of where the API
// is rather than of what it computes — which makes it invisible to any test that
// imports pw, and invisible to a net/http build, because both of those find the
// same functions under the other name and pass.
//
// This file is the guard. It is an external test package importing pwruntime and
// nothing else, so moving the construction half back into pw does not produce a
// wrong answer here: it stops compiling.
//
// The regression it exists for shipped once. Signals arrived in pw alone, an
// example's source reached for pw.NamedSignal from a file no build tag excludes,
// and the fasthttp build of that example linked the entire net/http runtime
// through one import in one file that had nothing to do with either transport.

// finished is a payload. An application generates this method rather than
// writing it, which is what keeps a signal encoded by the same codec as every
// other typed value the framework sends; hand-written here so the test needs no
// generation step.
type finished struct{ URL string }

func (f finished) AppendJSON(dst []byte) []byte {
	return append(append(append(dst, `{"url":"`...), f.URL...), `"}`...)
}

// watchJob is the shape the guide documents: a sequence that yields values, then
// yields an instruction in the error slot and stops. Written here as an
// application would write it, against this package alone.
func watchJob(steps int) iter.Seq2[int, error] {
	return func(yield func(int, error) bool) {
		for step := range steps {
			if !yield(step, nil) {
				return
			}
		}
		yield(0, pwruntime.NewSignal("app.finished", finished{URL: "/done"}))
	}
}

func TestASourceProducesAndClassifiesASignalWithoutNamingATransport(t *testing.T) {
	var values []int
	var signal pwruntime.Signal
	var found bool
	for value, err := range watchJob(3) {
		if err == nil {
			values = append(values, value)
			continue
		}
		// The classification an application needs when it wraps a source of its
		// own, and the one both live loops make before any failure path runs.
		if signal, found = pwruntime.AsSignal(err); !found {
			t.Fatalf("the error slot carried something that is not a signal: %v", err)
		}
		if !errors.Is(err, pwruntime.ErrSignal) {
			t.Error("a signal does not match ErrSignal, so code classifying by errors.Is misses it")
		}
	}
	if len(values) != 3 {
		t.Errorf("values = %v, so the signal ended the sequence early", values)
	}
	if !found {
		t.Fatal("the source yielded no signal")
	}
	if signal.Name() != "app.finished" {
		t.Errorf("name = %q", signal.Name())
	}

	// And the record the client reads is written from here too, so the produce
	// side and the write side are one package: a backend linking this one has
	// everything it needs to carry a signal end to end.
	var buffer bytes.Buffer
	if _, err := pwruntime.WriteLiveSignal(&buffer, nil, signal); err != nil {
		t.Fatal(err)
	}
	record := buffer.String()
	for _, want := range []string{`"r":"signal"`, `"name":"app.finished"`, `/done`} {
		if !strings.Contains(record, want) {
			t.Errorf("the record does not carry %s:\n%s", want, record)
		}
	}
}

// NamedSignal is the no-payload constructor, which is what a source uses when
// the instruction is the whole message. It is named separately because it is the
// one the regression above went through.
func TestNamedSignalCarriesNoPayload(t *testing.T) {
	signal := pwruntime.NamedSignal("app.message")
	if signal.Name() != "app.message" {
		t.Errorf("name = %q", signal.Name())
	}
	if payload := signal.Payload(); len(payload) != 0 {
		t.Errorf("payload = %q, and a named signal carries none", payload)
	}
	var buffer bytes.Buffer
	if _, err := pwruntime.WriteLiveSignal(&buffer, nil, signal); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buffer.String(), `"data"`) {
		t.Errorf("the record carries a data field for a signal with no payload:\n%s", buffer.String())
	}
}

// The framework's own namespace is reserved in this package, so both live loops
// enforce one prefix rather than each enforcing its own.
func TestTheReservedPrefixIsDecidedHere(t *testing.T) {
	if !pwruntime.ReservedSignalName(pwruntime.ReservedSignalPrefix + "delivery_applied") {
		t.Error("a lifecycle name is not recognized as reserved, so an application could claim one")
	}
	if pwruntime.ReservedSignalName("app.finished") {
		t.Error("an application name was rejected as reserved")
	}
}
