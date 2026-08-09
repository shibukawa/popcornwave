package pwruntime

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	tinybind "github.com/shibukawa/tinybind-go"
)

// Problem is the application-facing RFC problem value.
//
// It lives here rather than in pw because both transport runtimes have to
// build, inspect, and unwrap the same type. Two structs that agree today are
// two chances to disagree later, and the failure is silent: an errors.As that
// stops matching, a problem that no longer unwraps. The module reached the same
// conclusion for its own error types and put them in a leaf both of its
// runtimes alias.
//
// This is not the module's Problem, which carries a code and a message and is
// the body its error constructors take. This one is the whole answer: the
// status a response gets, the title it shows, the fields that failed, and the
// cause that stays server-side.
type Problem struct {
	Status    int
	Title     string
	Code      string
	Message   string
	Fields    []FieldError
	Cause     error
	RateLimit *RateLimit
}

// RateLimit is the retry metadata attached to a 429 problem. X-RateLimit-*
// fields are compatibility conventions; Retry-After is the standard signal.
type RateLimit struct {
	Limit      uint64
	Remaining  uint64
	Reset      time.Time
	RetryAfter time.Duration
}

// Validate rejects metadata that would contradict itself on the wire.
func (r RateLimit) Validate() error {
	if r.Remaining > r.Limit {
		return fmt.Errorf("pwruntime: rate limit remaining %d exceeds limit %d", r.Remaining, r.Limit)
	}
	if r.RetryAfter < 0 {
		return fmt.Errorf("pwruntime: negative retry-after %s", r.RetryAfter)
	}
	return nil
}

// ApplyProblemHeaders writes response metadata before a problem response is
// committed. Invalid optional metadata is returned to the caller and omitted.
func ApplyProblemHeaders(header http.Header, problem Problem) error {
	fields, err := ProblemHeaders(problem)
	for name, values := range fields {
		header[name] = append([]string(nil), values...)
	}
	return err
}

// ProblemHeaders returns a detached set of response metadata for either HTTP
// transport runtime.
func ProblemHeaders(problem Problem) (http.Header, error) {
	header := make(http.Header)
	if problem.Status == http.StatusTooManyRequests {
		header.Set("Cache-Control", "no-store")
	}
	if problem.RateLimit == nil {
		return header, nil
	}
	if problem.Status != http.StatusTooManyRequests {
		return header, fmt.Errorf("pwruntime: rate limit metadata requires HTTP 429")
	}
	rate := *problem.RateLimit
	if err := rate.Validate(); err != nil {
		return header, err
	}
	header.Set("X-RateLimit-Limit", strconv.FormatUint(rate.Limit, 10))
	header.Set("X-RateLimit-Remaining", strconv.FormatUint(rate.Remaining, 10))
	if !rate.Reset.IsZero() {
		header.Set("X-RateLimit-Reset", strconv.FormatInt(rate.Reset.Unix(), 10))
	}
	if rate.RetryAfter > 0 {
		seconds := rate.RetryAfter / time.Second
		if rate.RetryAfter%time.Second != 0 {
			seconds++
		}
		header.Set("Retry-After", strconv.FormatInt(int64(seconds), 10))
	}
	return header, nil
}

// FieldError describes a single field-level validation failure. It is the
// module's own type, already shared by both of its runtimes.
type FieldError = tinybind.FieldError

// Field builds a field-level validation error for Validation.
func Field(field, location, message string) FieldError {
	return tinybind.Field(field, location, message)
}

func (p Problem) Error() string {
	if p.Message != "" {
		return p.Message
	}
	if p.Title != "" {
		return p.Title
	}
	return http.StatusText(p.Status)
}

func (p Problem) Unwrap() error { return p.Cause }

// NewProblem builds one from a status, a title, and whatever the caller passed:
// nothing, another problem to carry through, an error to wrap, or a message.
func NewProblem(status int, title string, value any) Problem {
	p := Problem{Status: status, Title: title, Code: strings.ReplaceAll(strings.ToLower(title), " ", "_")}
	switch value := value.(type) {
	case nil:
		p.Message = title
	case Problem:
		if value.Status == 0 {
			value.Status = status
		}
		if value.Title == "" {
			value.Title = title
		}
		return value
	case error:
		p.Message, p.Cause = value.Error(), value
	case string:
		p.Message = value
	default:
		p.Message = fmt.Sprint(value)
	}
	return p
}

// FirstValue is the variadic-to-optional shim the constructors share.
func FirstValue(values []any) any {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

func BadRequest(values ...any) Problem {
	return NewProblem(http.StatusBadRequest, "Bad Request", FirstValue(values))
}
func Unauthorized(values ...any) Problem {
	return NewProblem(http.StatusUnauthorized, "Unauthorized", FirstValue(values))
}
func Forbidden(values ...any) Problem {
	return NewProblem(http.StatusForbidden, "Forbidden", FirstValue(values))
}
func NotFound(values ...any) Problem {
	return NewProblem(http.StatusNotFound, "Not Found", FirstValue(values))
}
func Conflict(values ...any) Problem {
	return NewProblem(http.StatusConflict, "Conflict", FirstValue(values))
}
func PayloadTooLarge(values ...any) Problem {
	return NewProblem(http.StatusRequestEntityTooLarge, "Payload Too Large", FirstValue(values))
}
func TooManyRequests(values ...any) Problem {
	p := NewProblem(http.StatusTooManyRequests, "Too Many Requests", FirstValue(values))
	p.Code = "rate_limit_exceeded"
	return p
}

// RateLimited builds a 429 problem carrying retry and quota metadata.
func RateLimited(rate RateLimit, values ...any) Problem {
	p := TooManyRequests(values...)
	copy := rate
	p.RateLimit = &copy
	return p
}
func ServiceUnavailable(values ...any) Problem {
	return NewProblem(http.StatusServiceUnavailable, "Service Unavailable", FirstValue(values))
}
func InternalServerError(values ...any) Problem {
	p := NewProblem(http.StatusInternalServerError, "Internal Server Error", FirstValue(values))
	p.Code = "internal"
	return p
}

// Validation reports a 400 response carrying every detected field failure.
func Validation(fields ...FieldError) Problem {
	p := NewProblem(http.StatusBadRequest, "Validation failed", nil)
	p.Fields = append([]FieldError(nil), fields...)
	return p
}
