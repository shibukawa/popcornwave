package pwconfig

import (
	"reflect"
	"strconv"
	"testing"
	"time"
)

// TestHTMLDefaultsMatchTags keeps defaultHTMLConfig, which seeds a runtime that
// parses no config source, in agreement with the default tags configbind reads.
// The two are separate declarations of one fact, so only a test stops them
// drifting.
func TestHTMLDefaultsMatchTags(t *testing.T) {
	typ := reflect.TypeFor[HTMLConfig]()
	value := reflect.ValueOf(defaultHTMLConfig)
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		tag, ok := field.Tag.Lookup("default")
		if !ok {
			continue
		}
		var seeded string
		switch actual := value.Field(index).Interface().(type) {
		case time.Duration:
			seeded = actual.String()
		case bool:
			seeded = strconv.FormatBool(actual)
		case int:
			seeded = strconv.Itoa(actual)
		default:
			t.Fatalf("%s: unhandled default kind %T", field.Name, actual)
		}
		if seeded != tag {
			t.Errorf("%s: default tag is %q but defaultHTMLConfig seeds %q", field.Name, tag, seeded)
		}
	}
}
