package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shibukawa/popcornweb/contrib/passkey"
	"github.com/shibukawa/popcornweb/pwruntime"
)

// passkeyStateNamespace isolates ceremony records from the OIDC correlation
// records that share the auth state table.
const passkeyStateNamespace = "auth-passkey"

const (
	// ceremonyCookieSuffix names the short-lived cookie holding the opaque key
	// of a pending ceremony. The challenge itself never reaches the browser, so
	// a captured response cannot be replayed against another ceremony.
	ceremonyCookieSuffix = "_pkc"
	ceremonyCookieMaxAge = 300
	// maxCeremonyBody bounds the JSON a browser posts back.
	maxCeremonyBody = 64 << 10
	// userHandleBytes is the length of a generated passkey user handle. It is
	// random rather than derived, so it reveals nothing about the account.
	userHandleBytes = 32
)

// Ceremony endpoint suffixes. They hang off auth.passkey.path so one setting
// keeps all of them consistent and reachable.
const (
	loginBeginSuffix     = "/login/begin"
	loginFinishSuffix    = "/login/finish"
	registerBeginSuffix  = "/register/begin"
	registerFinishSuffix = "/register/finish"
	bootstrapSuffix      = "/bootstrap"
)

// passkeyPaths returns the mounted ceremony paths of this configuration.
func (c Config) passkeyPaths() map[string]string {
	if !c.usesPasskey() {
		return nil
	}
	base := strings.TrimSuffix(c.Passkey.Path, "/")
	paths := map[string]string{
		base + loginBeginSuffix:     loginBeginSuffix,
		base + loginFinishSuffix:    loginFinishSuffix,
		base + registerBeginSuffix:  registerBeginSuffix,
		base + registerFinishSuffix: registerFinishSuffix,
	}
	if c.Mode == ModePasskeyOnly {
		paths[base+bootstrapSuffix] = bootstrapSuffix
	}
	return paths
}

// newRelyingParty builds the WebAuthn relying party of this configuration.
func newRelyingParty(config PasskeyConfig) (*passkey.RelyingParty, error) {
	return passkey.New(passkey.Config{
		RPID:    config.RPID,
		RPName:  config.RPName,
		Origins: config.Origins,
		// Configuration validation already refused a non-loopback http origin,
		// so the relying party only has to accept the ones that survived.
		AllowLoopbackHTTP: true,
	})
}

// setupPasskey builds the relying party, the ceremony state store, and the
// credential stores. It runs only in a mode that mounts the endpoints, so an
// oidc_only deployment carries none of it.
func (rt *runtime) setupPasskey(ctx context.Context) error {
	relyingParty, err := newRelyingParty(rt.config.Passkey)
	if err != nil {
		return fmt.Errorf("auth.passkey: %w", err)
	}
	stateStore, err := openState(ctx, rt, passkeyStateNamespace, passkey.CeremonyStateCodec{})
	if err != nil {
		return fmt.Errorf("passkey ceremony state: %w", err)
	}
	flow, err := passkey.NewSessionFlow(relyingParty, stateStore)
	if err != nil {
		return err
	}
	rt.passkeyFlow = flow
	rt.credentials = rt.backend.Credentials
	rt.bootstrap = rt.backend.Bootstrap
	if rt.credentials == nil || rt.bootstrap == nil {
		return fmt.Errorf("auth.backend = %q supplies no passkey credential storage", rt.config.backendName())
	}
	if rt.config.Mode != ModePasskeyOnly {
		// Only passkey_only redeems a bootstrap credential, so no other mode
		// carries the ticket store or the endpoint that fills it.
		return nil
	}
	enrollment, err := openState(ctx, rt, enrollmentStateNamespace, enrollmentTicketCodec{})
	if err != nil {
		return fmt.Errorf("enrollment ticket: %w", err)
	}
	rt.enrollment = enrollment
	return nil
}

// handlePasskey dispatches one ceremony endpoint. suffix names which one.
func (rt *runtime) handlePasskey(x Exchange, suffix string) {
	if !allowMethod(x, http.MethodPost) {
		return
	}
	// A ceremony changes state, so it is refused cross-origin. The JSON content
	// type a browser must send is itself unreachable from a simple form post.
	if !rt.sameOrigin(x) {
		x.Problem(pwruntime.Forbidden())
		return
	}
	switch suffix {
	case loginBeginSuffix:
		rt.handlePasskeyLoginBegin(x)
	case loginFinishSuffix:
		rt.handlePasskeyLoginFinish(x)
	case registerBeginSuffix:
		rt.handlePasskeyRegisterBegin(x)
	case registerFinishSuffix:
		rt.handlePasskeyRegisterFinish(x)
	case bootstrapSuffix:
		rt.handleBootstrap(x)
	default:
		x.Problem(pwruntime.NotFound())
	}
}

