package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/shibukawa/popcornwave/contrib/authstate"
	"github.com/shibukawa/popcornwave/pw"
)

// enrollmentStateNamespace isolates enrollment tickets from the OIDC and
// ceremony records that share the auth state table.
const enrollmentStateNamespace = "auth-enroll"

// enrollmentCookieSuffix names the cookie holding the opaque key of a restricted
// enrollment ticket.
const enrollmentCookieSuffix = "_enroll"

// enrollmentTicket is what a redeemed bootstrap credential grants.
//
// It is deliberately not a session. An account that has proved only a temporary
// secret is not logged in, so pw.Authenticated stays false and no application
// handler can mistake this for authority. Its single power is to finish one
// passkey registration.
type enrollmentTicket struct {
	AccountID string `json:"account_id"`
	LoginID   string `json:"login_id"`
	Purpose   string `json:"purpose"`
}

// enrollmentTicketCodec stores the ticket as bounded JSON. It carries no
// secret: the temporary secret is verified before the ticket exists and is
// never written anywhere.
type enrollmentTicketCodec struct{}

const maxEnrollmentTicketBytes = 4 << 10

func (enrollmentTicketCodec) Encode(value enrollmentTicket) ([]byte, error) {
	if value.AccountID == "" || value.LoginID == "" {
		return nil, authstate.ErrCodec
	}
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) > maxEnrollmentTicketBytes {
		return nil, authstate.ErrCodec
	}
	return encoded, nil
}

func (enrollmentTicketCodec) Decode(encoded []byte) (enrollmentTicket, error) {
	if len(encoded) == 0 || len(encoded) > maxEnrollmentTicketBytes {
		return enrollmentTicket{}, authstate.ErrCodec
	}
	var value enrollmentTicket
	if err := json.Unmarshal(encoded, &value); err != nil || value.AccountID == "" || value.LoginID == "" {
		return enrollmentTicket{}, authstate.ErrCodec
	}
	return value, nil
}

// AccountActivator activates a provisional account. It runs inside the
// transaction that persists the first passkey, so the account never becomes
// active without a credential and never gains a credential without becoming
// active.
type AccountActivator func(ctx context.Context, accountID string) error

var activatorState struct {
	sync.RWMutex
	activate AccountActivator
}

// SetAccountActivator installs the application activation step of
// flow:passkey-only-registration. Without one the framework persists the
// credential and consumes the bootstrap credential, and the application is
// responsible for whatever "active" means to it.
func SetAccountActivator(activate AccountActivator) {
	activatorState.Lock()
	defer activatorState.Unlock()
	activatorState.activate = activate
}

func accountActivator() AccountActivator {
	activatorState.RLock()
	defer activatorState.RUnlock()
	return activatorState.activate
}

// BootstrapSecretDigest is the digest a BootstrapStore persists. Issuing code
// calls it once with the generated secret and never stores the secret itself.
func BootstrapSecretDigest(loginID, secret string) []byte {
	// The login ID is mixed in, so a digest lifted from one row cannot be
	// replayed against another.
	sum := sha256.Sum256([]byte("popcornwave/bootstrap\x00" + loginID + "\x00" + secret))
	return sum[:]
}

// GenerateBootstrapSecret returns a high-entropy single-use secret. A
// deployment shows it once at issuance and stores only its digest. A
// user-chosen temporary password is not accepted anywhere, so this is the only
// way a secret comes into existence.
func GenerateBootstrapSecret() (string, error) { return randomToken() }

// newStateKey returns the opaque key of one stored record. It is unguessable,
// so possession of the cookie is the only way to reach the record.
func newStateKey() (string, error) { return randomToken() }

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

