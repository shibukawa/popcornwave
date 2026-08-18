package pwfast

import (
	"reflect"
	"testing"

	"github.com/shibukawa/popcornweb/pwruntime"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

type benchConfig struct {
	Name  string
	Limit int
}

// The request value is a context, which is what makes the accessors portable:
// a handler passing r where the other half passes r.Context() compiles on both
// sides, so a rewritten call needs no argument moved. That this test compiles
// at all is half of what it asserts.
func TestConfigReadsThePublishedResolutionThroughTheRequest(t *testing.T) {
	// Nothing published yet in this binary: pw is not linked into it, so this
	// is the state a build with no configuration-binding runtime is in.
	if _, _, body := serve(t, func(r *fasthttp.RequestCtx) {
		_, _ = r.WriteString(Config[benchConfig](r).Name)
	}, "/"); body != "" {
		t.Errorf("an unpublished configuration produced %q rather than the zero value", body)
	}

	resolved := &benchConfig{Name: "published", Limit: 7}
	previous := pwruntime.PublishConfigLookup(func(target reflect.Type) (any, bool) {
		if target == reflect.TypeFor[benchConfig]() {
			return resolved, true
		}
		return nil, false
	})
	// Restored rather than cleared: the process lookup belongs to whichever
	// runtime parsed the configuration, and leaving none behind would answer
	// every binding in every later test with its zero value.
	t.Cleanup(func() { pwruntime.PublishConfigLookup(previous) })

	_, _, body := serve(t, func(r *fasthttp.RequestCtx) {
		config := Config[benchConfig](r)
		_, _ = r.WriteString(config.Name + ":" + itoa(config.Limit))
	}, "/")
	if body != "published:7" {
		t.Errorf("body = %q, want %q", body, "published:7")
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}