func (rt *runtime) handlePasskeyLoginBegin(x Exchange) {
	options, key, err := rt.passkeyFlow.BeginAuthentication(x.Context(), nil, passkey.AuthenticationOptions{
		RequireUserVerification: rt.config.Passkey.UserVerification == UserVerificationRequired,
	})
	if err != nil {
		logger(x).Log(x.Context(), pwruntime.LevelError, "passkey authentication could not start", pwruntime.Err(err))
		x.Problem(pwruntime.ServiceUnavailable())
		return
	}
	rt.writeCeremonyCookie(x, key)
	writeCeremonyJSON(x, options)
}

func (rt *runtime) handlePasskeyLoginFinish(x Exchange) {
	key, ok := rt.takeCeremonyCookie(x)
	if !ok {
		x.Problem(pwruntime.BadRequest())
		return
	}
	var response passkey.AuthenticationCredential
	if !decodeCeremonyJSON(x, &response) {
		return
	}
	credentialID, err := base64.RawURLEncoding.DecodeString(response.RawID)
	if err != nil || len(credentialID) == 0 {
		x.Problem(pwruntime.BadRequest())
		return
	}

	// One response shape covers an unknown credential, a rejected assertion,
	// and an unusable account, so a failed login reveals nothing about which.
	stored, err := rt.credentials.Find(x.Context(), credentialID)
	if err != nil {
		rt.refuseCeremony(x, "passkey credential lookup failed", err, ErrUnknownCredential)
		return
	}
	result, err := rt.passkeyFlow.FinishAuthentication(x.Context(), key, response, stored.record())
	if err != nil {
		rt.refuseCeremony(x, "passkey assertion rejected", err, nil)
		return
	}
	if result.CounterRisk {
		// The counter did not advance, which is what a cloned authenticator
		// looks like. An authenticator that keeps no counter reports zero on
		// both sides and never reaches here.
		logger(x).Log(x.Context(), pwruntime.LevelWarn, "passkey signature counter did not advance", pwruntime.String("account", stored.AccountID))
		x.Problem(pwruntime.Forbidden())
		return
	}
	now := time.Now()
	if err := rt.credentials.UpdateOnAssertion(x.Context(), credentialID, result.SignCount, result.BackupState, now); err != nil {
		logger(x).Log(x.Context(), pwruntime.LevelError, "passkey counter could not be persisted", pwruntime.Err(err))
		x.Problem(pwruntime.ServiceUnavailable())
		return
	}

	account, err := lookupAccount(x.Context(), stored.AccountID)
	if err != nil {
		rt.refuseCeremony(x, "passkey account lookup failed", err, ErrAccessDenied)
		return
	}
	if account.Suspended {
		x.Problem(pwruntime.Forbidden())
		return
	}
	// Rotation revokes whatever the browser held before this authentication.
	if err := rt.establish(x, SessionData{
		AccountID:   account.ID,
		DisplayName: account.DisplayName,
		Email:       account.Email,
	}, MethodPasskey); err != nil {
		logger(x).Log(x.Context(), pwruntime.LevelError, "session creation failed", pwruntime.Err(err))
		x.Problem(pwruntime.ServiceUnavailable())
		return
	}
	writeCeremonyJSON(x, map[string]string{"landing": rt.config.PostLoginPath})
}

// enroller names who is allowed to finish a registration: an authenticated
// account adding a method, or a redeemed bootstrap credential adding its first.
type enroller struct {
	accountID string
	label     string
	// ticket is set only for the restricted bootstrap path.
	ticket enrollmentTicket
	first  bool
}

// binding is what a registration ceremony begun by this enroller is tied to.
//
// It is compared at finish against the enroller resolved there, which is a
// different request and may be a different principal: the browser can log out,
// log in as someone else, or redeem a ticket in another tab while the
// authenticator is being touched. Without the comparison the ceremony adopts
// whoever is authenticated at that moment, and one account's authenticator is
// enrolled onto another's.
//
// The two paths are labelled apart so that a ticket-begun ceremony cannot be
// finished by an ordinary session of the same account, and the reverse. The
// value is not a secret; it is one side of an equality the server checks.
func (e enroller) binding() []byte {
	if e.first {
		return []byte("ticket\x00" + e.ticket.AccountID + "\x00" + e.ticket.LoginID)
	}
	return []byte("account\x00" + e.accountID)
}

