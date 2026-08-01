package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/authstate"
	"github.com/shibukawa/popcornwave/contrib/passkey"
	"github.com/shibukawa/popcornwave/pw"
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
func (rt *runtime) setupPasskey(ctx context.Context, db *sql.DB, dialect string) error {
	relyingParty, err := newRelyingParty(rt.config.Passkey)
	if err != nil {
		return fmt.Errorf("auth.passkey: %w", err)
	}
	stateStore, err := authstate.NewSQLStore[passkey.CeremonyState](db, passkey.CeremonyStateCodec{}, authstate.SQLOptions{
		Dialect:   dialect,
		Namespace: passkeyStateNamespace,
	})
	if err != nil {
		return err
	}
	// The table already exists, so this validates its column layout only.
	if err := stateStore.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("passkey ceremony state schema: %w", err)
	}
	flow, err := passkey.NewSessionFlow(relyingParty, stateStore)
	if err != nil {
		return err
	}
	rt.passkeyFlow = flow
	rt.credentials = installedCredentialStore()
	if rt.credentials == nil {
		rt.credentials = dbStore{db: db}
	}
	rt.bootstrap = installedBootstrapStore()
	if rt.bootstrap == nil {
		rt.bootstrap = bootstrapStore{db: db}
	}
	if rt.config.Mode != ModePasskeyOnly {
		// Only passkey_only redeems a bootstrap credential, so no other mode
		// carries the ticket store or the endpoint that fills it.
		return nil
	}
	enrollment, err := authstate.NewSQLStore[enrollmentTicket](db, enrollmentTicketCodec{}, authstate.SQLOptions{
		Dialect:   dialect,
		Namespace: enrollmentStateNamespace,
	})
	if err != nil {
		return err
	}
	if err := enrollment.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("enrollment ticket schema: %w", err)
	}
	rt.enrollment = enrollment
	return nil
}

// handlePasskey dispatches one ceremony endpoint. suffix names which one.
func (rt *runtime) handlePasskey(w http.ResponseWriter, r *http.Request, suffix string) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	// A ceremony changes state, so it is refused cross-origin. The JSON content
	// type a browser must send is itself unreachable from a simple form post.
	if !sameOrigin(r) {
		pw.WriteProblem(w, r, pw.Forbidden())
		return
	}
	switch suffix {
	case loginBeginSuffix:
		rt.handlePasskeyLoginBegin(w, r)
	case loginFinishSuffix:
		rt.handlePasskeyLoginFinish(w, r)
	case registerBeginSuffix:
		rt.handlePasskeyRegisterBegin(w, r)
	case registerFinishSuffix:
		rt.handlePasskeyRegisterFinish(w, r)
	case bootstrapSuffix:
		rt.handleBootstrap(w, r)
	default:
		pw.WriteProblem(w, r, pw.NotFound())
	}
}

func (rt *runtime) handlePasskeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	options, key, err := rt.passkeyFlow.BeginAuthentication(r.Context(), nil, passkey.AuthenticationOptions{
		RequireUserVerification: rt.config.Passkey.UserVerification == UserVerificationRequired,
	})
	if err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "passkey authentication could not start", pw.Err(err))
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return
	}
	rt.writeCeremonyCookie(w, key)
	writeCeremonyJSON(w, r, options)
}

