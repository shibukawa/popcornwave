package session

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultCookieName is used when Options.Cookie.Name is empty.
	DefaultCookieName = "pw_session"
	maxTTL            = 365 * 24 * time.Hour
)

// CookieOptions is the browser cookie policy of a Manager or a Jar. Secure and
// HTTPOnly default to true: a zero value of either is upgraded rather than
// honored, because a session cookie readable by script or sent over plaintext
// is an omission far more often than a decision. The decision is written as
// AllowInsecure or ScriptReadable, which keep the corresponding false in place.
type CookieOptions struct {
	Name     string
	Path     string
	Domain   string
	Secure   bool
	HTTPOnly bool
	SameSite http.SameSite
	// AllowInsecure keeps Secure = false in place instead of defaulting it to
	// true. Only an explicit loopback development deployment should set it.
	AllowInsecure bool
	// ScriptReadable keeps HTTPOnly = false in place instead of defaulting it
	// to true, for a cookie page script is meant to read.
	ScriptReadable bool
}

// resolve applies the Secure and HTTPOnly defaults the opt-outs guard.
func (c CookieOptions) resolve() CookieOptions {
	if !c.AllowInsecure {
		c.Secure = true
	}
	if !c.ScriptReadable {
		c.HTTPOnly = true
	}
	return c
}

// normalizeCookie applies the cookie policy defaults shared by the session
// manager and Jar, and rejects a policy the browser would not honor safely.
// An empty defaultName makes the name required.
func normalizeCookie(cookie CookieOptions, defaultName string) (CookieOptions, http.SameSite, error) {
	cookie = cookie.resolve()
	if cookie.Name == "" {
		cookie.Name = defaultName
	}
	if !validCookieName(cookie.Name) {
		return CookieOptions{}, 0, fmt.Errorf("%w: cookie name", ErrInvalidOptions)
	}
	if cookie.Path == "" {
		cookie.Path = "/"
	}
	if !strings.HasPrefix(cookie.Path, "/") {
		return CookieOptions{}, 0, fmt.Errorf("%w: cookie path", ErrInvalidOptions)
	}
	sameSite := cookie.SameSite
	if sameSite == 0 {
		sameSite = http.SameSiteLaxMode
	}
	if sameSite == http.SameSiteNoneMode && !cookie.Secure {
		return CookieOptions{}, 0, fmt.Errorf("%w: insecure same-site none cookie", ErrInvalidOptions)
	}
	return cookie, sameSite, nil
}

func validCookieName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index := range len(value) {
		c := value[index]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}
