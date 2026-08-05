package pwdata

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// ErrUnsupportedParams is what a declared statement reports when a form cannot
// produce one of its arguments.
//
// The statement is still listed. A developer scanning for it should find it and
// read why it cannot be run here, rather than conclude that generation missed
// it or that the statement does not exist.
var ErrUnsupportedParams = errors.New(
	"this statement takes a parameter no form field can produce, so it cannot be run from here; " +
		"call it from a test or a handler instead")

// The argument converters generated registration code calls.
//
// A form field is text, and a declared statement takes the Go types its source
// declared. These turn one into the other and name the parameter when they
// cannot, because "invalid syntax" without a field name is a worse answer than
// the browser's own.
//
// They are here rather than in the generated file so that adding a supported
// type is a change to this package rather than to every project's output.

func argError(name, kind, value string, err error) error {
	return fmt.Errorf("%s (%s): %q is not a %s: %w", name, kind, value, kind, err)
}

// ArgString passes text through. It exists so that generated code calls a
// converter for every parameter rather than special-casing one.
func ArgString(_, value string) (string, error) { return value, nil }

func ArgInt(name, value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, argError(name, "int", value, err)
	}
	return parsed, nil
}

func ArgInt64(name, value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, argError(name, "int64", value, err)
	}
	return parsed, nil
}

func ArgInt32(name, value string) (int32, error) {
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, argError(name, "int32", value, err)
	}
	return int32(parsed), nil
}

func ArgFloat64(name, value string) (float64, error) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, argError(name, "float64", value, err)
	}
	return parsed, nil
}

func ArgFloat32(name, value string) (float32, error) {
	parsed, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return 0, argError(name, "float32", value, err)
	}
	return float32(parsed), nil
}

func ArgBool(name, value string) (bool, error) {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, argError(name, "bool", value, err)
	}
	return parsed, nil
}

// ArgTime accepts the forms a browser date or datetime field produces, and RFC
// 3339 for a value pasted from somewhere else.
func ArgTime(name, value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04", "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("%s (time.Time): %q is not a date or timestamp", name, value)
}

// SupportedArgKinds are the parameter types a form can produce a value for.
// A declared statement taking anything else is registered without a form and
// says so, rather than being left out of the list entirely.
var SupportedArgKinds = map[string]string{
	"string":    "ArgString",
	"int":       "ArgInt",
	"int64":     "ArgInt64",
	"int32":     "ArgInt32",
	"float64":   "ArgFloat64",
	"float32":   "ArgFloat32",
	"bool":      "ArgBool",
	"time.Time": "ArgTime",
}
