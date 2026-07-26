package session

import (
	"errors"
	"net/http"

	"github.com/shibukawa/popcornwave/pwruntime"
)

// UnavailableHandler responds to a request whose session backend could not be
// reached. flow:session-lifecycle fails closed here instead of silently
// downgrading the request to unauthenticated.
type UnavailableHandler func(http.ResponseWriter, *http.Request, error)

// Middleware resolves the session cookie into a validated request session.
//
// A missing, malformed, or expired cookie continues as an explicitly
// unauthenticated request with the cookie cleared. A backend failure is
// answered by unavailable without calling the next handler.
func (m *Manager[T]) Middleware(unavailable UnavailableHandler) func(http.Handler) http.Handler {
	if unavailable == nil {
		unavailable = defaultUnavailable
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			record, ok, err := m.resolve(w, r)
			switch {
			case err == nil && ok:
				view := record.view()
				ctx := pwruntime.WithSession(r.Context(), &view)
				ctx = pwruntime.WithAuthentication(ctx, m.authentication(record))
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			case err == nil,
				errors.Is(err, ErrNotFound),
				errors.Is(err, ErrExpired),
				errors.Is(err, ErrCodec),
				errors.Is(err, ErrInvalidKey):
				// Stale or unreadable browser state: clear it and continue as
				// an unauthenticated request.
				if err != nil {
					m.clearCookie(w)
				}
				next.ServeHTTP(w, r)
				return
			default:
				unavailable(w, r, err)
				return
			}
		})
	}
}

func (m *Manager[T]) authentication(record Record[T]) pwruntime.Authentication {
	subject := ""
	if m.options.Subject != nil {
		subject = m.options.Subject(record.Data)
	}
	return pwruntime.Authentication{
		Authenticated:   true,
		Subject:         subject,
		Method:          record.Method,
		Principal:       record.Data,
		AuthenticatedAt: record.AuthenticatedAt,
		ExpiresAt:       record.deadline(),
	}
}

func defaultUnavailable(w http.ResponseWriter, _ *http.Request, _ error) {
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
}
