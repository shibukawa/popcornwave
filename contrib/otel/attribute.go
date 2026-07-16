// Package otel contains the small set of attribute value types shared by the
// Petitweb trace and log packages.
package otel

// ValueKind identifies the representation stored in a Value.
type ValueKind uint8

const (
	StringKind ValueKind = iota + 1
	BoolKind
	Int64Kind
	Float64Kind
)

// Value is an OpenTelemetry attribute value. Only scalar values are supported.
type Value struct {
	kind ValueKind
	s    string
	b    bool
	i    int64
	f    float64
}

// Kind returns the value's representation.
func (v Value) Kind() ValueKind { return v.kind }

// AsString returns a string value and whether the value has that kind.
func (v Value) AsString() (string, bool) { return v.s, v.kind == StringKind }

// AsBool returns a bool value and whether the value has that kind.
func (v Value) AsBool() (bool, bool) { return v.b, v.kind == BoolKind }

// AsInt64 returns an int64 value and whether the value has that kind.
func (v Value) AsInt64() (int64, bool) { return v.i, v.kind == Int64Kind }

// AsFloat64 returns a float64 value and whether the value has that kind.
func (v Value) AsFloat64() (float64, bool) { return v.f, v.kind == Float64Kind }

// Attribute is a key-value pair attached to a span or log record.
type Attribute struct {
	Key   string
	Value Value
}

func String(key, value string) Attribute {
	return Attribute{Key: key, Value: Value{kind: StringKind, s: value}}
}
func Bool(key string, value bool) Attribute {
	return Attribute{Key: key, Value: Value{kind: BoolKind, b: value}}
}
func Int64(key string, value int64) Attribute {
	return Attribute{Key: key, Value: Value{kind: Int64Kind, i: value}}
}
func Float64(key string, value float64) Attribute {
	return Attribute{Key: key, Value: Value{kind: Float64Kind, f: value}}
}
