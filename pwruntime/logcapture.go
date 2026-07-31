package pwruntime

import (
	"context"
	"sync"
)

// CaptureSink collects records in memory instead of writing them anywhere.
//
// It exists so a test can assert on what was logged without parsing formatted
// output, which is the difference between checking a fact and checking an
// encoder. It is safe for concurrent use.
type CaptureSink struct {
	mu      sync.Mutex
	records []Record
}

// NewCaptureSink returns an empty sink.
func NewCaptureSink() *CaptureSink { return &CaptureSink{} }

func (sink *CaptureSink) Emit(_ context.Context, record Record) {
	sink.mu.Lock()
	sink.records = append(sink.records, record)
	sink.mu.Unlock()
}

// Records returns a copy of what has been captured so far.
func (sink *CaptureSink) Records() []Record {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	return append([]Record(nil), sink.records...)
}

// Reset discards everything captured so far.
func (sink *CaptureSink) Reset() {
	sink.mu.Lock()
	sink.records = nil
	sink.mu.Unlock()
}

// Lookup returns the attribute stored under key.
func (record Record) Lookup(key string) (Attribute, bool) {
	for _, attribute := range record.Attributes {
		if attribute.Key == key {
			return attribute, true
		}
	}
	return Attribute{}, false
}

// Text returns the string value stored under key, or the empty string when the
// key is absent or holds another kind.
func (record Record) Text(key string) string {
	attribute, ok := record.Lookup(key)
	if !ok {
		return ""
	}
	value, _ := attribute.Value.AsString()
	return value
}
