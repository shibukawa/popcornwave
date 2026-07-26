package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/pw"
)

// Cookie names. The transaction cookie only lives between the login redirect
// and the callback; the session cookie carries the signed identity afterwards.
const (
	sessionCookieName     = "pw_session"
	transactionCookieName = "pw_auth_tx"
)

// maxSessionCookieBytes bounds what a browser sends back, so a forged cookie
// cannot force expensive work before the signature is even checked.
const maxSessionCookieBytes = 4096

var errSession = errors.New("popcornwave/auth: unusable session cookie")

// sessionPayload is the signed content of the session cookie. The identity is
// small on purpose: anything larger belongs in the application's own store,
// keyed by subject.
type sessionPayload struct {
	Issuer  string            `json:"iss"`
	Subject string            `json:"sub"`
	Name    string            `json:"name,omitempty"`
	Email   string            `json:"email,omitempty"`
	Claims  map[string]string `json:"claims,omitempty"`
	// IDToken is kept for id_token_hint on RP-initiated logout, which is what
	// lets the provider end its own session instead of signing the user
	// straight back in. It never reaches a handler: pw.Identity omits it.
	IDToken   string `json:"idt,omitempty"`
	ExpiresAt int64  `json:"exp"`
}

// session is the decoded cookie: the identity handlers see, plus the logout
// material they do not.
type session struct {
	identity pw.Identity
	idToken  string
}

// sessionCodec signs and verifies the session cookie with the session secret.
// There is no server-side store: a development or single-process deployment
// needs none, and a shared store is the session backend's job.
type sessionCodec struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func newSessionCodec(config pw.SessionConfig) (*sessionCodec, error) {
	secret := strings.TrimSpace(config.Secret)
	if secret == "" {
		return nil, errors.New("popcornwave/auth: session.secret is required when authentication is enabled (SESSION_SECRET)")
	}
	ttl := config.TTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &sessionCodec{secret: []byte(secret), ttl: ttl, now: time.Now}, nil
}

func (codec *sessionCodec) encode(current session) (string, time.Time, error) {
	expires := codec.now().Add(codec.ttl)
	payload := sessionPayload{
		Issuer:    current.identity.Issuer,
		Subject:   current.identity.Subject,
		Name:      current.identity.Name,
		Email:     current.identity.Email,
		Claims:    current.identity.Claims,
		IDToken:   current.idToken,
		ExpiresAt: expires.Unix(),
	}
	value, err := codec.seal(payload)
	if err != nil {
		return "", time.Time{}, err
	}
	if len(value) > maxSessionCookieBytes {
		// A provider with large claims can push the cookie past what browsers
		// keep. Dropping the hint costs a weaker logout request, not the
		// session, so it beats failing the login.
		payload.IDToken = ""
		if value, err = codec.seal(payload); err != nil {
			return "", time.Time{}, err
		}
		if len(value) > maxSessionCookieBytes {
			return "", time.Time{}, errors.New("popcornwave/auth: the identity does not fit in a session cookie")
		}
	}
	return value, expires, nil
}

func (codec *sessionCodec) seal(payload sessionPayload) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(encoded)
	return body + "." + codec.sign(body), nil
}

func (codec *sessionCodec) decode(value string) (session, error) {
	if value == "" || len(value) > maxSessionCookieBytes {
		return session{}, errSession
	}
	body, signature, ok := strings.Cut(value, ".")
	if !ok || !hmac.Equal([]byte(signature), []byte(codec.sign(body))) {
		return session{}, errSession
	}
	encoded, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return session{}, errSession
	}
	var payload sessionPayload
	if err := json.Unmarshal(encoded, &payload); err != nil || payload.Subject == "" {
		return session{}, errSession
	}
	if codec.now().After(time.Unix(payload.ExpiresAt, 0)) {
		return session{}, errSession
	}
	return session{
		identity: pw.Identity{
			Subject: payload.Subject,
			Issuer:  payload.Issuer,
			Name:    payload.Name,
			Email:   payload.Email,
			Claims:  payload.Claims,
		},
		idToken: payload.IDToken,
	}, nil
}

func (codec *sessionCodec) sign(body string) string {
	mac := hmac.New(sha256.New, codec.secret)
	_, _ = mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// setCookie writes a hardened cookie. Secure is set from the request scheme so
// a loopback development server still works over HTTP.
func setCookie(w http.ResponseWriter, r *http.Request, name, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearCookie(w http.ResponseWriter, r *http.Request, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func cookieValue(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}