// IssueBootstrapCredential generates a single-use secret, stores only its
// digest, and returns the secret for the administrator to deliver out of band.
// The raw secret is returned exactly once and is never stored or logged.
//
// It is the framework side of flow:passkey-only-registration; deciding who may
// call it, and through which channel the secret travels, stays with the
// application.
func IssueBootstrapCredential(ctx context.Context, loginID, accountID, purpose string) (string, error) {
	rt := activeRuntime()
	if rt == nil || rt.bootstrap == nil {
		return "", errors.New("auth: bootstrap credentials need auth.mode passkey_only")
	}
	if loginID == "" || accountID == "" {
		return "", errors.New("auth: a bootstrap credential needs a login ID and an account")
	}
	switch purpose {
	case PurposeInitialPasskey, PurposeRecoveryPasskey:
	default:
		return "", fmt.Errorf("auth: bootstrap purpose must be %q or %q", PurposeInitialPasskey, PurposeRecoveryPasskey)
	}
	secret, err := GenerateBootstrapSecret()
	if err != nil {
		return "", err
	}
	now := time.Now()
	if err := rt.bootstrap.Issue(ctx, BootstrapCredential{
		LoginID:           loginID,
		AccountID:         accountID,
		SecretDigest:      BootstrapSecretDigest(loginID, secret),
		Purpose:           purpose,
		IssuedAt:          now,
		ExpiresAt:         now.Add(rt.config.Bootstrap.IssueTTL),
		AttemptsRemaining: rt.config.Bootstrap.MaxAttempts,
	}); err != nil {
		return "", err
	}
	return secret, nil
}

type bootstrapRequest struct {
	LoginID string `json:"login_id"`
	Secret  string `json:"secret"`
}

// handleBootstrap redeems a login ID and temporary secret for a restricted
// enrollment ticket. It never creates a normal session: the caller has proved
// possession of an issued secret, which authorizes one enrollment and nothing
// else.
func (rt *runtime) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	var request bootstrapRequest
	if !decodeCeremonyJSON(w, r, &request) {
		return
	}
	if request.LoginID == "" || request.Secret == "" {
		rt.refuseBootstrap(w, r, "empty bootstrap request")
		return
	}

	// The attempt is spent before the secret is checked, so a guess costs a
	// budget entry whether or not it was close.
	if _, err := rt.bootstrap.RecordAttempt(r.Context(), request.LoginID); err != nil {
		rt.refuseBootstrap(w, r, "bootstrap attempt refused")
		return
	}
	credential, err := rt.bootstrap.Find(r.Context(), request.LoginID)
	if err != nil {
		rt.refuseBootstrap(w, r, "unknown bootstrap credential")
		return
	}
	expected := BootstrapSecretDigest(request.LoginID, request.Secret)
	if subtle.ConstantTimeCompare(expected, credential.SecretDigest) != 1 {
		rt.refuseBootstrap(w, r, "bootstrap secret mismatch")
		return
	}
	if time.Now().After(credential.ExpiresAt) {
		rt.refuseBootstrap(w, r, "expired bootstrap credential")
		return
	}

	key, err := rt.putEnrollmentTicket(r.Context(), enrollmentTicket{
		AccountID: credential.AccountID,
		LoginID:   credential.LoginID,
		Purpose:   credential.Purpose,
	})
	if err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "enrollment ticket could not be stored", pw.Err(err))
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return
	}
	rt.writeEnrollmentCookie(w, key)
	writeCeremonyJSON(w, r, map[string]string{"next": rt.ceremonyCookiePath() + registerBeginSuffix})
}

// refuseBootstrap answers every redemption failure identically, so a caller
// cannot tell an unknown login ID from a wrong secret, an expired credential,
// or an exhausted attempt budget.
func (rt *runtime) refuseBootstrap(w http.ResponseWriter, r *http.Request, reason string) {
	// The reason is recorded without the login ID or the secret, so an audit
	// trail exists without a credential in the log.
	pw.Logger(r.Context()).Log(r.Context(), pw.LevelWarn, "bootstrap redemption refused", pw.String("reason", reason))
	pw.WriteProblem(w, r, pw.Forbidden())
}

func (rt *runtime) putEnrollmentTicket(ctx context.Context, ticket enrollmentTicket) (string, error) {
	key, err := newStateKey()
	if err != nil {
		return "", err
	}
	ttl := rt.config.Bootstrap.EnrollmentTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	if err := rt.enrollment.Put(ctx, key, ticket, time.Now().Add(ttl)); err != nil {
		return "", err
	}
	return key, nil
}