// resolveEnroller admits an authenticated session with recent proof, and
// otherwise a restricted enrollment ticket. It never admits both at once: a
// ticket is the entry point of an account that has no session yet.
func (rt *runtime) resolveEnroller(x Exchange, consume bool) (enroller, bool) {
	if view, ok := Session(x.Context()); ok {
		// Adding a login method is an authentication-strength change, so it
		// needs proof that is recent rather than merely valid. Going through
		// the guard rather than comparing timestamps here gives the refusal a
		// way back: these are JSON endpoints, so the challenge names what was
		// missing instead of redirecting a fetch into a login page.
		if !IsRecentOn(x, Default()) {
			ChallengeOn(x, Default(), true)
			return enroller{}, false
		}
		return enroller{accountID: view.AccountID, label: displayOrAccount(view)}, true
	}
	if rt.enrollment != nil {
		take := rt.rotateEnrollmentTicket
		if consume {
			take = rt.takeEnrollmentTicket
		}
		if ticket, ok := take(x); ok {
			return enroller{accountID: ticket.AccountID, label: ticket.AccountID, ticket: ticket, first: true}, true
		}
	}
	x.Problem(pwruntime.Unauthorized())
	return enroller{}, false
}

func (rt *runtime) handlePasskeyRegisterBegin(x Exchange) {
	who, ok := rt.resolveEnroller(x, false)
	if !ok {
		return
	}
	existing, err := rt.credentials.ListByAccount(x.Context(), who.accountID)
	if err != nil {
		logger(x).Log(x.Context(), pwruntime.LevelError, "passkey credential listing failed", pwruntime.Err(err))
		x.Problem(pwruntime.ServiceUnavailable())
		return
	}
	handle, err := accountUserHandle(existing)
	if err != nil {
		x.Problem(pwruntime.InternalServerError(err))
		return
	}
	options, key, err := rt.passkeyFlow.BeginRegistration(x.Context(), passkey.User{
		ID:          handle,
		Name:        who.label,
		DisplayName: who.label,
	}, passkey.RegistrationOptions{
		RequireUserVerification: rt.config.Passkey.UserVerification == UserVerificationRequired,
		ResidentKey:             rt.config.Passkey.Discoverable,
		ExcludeCredentials:      descriptorsOf(existing),
		Binding:                 who.binding(),
	})
	if err != nil {
		logger(x).Log(x.Context(), pwruntime.LevelError, "passkey registration could not start", pwruntime.Err(err))
		x.Problem(pwruntime.ServiceUnavailable())
		return
	}
	rt.writeCeremonyCookie(x, key)
	writeCeremonyJSON(x, options)
}

func (rt *runtime) handlePasskeyRegisterFinish(x Exchange) {
	who, ok := rt.resolveEnroller(x, true)
	if !ok {
		return
	}
	key, ok := rt.takeCeremonyCookie(x)
	if !ok {
		x.Problem(pwruntime.BadRequest())
		return
	}
	var response passkey.RegistrationCredential
	if !decodeCeremonyJSON(x, &response) {
		return
	}
	// The binding is what ties this credential to the account that began the
	// ceremony rather than the one authenticated now. A mismatch is refused
	// like any other rejected ceremony.
	result, err := rt.passkeyFlow.FinishRegistration(x.Context(), key, who.binding(), response)
	if err != nil {
		rt.refuseCeremony(x, "passkey registration rejected", err, nil)
		return
	}
	credential := credentialFrom(result.Credential, who.accountID, time.Now())
	if who.first {
		// The first credential of a passkey_only account persists, activates,
		// and consumes the bootstrap credential as one unit, then trades the
		// restricted ticket for a normal session.
		if !rt.completeBootstrapEnrollment(x, who.ticket, credential) {
			return
		}
	} else {
		if err := rt.credentials.Save(x.Context(), credential, nil); err != nil {
			logger(x).Log(x.Context(), pwruntime.LevelError, "passkey credential could not be persisted", pwruntime.Err(err))
			x.Problem(pwruntime.ServiceUnavailable())
			return
		}
		// The account can now authenticate a new way, so the session it did
		// that from is replaced rather than carried forward.
		view, _ := Session(x.Context())
		if err := rt.establish(x, view, view.Method); err != nil {
			logger(x).Log(x.Context(), pwruntime.LevelError, "session rotation failed", pwruntime.Err(err))
			x.Problem(pwruntime.ServiceUnavailable())
			return
		}
	}
	writeCeremonyJSON(x, map[string]string{
		"credential_id": base64.RawURLEncoding.EncodeToString(credential.CredentialID),
	})
}

// refuseCeremony answers every ceremony failure the same way. expected names an
// error that is an ordinary refusal rather than a fault worth an error log.
func (rt *runtime) refuseCeremony(x Exchange, message string, err, expected error) {
	logger := logger(x)
	if expected != nil && errors.Is(err, expected) {
		logger.Log(x.Context(), pwruntime.LevelWarn, message)
	} else {
		// The error itself carries no challenge, credential ID, or handle;
		// contrib/passkey returns sentinel values only.
		logger.Log(x.Context(), pwruntime.LevelWarn, message, pwruntime.Err(err))
	}
	x.Problem(pwruntime.Forbidden())
}