func (rt *runtime) handlePasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	key, ok := rt.takeCeremonyCookie(w, r)
	if !ok {
		pw.WriteProblem(w, r, pw.BadRequest())
		return
	}
	var response passkey.AuthenticationCredential
	if !decodeCeremonyJSON(w, r, &response) {
		return
	}
	credentialID, err := base64.RawURLEncoding.DecodeString(response.RawID)
	if err != nil || len(credentialID) == 0 {
		pw.WriteProblem(w, r, pw.BadRequest())
		return
	}

	// One response shape covers an unknown credential, a rejected assertion,
	// and an unusable account, so a failed login reveals nothing about which.
	stored, err := rt.credentials.Find(r.Context(), credentialID)
	if err != nil {
		rt.refuseCeremony(w, r, "passkey credential lookup failed", err, ErrUnknownCredential)
		return
	}
	result, err := rt.passkeyFlow.FinishAuthentication(r.Context(), key, response, stored.record())
	if err != nil {
		rt.refuseCeremony(w, r, "passkey assertion rejected", err, nil)
		return
	}
	if result.CounterRisk {
		// The counter did not advance, which is what a cloned authenticator
		// looks like. An authenticator that keeps no counter reports zero on
		// both sides and never reaches here.
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelWarn, "passkey signature counter did not advance", pw.String("account", stored.AccountID))
		pw.WriteProblem(w, r, pw.Forbidden())
		return
	}
	now := time.Now()
	if err := rt.credentials.UpdateOnAssertion(r.Context(), credentialID, result.SignCount, result.BackupState, now); err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "passkey counter could not be persisted", pw.Err(err))
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return
	}

	account, err := lookupAccount(r.Context(), stored.AccountID)
	if err != nil {
		rt.refuseCeremony(w, r, "passkey account lookup failed", err, ErrAccessDenied)
		return
	}
	if account.Suspended {
		pw.WriteProblem(w, r, pw.Forbidden())
		return
	}
	// Rotation revokes whatever the browser held before this authentication.
	if err := rt.manager.RotateWithMethod(w, r, SessionData{
		AccountID:   account.ID,
		DisplayName: account.DisplayName,
		Email:       account.Email,
	}, MethodPasskey); err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "session creation failed", pw.Err(err))
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return
	}
	writeCeremonyJSON(w, r, map[string]string{"landing": rt.config.PostLoginPath})
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

// resolveEnroller admits an authenticated session with recent proof, and
// otherwise a restricted enrollment ticket. It never admits both at once: a
// ticket is the entry point of an account that has no session yet.
func (rt *runtime) resolveEnroller(w http.ResponseWriter, r *http.Request, consume bool) (enroller, bool) {
	if view, ok := Session(r.Context()); ok {
		// Adding a login method is an authentication-strength change, so it
		// needs proof that is recent rather than merely valid. Going through
		// the guard rather than comparing timestamps here gives the refusal a
		// way back: these are JSON endpoints, so the challenge names what was
		// missing instead of redirecting a fetch into a login page.
		if !IsRecent(r, Default()) {
			Challenge(w, r, Default(), true)
			return enroller{}, false
		}
		return enroller{accountID: view.Data.AccountID, label: displayOrAccount(view.Data)}, true
	}
	if rt.enrollment != nil {
		take := rt.rotateEnrollmentTicket
		if consume {
			take = rt.takeEnrollmentTicket
		}
		if ticket, ok := take(w, r); ok {
			return enroller{accountID: ticket.AccountID, label: ticket.AccountID, ticket: ticket, first: true}, true
		}
	}
	pw.WriteProblem(w, r, pw.Unauthorized())
	return enroller{}, false
}

func (rt *runtime) handlePasskeyRegisterBegin(w http.ResponseWriter, r *http.Request) {
	who, ok := rt.resolveEnroller(w, r, false)
	if !ok {
		return
	}
	existing, err := rt.credentials.ListByAccount(r.Context(), who.accountID)
	if err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "passkey credential listing failed", pw.Err(err))
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return
	}
	handle, err := accountUserHandle(existing)
	if err != nil {
		pw.WriteProblem(w, r, pw.InternalServerError(err))
		return
	}
	options, key, err := rt.passkeyFlow.BeginRegistration(r.Context(), passkey.User{
		ID:          handle,
		Name:        who.label,
		DisplayName: who.label,
	}, passkey.RegistrationOptions{
		RequireUserVerification: rt.config.Passkey.UserVerification == UserVerificationRequired,
		ResidentKey:             rt.config.Passkey.Discoverable,
		ExcludeCredentials:      descriptorsOf(existing),
	})
	if err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "passkey registration could not start", pw.Err(err))
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return
	}
	rt.writeCeremonyCookie(w, key)
	writeCeremonyJSON(w, r, options)
}

