package trace

import (
	"context"
	"testing"

	"github.com/shibukawa/popcornwave/contrib/otel"
)

func TestAlwaysOffRecordsNothingAndStillPropagates(t *testing.T) {
	collector := &spanCollector{}
	tracer := NewProvider(collector, WithSampler(AlwaysOff())).Tracer("test-scope")
	ctx, span := tracer.Start(context.Background(), "request")
	if span.IsRecording() {
		t.Fatal("an unsampled span reports itself as recording")
	}
	span.SetAttributes(otel.String("url.path", "/"))
	span.AddEvent("event")
	span.SetStatus(StatusError, "failed")
	span.End()
	if len(collector.spans) != 0 {
		t.Fatalf("exported spans = %d, want none", len(collector.spans))
	}
	sc := SpanContextFromContext(ctx)
	if !sc.IsValid() {
		t.Fatal("an unsampled span has no valid span context to propagate")
	}
	if sc.TraceFlags()&flagSampled != 0 {
		t.Fatalf("trace flags = %02x, want the sampled bit clear", sc.TraceFlags())
	}
}

func TestAlwaysOnSetsTheSampledFlag(t *testing.T) {
	collector := &spanCollector{}
	tracer := NewProvider(collector, WithSampler(AlwaysOn())).Tracer("test-scope")
	_, span := tracer.Start(context.Background(), "request")
	if !span.IsRecording() {
		t.Fatal("a sampled span does not report itself as recording")
	}
	span.End()
	if len(collector.spans) != 1 {
		t.Fatalf("exported spans = %d, want 1", len(collector.spans))
	}
	if got := collector.spans[0].SpanContext.TraceFlags(); got&flagSampled == 0 {
		t.Fatalf("trace flags = %02x, want the sampled bit set", got)
	}
}

func TestDefaultSamplerRecordsEverything(t *testing.T) {
	collector := &spanCollector{}
	tracer := NewProvider(collector).Tracer("test-scope")
	_, span := tracer.Start(context.Background(), "request")
	span.End()
	if len(collector.spans) != 1 {
		t.Fatalf("exported spans = %d, want 1", len(collector.spans))
	}
}

func TestUnsampledSpanKeepsItsParentChain(t *testing.T) {
	tracer := NewProvider(&spanCollector{}, WithSampler(AlwaysOff())).Tracer("test-scope")
	ctx, root := tracer.Start(context.Background(), "root")
	ctx, middle := Start(ctx, "middle")
	_, leaf := Start(ctx, "leaf")
	if leaf.Parent() != middle || middle.Parent() != root {
		t.Fatal("an unsampled span lost its parent pointer")
	}
	if leaf.Root() != root {
		t.Fatal("Root does not reach the outermost unsampled span")
	}
}

func TestChildNeverDisagreesWithItsRoot(t *testing.T) {
	for _, sampler := range []Sampler{AlwaysOn(), AlwaysOff(), TraceIDRatioBased(0.5)} {
		collector := &spanCollector{}
		tracer := NewProvider(collector, WithSampler(ParentBased(sampler))).Tracer("test-scope")
		// Enough roots that a ratio sampler produces both decisions.
		for i := 0; i < 64; i++ {
			ctx, root := tracer.Start(context.Background(), "root")
			ctx, middle := Start(ctx, "middle")
			_, leaf := Start(ctx, "leaf")
			if root.IsRecording() != middle.IsRecording() || middle.IsRecording() != leaf.IsRecording() {
				t.Fatalf("%s: a descendant disagreed with its root", sampler.Description())
			}
			sampled := root.SpanContext().TraceFlags() & flagSampled
			if got := leaf.SpanContext().TraceFlags() & flagSampled; got != sampled {
				t.Fatalf("%s: leaf sampled bit = %02x, root = %02x", sampler.Description(), got, sampled)
			}
			leaf.End()
			middle.End()
			root.End()
		}
	}
}