// accountUserHandle reuses the handle the account already has, so every
// credential of one account shares one handle, and otherwise mints a random
// one. It is never derived from an email, a name, or a sequence.
func accountUserHandle(existing []Credential) ([]byte, error) {
	for _, credential := range existing {
		if len(credential.UserHandle) > 0 {
			return credential.UserHandle, nil
		}
	}
	handle := make([]byte, userHandleBytes)
	if _, err := io.ReadFull(rand.Reader, handle); err != nil {
		return nil, err
	}
	return handle, nil
}

func descriptorsOf(credentials []Credential) []passkey.CredentialDescriptor {
	descriptors := make([]passkey.CredentialDescriptor, 0, len(credentials))
	for _, credential := range credentials {
		descriptors = append(descriptors, passkey.CredentialDescriptor{
			Type:       "public-key",
			ID:         base64.RawURLEncoding.EncodeToString(credential.CredentialID),
			Transports: credential.Transports,
		})
	}
	return descriptors
}

func displayOrAccount(data SessionData) string {
	if data.DisplayName != "" {
		return data.DisplayName
	}
	return data.AccountID
}

// record converts a stored credential into what the relying party verifies.
func (c Credential) record() passkey.CredentialRecord {
	return passkey.CredentialRecord{
		ID:             c.CredentialID,
		UserHandle:     c.UserHandle,
		PublicKeyCOSE:  c.PublicKey,
		PublicKeyX:     c.PublicKeyX,
		PublicKeyY:     c.PublicKeyY,
		Algorithm:      c.Algorithm,
		SignCount:      c.SignCount,
		BackupEligible: c.BackupEligible,
		BackupState:    c.BackupState,
		Transports:     c.Transports,
	}
}

func credentialFrom(record passkey.CredentialRecord, accountID string, now time.Time) Credential {
	return Credential{
		CredentialID:   record.ID,
		AccountID:      accountID,
		UserHandle:     record.UserHandle,
		PublicKey:      record.PublicKeyCOSE,
		PublicKeyX:     record.PublicKeyX,
		PublicKeyY:     record.PublicKeyY,
		Algorithm:      record.Algorithm,
		SignCount:      record.SignCount,
		BackupEligible: record.BackupEligible,
		BackupState:    record.BackupState,
		Transports:     record.Transports,
		CreatedAt:      now,
	}
}

func (rt *runtime) ceremonyCookieName() string {
	return rt.manager.CookieName() + ceremonyCookieSuffix
}

// ceremonyCookiePath scopes the cookie to the ceremony endpoints, so it is not
// attached to ordinary application requests.
func (rt *runtime) ceremonyCookiePath() string {
	return strings.TrimSuffix(rt.config.Passkey.Path, "/")
}

func (rt *runtime) writeCeremonyCookie(x Exchange, key string) {
	x.SetCookie(&http.Cookie{
		Name:     rt.ceremonyCookieName(),
		Value:    key,
		Path:     rt.ceremonyCookiePath(),
		MaxAge:   ceremonyCookieMaxAge,
		Secure:   rt.cookieSecure(),
		HttpOnly: true,
		// A ceremony never leaves this origin, unlike the provider redirect.
		SameSite: http.SameSiteStrictMode,
	})
}

// takeCeremonyCookie reads and immediately expires the pending ceremony cookie,
// so one response can consume it only once.
func (rt *runtime) takeCeremonyCookie(x Exchange) (string, bool) {
	value, present := requestCookie(x, rt.ceremonyCookieName())
	x.SetCookie(&http.Cookie{
		Name:     rt.ceremonyCookieName(),
		Value:    "",
		Path:     rt.ceremonyCookiePath(),
		MaxAge:   -1,
		Secure:   rt.cookieSecure(),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	return value, present
}

func decodeCeremonyJSON(x Exchange, target any) bool {
	if contentType := x.Header("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		x.Problem(pwruntime.BadRequest())
		return false
	}
	body, err := x.Body(maxCeremonyBody)
	if err != nil {
		x.Problem(pwruntime.BadRequest())
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		x.Problem(pwruntime.BadRequest())
		return false
	}
	return true
}

func writeCeremonyJSON(x Exchange, payload any) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		x.Problem(pwruntime.InternalServerError(err))
		return
	}
	x.SetHeader("Content-Type", "application/json")
	// Ceremony options are single use, so nothing may keep a copy.
	x.SetHeader("Cache-Control", "no-store")
	x.Write(http.StatusOK, encoded)
}