func (rt *runtime) handlePasskeyRegisterFinish(w http.ResponseWriter, r *http.Request) {
	who, ok := rt.resolveEnroller(w, r, true)
	if !ok {
		return
	}
	key, ok := rt.takeCeremonyCookie(w, r)
	if !ok {
		pw.WriteProblem(w, r, pw.BadRequest())
		return
	}
	var response passkey.RegistrationCredential
	if !decodeCeremonyJSON(w, r, &response) {
		return
	}
	result, err := rt.passkeyFlow.FinishRegistration(r.Context(), key, response)
	if err != nil {
		rt.refuseCeremony(w, r, "passkey registration rejected", err, nil)
		return
	}
	credential := credentialFrom(result.Credential, who.accountID, time.Now())
	if who.first {
		// The first credential of a passkey_only account persists, activates,
		// and consumes the bootstrap credential as one unit, then trades the
		// restricted ticket for a normal session.
		if !rt.completeBootstrapEnrollment(w, r, who.ticket, credential) {
			return
		}
	} else {
		if err := rt.credentials.Save(r.Context(), credential, nil); err != nil {
			pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "passkey credential could not be persisted", pw.Err(err))
			pw.WriteProblem(w, r, pw.ServiceUnavailable())
			return
		}
		// The account can now authenticate a new way, so the session it did
		// that from is replaced rather than carried forward.
		view, _ := Session(r.Context())
		if err := rt.manager.RotateWithMethod(w, r, view.Data, view.Method); err != nil {
			pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "session rotation failed", pw.Err(err))
			pw.WriteProblem(w, r, pw.ServiceUnavailable())
			return
		}
	}
	writeCeremonyJSON(w, r, map[string]string{
		"credential_id": base64.RawURLEncoding.EncodeToString(credential.CredentialID),
	})
}

// refuseCeremony answers every ceremony failure the same way. expected names an
// error that is an ordinary refusal rather than a fault worth an error log.
func (rt *runtime) refuseCeremony(w http.ResponseWriter, r *http.Request, message string, err, expected error) {
	logger := pw.Logger(r.Context())
	if expected != nil && errors.Is(err, expected) {
		logger.Log(r.Context(), pw.LevelWarn, message)
	} else {
		// The error itself carries no challenge, credential ID, or handle;
		// contrib/passkey returns sentinel values only.
		logger.Log(r.Context(), pw.LevelWarn, message, pw.Err(err))
	}
	pw.WriteProblem(w, r, pw.Forbidden())
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

func (rt *runtime) writeCeremonyCookie(w http.ResponseWriter, key string) {
	http.SetCookie(w, &http.Cookie{
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
func (rt *runtime) takeCeremonyCookie(w http.ResponseWriter, r *http.Request) (string, bool) {
	cookie, err := r.Cookie(rt.ceremonyCookieName())
	http.SetCookie(w, &http.Cookie{
		Name:     rt.ceremonyCookieName(),
		Value:    "",
		Path:     rt.ceremonyCookiePath(),
		MaxAge:   -1,
		Secure:   rt.cookieSecure(),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	if err != nil || cookie == nil || cookie.Value == "" {
		return "", false
	}
	return cookie.Value, true
}

func decodeCeremonyJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if contentType := r.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		pw.WriteProblem(w, r, pw.BadRequest())
		return false
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxCeremonyBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		pw.WriteProblem(w, r, pw.BadRequest())
		return false
	}
	return true
}

func writeCeremonyJSON(w http.ResponseWriter, r *http.Request, payload any) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		pw.WriteProblem(w, r, pw.InternalServerError(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	// Ceremony options are single use, so nothing may keep a copy.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}
