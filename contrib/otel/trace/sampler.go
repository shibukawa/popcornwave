package trace

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// flagSampled is bit 0 of the W3C trace flags, the only bit the specification
// assigns. Every other bit is carried from the parent untouched.
const flagSampled byte = 0x01

// SamplingParameters is what a Sampler decides from.
type SamplingParameters struct {
	// Parent is the span context this span descends from, local or extracted
	// from a remote caller. It is the zero value for a root, which is the only
	// case a parent-based sampler consults its delegate about.
	Parent SpanContext
	// TraceID is the trace being decided, which for a child is the parent's.
	// A ratio sampler reads it instead of drawing a random number, so two
	// processes configured at one ratio agree about one trace.
	TraceID [16]byte
	// Name and Kind describe the span being started. Neither is read by any
	// sampler here; they are passed so a project's own sampler can.
	Name string
	Kind SpanKind
}

// Sampler decides whether a trace is recorded.
//
// The decision is taken once, at the root, and carried in the sampled bit of
// the trace flags. A child never decides again: it inherits, which is what
// makes a trace whole rather than a set of spans that each rolled dice.
//
// An unsampled span still has a valid span context, so propagation and log
// correlation keep working; it records no attribute and reaches no processor.
type Sampler interface {
	ShouldSample(SamplingParameters) bool
	// Description names the sampler for a diagnostic. It uses the same tokens
	// the configuration accepts, so what a process reports is what an operator
	// would have to write to reproduce it.
	Description() string
}

// Sampler names, which are the OTEL_TRACES_SAMPLER vocabulary.
const (
	SamplerAlwaysOn               = "always_on"
	SamplerAlwaysOff              = "always_off"
	SamplerTraceIDRatio           = "traceidratio"
	SamplerParentBasedAlwaysOn    = "parentbased_always_on"
	SamplerParentBasedAlwaysOff   = "parentbased_always_off"
	SamplerParentBasedTraceIDRate = "parentbased_traceidratio"
)

type alwaysOnSampler struct{}

func (alwaysOnSampler) ShouldSample(SamplingParameters) bool { return true }
func (alwaysOnSampler) Description() string                  { return SamplerAlwaysOn }

// AlwaysOn records every trace it is consulted about.
func AlwaysOn() Sampler { return alwaysOnSampler{} }

type alwaysOffSampler struct{}

func (alwaysOffSampler) ShouldSample(SamplingParameters) bool { return false }
func (alwaysOffSampler) Description() string                  { return SamplerAlwaysOff }

// AlwaysOff records nothing. Spans are still created and still propagate, which
// is what distinguishes it from turning tracing off.
func AlwaysOff() Sampler { return alwaysOffSampler{} }

type ratioSampler struct {
	ratio     float64
	threshold uint64
}

// TraceIDRatioBased records the given fraction of traces, deciding from the
// trace id.
//
// The decision is a comparison against the low 8 bytes rather than a random
// draw, because a ratio is only useful across services if two of them
// configured alike keep or drop the same trace. The top bit is discarded so the
// comparison is unsigned in the same way every other implementation's is.
func TraceIDRatioBased(ratio float64) Sampler {
	switch {
	case ratio <= 0:
		return AlwaysOff()
	case ratio >= 1:
		return AlwaysOn()
	}
	return ratioSampler{ratio: ratio, threshold: uint64(ratio * (1 << 63))}
}

func (s ratioSampler) ShouldSample(parameters SamplingParameters) bool {
	return binary.BigEndian.Uint64(parameters.TraceID[8:])>>1 < s.threshold
}

func (s ratioSampler) Description() string {
	return SamplerTraceIDRatio + "{" + strconv.FormatFloat(s.ratio, 'g', -1, 64) + "}"
}

type parentSampler struct{ root Sampler }

// ParentBased defers to a valid parent's decision and consults root only where
// there is none.
//
// It is the wrapper every deployed configuration wants: without it a ratio
// applied at each hop would keep a trace's middle and drop its ends, and an
// upstream service that decided to record would be truncated by a downstream
// service that decided not to.
func ParentBased(root Sampler) Sampler {
	if root == nil {
		root = AlwaysOn()
	}
	return parentSampler{root: root}
}

func (s parentSampler) ShouldSample(parameters SamplingParameters) bool {
	if parameters.Parent.IsValid() {
		return parameters.Parent.traceFlags&flagSampled != 0
	}
	return s.root.ShouldSample(parameters)
}

func (s parentSampler) Description() string { return "parentbased_" + s.root.Description() }

// ParseSampler builds a sampler from the OTEL_TRACES_SAMPLER name and argument.
//
// An unparseable argument is an error rather than a fallback to recording
// everything. The specification suggests warning and continuing, which would
// mean a mistyped ratio produces the most expensive possible behavior on the
// route that bills per span; a process that cannot honor its configuration
// stops at startup here, as every other observability key already does.
func ParseSampler(name, argument string) (Sampler, error) {
	argument = strings.TrimSpace(argument)
	switch strings.ToLower(strings.TrimSpace(name)) {
	case SamplerAlwaysOn:
		return AlwaysOn(), nil
	case SamplerAlwaysOff:
		return AlwaysOff(), nil
	case SamplerTraceIDRatio:
		ratio, err := parseRatio(argument)
		if err != nil {
			return nil, err
		}
		return TraceIDRatioBased(ratio), nil
	case SamplerParentBasedAlwaysOn:
		return ParentBased(AlwaysOn()), nil
	case SamplerParentBasedAlwaysOff:
		return ParentBased(AlwaysOff()), nil
	case SamplerParentBasedTraceIDRate:
		ratio, err := parseRatio(argument)
		if err != nil {
			return nil, err
		}
		return ParentBased(TraceIDRatioBased(ratio)), nil
	default:
		return nil, fmt.Errorf("must be %s, %s, %s, %s, %s, or %s",
			SamplerAlwaysOn, SamplerAlwaysOff, SamplerTraceIDRatio,
			SamplerParentBasedAlwaysOn, SamplerParentBasedAlwaysOff, SamplerParentBasedTraceIDRate)
	}
}

// parseRatio reads the sampler argument. An absent argument is 1.0, which is
// what the specification assigns, and is why a deployment that wants a fraction
// states one rather than relying on the name alone.
func parseRatio(argument string) (float64, error) {
	if argument == "" {
		return 1, nil
	}
	ratio, err := strconv.ParseFloat(argument, 64)
	if err != nil {
		return 0, errors.New("sampler argument must be a number between 0 and 1")
	}
	if ratio < 0 || ratio > 1 {
		return 0, errors.New("sampler argument must be between 0 and 1")
	}
	return ratio, nil
}
