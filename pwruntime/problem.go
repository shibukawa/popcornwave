package pwruntime

import (
	"fmt"
	"net/http"
	"strings"

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
	Status  int
	Title   string
	Code    string
	Message string
	Fields  []FieldError
	Cause   error
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