// readEnrollmentTicket consumes the ticket cookie and its record. A ticket is
// single use, so an abandoned enrollment cannot be resumed later.
func (rt *runtime) takeEnrollmentTicket(w http.ResponseWriter, r *http.Request) (enrollmentTicket, bool) {
	cookie, err := r.Cookie(rt.enrollmentCookieName())
	if err != nil || cookie == nil || cookie.Value == "" {
		return enrollmentTicket{}, false
	}
	ticket, err := rt.enrollment.Take(r.Context(), cookie.Value)
	rt.clearEnrollmentCookie(w)
	if err != nil {
		return enrollmentTicket{}, false
	}
	return ticket, true
}

// rotateEnrollmentTicket consumes the ticket and reissues it under a new key,
// which is what the begin half of the ceremony needs: it must read the account
// without spending the ticket, and the store offers only a consuming read.
//
// Rotation is the safer shape anyway: whatever key the browser held before this
// request is dead, exactly as a rotated session token is.
func (rt *runtime) rotateEnrollmentTicket(w http.ResponseWriter, r *http.Request) (enrollmentTicket, bool) {
	ticket, ok := rt.takeEnrollmentTicket(w, r)
	if !ok {
		return enrollmentTicket{}, false
	}
	key, err := rt.putEnrollmentTicket(r.Context(), ticket)
	if err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "enrollment ticket could not be reissued", pw.Err(err))
		return enrollmentTicket{}, false
	}
	rt.writeEnrollmentCookie(w, key)
	return ticket, true
}

func (rt *runtime) enrollmentCookieName() string {
	return rt.manager.CookieName() + enrollmentCookieSuffix
}

func (rt *runtime) writeEnrollmentCookie(w http.ResponseWriter, key string) {
	maxAge := int(rt.config.Bootstrap.EnrollmentTTL.Seconds())
	if maxAge <= 0 {
		maxAge = 600
	}
	http.SetCookie(w, &http.Cookie{
		Name:     rt.enrollmentCookieName(),
		Value:    key,
		Path:     rt.ceremonyCookiePath(),
		MaxAge:   maxAge,
		Secure:   rt.cookieSecure(),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func (rt *runtime) clearEnrollmentCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     rt.enrollmentCookieName(),
		Value:    "",
		Path:     rt.ceremonyCookiePath(),
		MaxAge:   -1,
		Secure:   rt.cookieSecure(),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// completeBootstrapEnrollment persists the first credential, activates the
// account, and consumes the bootstrap credential as one transaction, then
// replaces the restricted ticket with a normal session.
func (rt *runtime) completeBootstrapEnrollment(w http.ResponseWriter, r *http.Request, ticket enrollmentTicket, credential Credential) bool {
	activate := accountActivator()
	err := rt.credentials.Save(r.Context(), credential, func(txCtx context.Context) error {
		if activate != nil {
			if err := activate(txCtx, ticket.AccountID); err != nil {
				return fmt.Errorf("activate account: %w", err)
			}
		}
		return rt.bootstrap.Consume(txCtx, ticket.LoginID, time.Now())
	})
	if err != nil {
		// A partially applied enrollment would leave an account that looks
		// enrolled but cannot log in, so the whole unit fails.
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "first passkey enrollment failed", pw.Err(err))
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return false
	}

	account, err := lookupAccount(r.Context(), ticket.AccountID)
	if err != nil || account.Suspended {
		if err != nil && !errors.Is(err, ErrAccessDenied) {
			pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "account lookup failed", pw.Err(err))
		}
		pw.WriteProblem(w, r, pw.Forbidden())
		return false
	}
	if err := rt.manager.RotateWithMethod(w, r, SessionData{
		AccountID:   account.ID,
		DisplayName: account.DisplayName,
		Email:       account.Email,
	}, MethodPasskey); err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "session creation failed", pw.Err(err))
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return false
	}
	return true
}
