package pwobservability

import (
	"runtime"
	runtimemetrics "runtime/metrics"
	"sync"

	"github.com/shibukawa/popcornwave/contrib/otel"
	"github.com/shibukawa/popcornwave/contrib/otel/metric"
)

// runtimeMetricScope is a scope of its own rather than the framework's, because
// these values describe the Go runtime and not anything this framework did.
const runtimeMetricScope = "go.runtime"

// The runtime/metrics names this group reads. They are read through one Read
// call per collection rather than one per instrument, because that call is what
// costs and a single sample keeps every value consistent with the others.
const (
	sampleHeapObjects = "/memory/classes/heap/objects:bytes"
	sampleHeapFree    = "/memory/classes/heap/free:bytes"
	sampleStacks      = "/memory/classes/heap/stacks:bytes"
	sampleOtherMemory = "/memory/classes/other:bytes"
	sampleTotalMemory = "/memory/classes/total:bytes"
	sampleGCGoal      = "/gc/heap/goal:bytes"
	sampleAllocations = "/gc/heap/allocs:objects"
	sampleAllocated   = "/gc/heap/allocs:bytes"
	sampleGoroutines  = "/sched/goroutines:goroutines"
	sampleGOGC        = "/gc/gogc:percent"
)

// RegisterRuntimeMetrics registers the go.* instruments of the runtime group.
//
// Every one is an observable: the value already exists inside the runtime, and
// recording it as it changed would mean instrumenting the allocator. It is also
// the one group with no framework seam at all, which is why a deployment already
// collecting it from its own agent can decline it.
//
// The process samples of the development telemetry viewer are not these: that
// watches the process from outside and cannot see a heap, a goroutine, or a GC
// cycle. Neither is derived from the other.
func RegisterRuntimeMetrics(meter *metric.Meter) {
	if meter == nil {
		return
	}
	sampler := &runtimeSampler{samples: newRuntimeSamples()}
	meter.ObservableUpDownCounter("go.memory.used", "By", "Memory used by the Go runtime.", func() []metric.Observation {
		sampler.read()
		// The attribute splits the classes the specification names, so a reader
		// can total them or look at one.
		return []metric.Observation{
			{Attributes: memoryType("heap"), Value: sampler.value(sampleHeapObjects)},
			{Attributes: memoryType("stack"), Value: sampler.value(sampleStacks)},
			{Attributes: memoryType("other"), Value: sampler.value(sampleOtherMemory)},
			{Attributes: memoryType("free"), Value: sampler.value(sampleHeapFree)},
		}
	})
	meter.ObservableUpDownCounter("go.memory.limit", "By", "Total memory obtained from the OS.", func() []metric.Observation {
		sampler.read()
		return []metric.Observation{{Value: sampler.value(sampleTotalMemory)}}
	})
	meter.ObservableUpDownCounter("go.memory.gc.goal", "By", "Heap size target of the next GC cycle.", func() []metric.Observation {
		sampler.read()
		return []metric.Observation{{Value: sampler.value(sampleGCGoal)}}
	})
	meter.ObservableCounter("go.memory.allocated", "By", "Bytes allocated on the heap since process start.", func() []metric.Observation {
		sampler.read()
		return []metric.Observation{{Value: sampler.value(sampleAllocated)}}
	})
	meter.ObservableCounter("go.memory.allocations", "{allocation}", "Heap allocations since process start.", func() []metric.Observation {
		sampler.read()
		return []metric.Observation{{Value: sampler.value(sampleAllocations)}}
	})
	meter.ObservableUpDownCounter("go.goroutine.count", "{goroutine}", "Goroutines currently running.", func() []metric.Observation {
		sampler.read()
		return []metric.Observation{{Value: sampler.value(sampleGoroutines)}}
	})
	meter.ObservableUpDownCounter("go.config.gogc", "%", "The GOGC setting in force.", func() []metric.Observation {
		sampler.read()
		return []metric.Observation{{Value: sampler.value(sampleGOGC)}}
	})
	meter.ObservableUpDownCounter("go.processor.limit", "{thread}", "The GOMAXPROCS setting in force.", func() []metric.Observation {
		return []metric.Observation{{Value: float64(runtime.GOMAXPROCS(0))}}
	})
}

func memoryType(value string) []otel.Attribute {
	return []otel.Attribute{otel.String("go.memory.type", value)}
}

// runtimeSampler holds one runtime/metrics sample set.
//
// Each callback refreshes it before reading, so a collection performs one Read
// per instrument. That is a handful of reads per interval rather than per
// request, and runtime/metrics.Read walks a fixed slice with no syscall, so
// sharing one sample across the callbacks would buy an ordering constraint and
// nothing measurable.
type runtimeSampler struct {
	mu      sync.Mutex
	samples []runtimemetrics.Sample
	index   map[string]int
}

func newRuntimeSamples() []runtimemetrics.Sample {
	names := []string{
		sampleHeapObjects, sampleHeapFree, sampleStacks, sampleOtherMemory, sampleTotalMemory,
		sampleGCGoal, sampleAllocations, sampleAllocated, sampleGoroutines, sampleGOGC,
	}
	samples := make([]runtimemetrics.Sample, 0, len(names))
	for _, name := range names {
		samples = append(samples, runtimemetrics.Sample{Name: name})
	}
	return samples
}

// read refreshes the sample set.
func (s *runtimeSampler) read() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index == nil {
		s.index = make(map[string]int, len(s.samples))
		for position, sample := range s.samples {
			s.index[sample.Name] = position
		}
	}
	runtimemetrics.Read(s.samples)
}

func (s *runtimeSampler) value(name string) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	position, ok := s.index[name]
	if !ok {
		return 0
	}
	sample := s.samples[position]
	switch sample.Value.Kind() {
	case runtimemetrics.KindUint64:
		return float64(sample.Value.Uint64())
	case runtimemetrics.KindFloat64:
		return sample.Value.Float64()
	default:
		// A metric this Go version does not implement, or a histogram this group
		// does not read. Reporting zero would be a claim; the caller filters it.
		return 0
	}
}