func TestRemoteParentDecidesUnderParentBased(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		flags      byte
		root       Sampler
		wantRecord bool
	}{
		{name: "sampled parent overrides an always_off root", flags: flagSampled, root: AlwaysOff(), wantRecord: true},
		{name: "unsampled parent overrides an always_on root", flags: 0, root: AlwaysOn(), wantRecord: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			remote, err := NewSpanContext("0af7651916cd43dd8448eb211c80319c", "b7ad6b7169203331", testCase.flags, "", true)
			if err != nil {
				t.Fatal(err)
			}
			collector := &spanCollector{}
			provider := NewProvider(collector, WithSampler(ParentBased(testCase.root)))
			ctx := ContextWithSpanContext(context.Background(), remote)
			_, span := provider.Tracer("test-scope").Start(ctx, "request")
			span.End()
			if got := len(collector.spans) == 1; got != testCase.wantRecord {
				t.Fatalf("recorded = %v, want %v", got, testCase.wantRecord)
			}
			if got := span.SpanContext().TraceFlags() & flagSampled; (got != 0) != testCase.wantRecord {
				t.Fatalf("outbound sampled bit = %02x, want recorded = %v", got, testCase.wantRecord)
			}
		})
	}
}

func TestRatioSamplerIsDeterministicForOneTraceID(t *testing.T) {
	sampler := TraceIDRatioBased(0.5)
	other := TraceIDRatioBased(0.5)
	var decided, kept int
	for i := 0; i < 512; i++ {
		var traceID [16]byte
		fillID(traceID[:])
		parameters := SamplingParameters{TraceID: traceID}
		first := sampler.ShouldSample(parameters)
		if first != other.ShouldSample(parameters) {
			t.Fatal("two samplers at one ratio disagreed about one trace ID")
		}
		if first != sampler.ShouldSample(parameters) {
			t.Fatal("one sampler gave two answers for one trace ID")
		}
		decided++
		if first {
			kept++
		}
	}
	// A determinism test, not a distribution test: the bounds are wide enough
	// that only a sampler ignoring the ratio entirely fails them.
	if kept == 0 || kept == decided {
		t.Fatalf("kept %d of %d, which is not a fraction", kept, decided)
	}
}

func TestRatioBoundsCollapseToTheConstantSamplers(t *testing.T) {
	if got := TraceIDRatioBased(0).Description(); got != SamplerAlwaysOff {
		t.Fatalf("ratio 0 = %q", got)
	}
	if got := TraceIDRatioBased(1).Description(); got != SamplerAlwaysOn {
		t.Fatalf("ratio 1 = %q", got)
	}
	if got := TraceIDRatioBased(-1).Description(); got != SamplerAlwaysOff {
		t.Fatalf("negative ratio = %q", got)
	}
}

func TestParseSampler(t *testing.T) {
	for _, testCase := range []struct {
		name, argument, want string
	}{
		{name: "always_on", want: "always_on"},
		{name: "ALWAYS_OFF", want: "always_off"},
		{name: " traceidratio ", argument: "0.25", want: "traceidratio{0.25}"},
		{name: "traceidratio", argument: "", want: "always_on"},
		{name: "parentbased_always_on", want: "parentbased_always_on"},
		{name: "parentbased_always_off", want: "parentbased_always_off"},
		{name: "parentbased_traceidratio", argument: "0.1", want: "parentbased_traceidratio{0.1}"},
	} {
		sampler, err := ParseSampler(testCase.name, testCase.argument)
		if err != nil {
			t.Fatalf("%s: %v", testCase.name, err)
		}
		if got := sampler.Description(); got != testCase.want {
			t.Fatalf("%s: description = %q, want %q", testCase.name, got, testCase.want)
		}
	}
}

func TestParseSamplerRejectsRatherThanFallingBack(t *testing.T) {
	for _, testCase := range []struct{ name, argument string }{
		{name: "", argument: ""},
		{name: "jaeger_remote", argument: ""},
		{name: "traceidratio", argument: "half"},
		{name: "traceidratio", argument: "2"},
		{name: "parentbased_traceidratio", argument: "-1"},
	} {
		if _, err := ParseSampler(testCase.name, testCase.argument); err == nil {
			t.Fatalf("%q/%q was accepted", testCase.name, testCase.argument)
		}
	}
}
