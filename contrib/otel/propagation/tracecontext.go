// Package propagation implements W3C Trace Context extraction and injection.
package propagation

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/shibukawa/popcornwave/contrib/otel/trace"
)

// TraceContext extracts and injects traceparent and tracestate HTTP fields.
type TraceContext struct{}

func (TraceContext) Extract(ctx context.Context, header http.Header) context.Context {
	parents := header.Values("traceparent")
	if len(parents) != 1 {
		return ctx
	}
	parent := parents[0]
	if len(parent) < 55 || parent[2] != '-' || parent[35] != '-' || parent[52] != '-' || parent[:2] == "ff" {
		return ctx
	}
	if parent[:2] == "00" && len(parent) != 55 {
		return ctx
	}
	if len(parent) > 55 && parent[55] != '-' {
		return ctx
	}
	if !lowerHex(parent[:2]) || !lowerHex(parent[3:35]) || !lowerHex(parent[36:52]) || !lowerHex(parent[53:55]) {
		return ctx
	}
	flags := fromHex(parent[53])<<4 | fromHex(parent[54])
	state := strings.Join(header.Values("tracestate"), ",")
	if !validTraceState(state) {
		state = ""
	}
	sc, err := trace.NewSpanContext(parent[3:35], parent[36:52], flags, state, true)
	if err != nil {
		return ctx
	}
	return trace.ContextWithSpanContext(ctx, sc)
}

func (TraceContext) Inject(ctx context.Context, header http.Header) {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return
	}
	header.Set("traceparent", fmt.Sprintf("00-%s-%s-%02x", sc.TraceID(), sc.SpanID(), sc.TraceFlags()))
	if sc.TraceState() == "" {
		header.Del("tracestate")
	} else {
		header.Set("tracestate", sc.TraceState())
	}
}

func lowerHex(value string) bool {
	for i := 0; i < len(value); i++ {
		if !((value[i] >= '0' && value[i] <= '9') || (value[i] >= 'a' && value[i] <= 'f')) {
			return false
		}
	}
	return true
}
func fromHex(value byte) byte {
	if value >= '0' && value <= '9' {
		return value - '0'
	}
	return value - 'a' + 10
}

func validTraceState(state string) bool {
	if state == "" {
		return true
	}
	if len(state) > 512 {
		return false
	}
	members := strings.Split(state, ",")
	if len(members) > 32 {
		return false
	}
	seen := make(map[string]struct{}, len(members))
	for _, member := range members {
		member = strings.TrimSpace(member)
		key, value, ok := strings.Cut(member, "=")
		if !ok || !validKey(key) || !validValue(value) {
			return false
		}
		if _, exists := seen[key]; exists {
			return false
		}
		seen[key] = struct{}{}
		for i := 0; i < len(member); i++ {
			if member[i] < 0x20 || member[i] > 0x7e || member[i] == ',' {
				return false
			}
		}
	}
	return true
}

func validKey(key string) bool {
	if len(key) == 0 || len(key) > 256 {
		return false
	}
	if strings.Count(key, "@") > 1 {
		return false
	}
	tenant, system, multi := strings.Cut(key, "@")
	if multi {
		if len(tenant) == 0 || len(tenant) > 241 || !lowerAlphaOrDigit(tenant[0]) || !keyPart(tenant) {
			return false
		}
		return len(system) > 0 && len(system) <= 14 && lowerAlpha(system[0]) && keyPart(system)
	}
	return lowerAlpha(key[0]) && keyPart(key)
}

func keyPart(value string) bool {
	for i := 0; i < len(value); i++ {
		b := value[i]
		if !lowerAlphaOrDigit(b) && b != '_' && b != '-' && b != '*' && b != '/' {
			return false
		}
	}
	return true
}

func lowerAlpha(value byte) bool { return value >= 'a' && value <= 'z' }
func lowerAlphaOrDigit(value byte) bool {
	return lowerAlpha(value) || (value >= '0' && value <= '9')
}

func validValue(value string) bool {
	if len(value) == 0 || len(value) > 256 || value[len(value)-1] == ' ' {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 || value[i] > 0x7e || value[i] == ',' || value[i] == '=' {
			return false
		}
	}
	return true
}
