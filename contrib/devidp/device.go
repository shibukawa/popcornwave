package devidp

import (
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/contrib/internal/authn"
)

const (
	deviceGrantURN       = "urn:ietf:params:oauth:grant-type:device_code"
	deviceLifetime       = 10 * time.Minute
	devicePollInterval   = 5 * time.Second
	maxVerificationTries = 5
	userCodeAlphabet     = "BCDFGHJKLMNPQRSTVWXZ"
)

type deviceStatus uint8

const (
	devicePending deviceStatus = iota
	deviceApproved
	deviceDenied
)

type pendingDevice struct {
	clientID             string
	deviceCode           string
	userCode             string
	scopes               []string
	csrf                 string
	expiresAt            time.Time
	nextPollAt           time.Time
	interval             time.Duration
	status               deviceStatus
	subject              string
	authTime             time.Time
	verificationFailures int
}

type verificationAttempt struct {
	startedAt time.Time
	count     int
}

func (p *Provider) handleDeviceAuthorization(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeTokenError(w, http.StatusBadRequest, "invalid_request", "the device authorization request could not be parsed")
		return
	}
	if !singleFormValues(r.Form, "client_id", "client_secret", "scope") {
		writeTokenError(w, http.StatusBadRequest, "invalid_request", "device authorization parameters must not be repeated")
		return
	}
	client := p.authenticateDeviceClient(r)
	if client == nil || !supportsGrant(client, GrantDeviceCode) {
		w.Header().Set("WWW-Authenticate", `Basic realm="devidp"`)
		writeTokenError(w, http.StatusUnauthorized, "invalid_client", "device client authentication failed")
		return
	}
	scopes := strings.Fields(r.Form.Get("scope"))
	if !contains(scopes, "openid") {
		writeTokenError(w, http.StatusBadRequest, "invalid_scope", "the openid scope is required")
		return
	}
	for _, scope := range scopes {
		if !validScopeToken(scope) {
			writeTokenError(w, http.StatusBadRequest, "invalid_scope", "a requested scope is malformed")
			return
		}
	}
	device, err := p.newPendingDevice(client, scopes)
	if err != nil {
		writeTokenError(w, http.StatusServiceUnavailable, "server_error", "the provider could not start device authorization")
		return
	}
	verificationURI := p.endpoint("/device")
	complete, _ := url.Parse(verificationURI)
	query := complete.Query()
	query.Set("user_code", device.userCode)
	complete.RawQuery = query.Encode()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, map[string]any{
		"device_code":               device.deviceCode,
		"user_code":                 displayUserCode(device.userCode),
		"verification_uri":          verificationURI,
		"verification_uri_complete": complete.String(),
		"expires_in":                int(deviceLifetime / time.Second),
		"interval":                  int(devicePollInterval / time.Second),
	})
}

func (p *Provider) newPendingDevice(client *Client, scopes []string) (*pendingDevice, error) {
	for range 3 {
		deviceCode, err := authn.GenerateSecret(p.random, secretBytes)
		if err != nil {
			return nil, err
		}
		userCode, err := p.generateUserCode()
		if err != nil {
			return nil, err
		}
		csrf, err := authn.GenerateSecret(p.random, secretBytes)
		if err != nil {
			return nil, err
		}
		now := p.now()
		device := &pendingDevice{clientID: client.ID, deviceCode: deviceCode, userCode: userCode,
			scopes: append([]string(nil), scopes...), csrf: csrf, expiresAt: now.Add(deviceLifetime),
			nextPollAt: now.Add(devicePollInterval), interval: devicePollInterval}
		p.mu.Lock()
		p.sweepLocked()
		if len(p.devicesByCode) >= maxPendingDevices {
			p.mu.Unlock()
			return nil, ErrClosed
		}
		clientDevices := 0
		for _, existing := range p.devicesByCode {
			if existing.clientID == client.ID {
				clientDevices++
			}
		}
		if clientDevices >= maxPendingDevicesPerClient {
			p.mu.Unlock()
			return nil, ErrClosed
		}
		if p.devicesByCode[deviceCode] == nil && p.devicesByUserCode[userCode] == nil {
			p.devicesByCode[deviceCode] = device
			p.devicesByUserCode[userCode] = device
			p.mu.Unlock()
			return device, nil
		}
		p.mu.Unlock()
	}
	return nil, ErrClosed
}

func (p *Provider) generateUserCode() (string, error) {
	result := make([]byte, 8)
	buffer := []byte{0}
	for index := range result {
		for {
			if _, err := io.ReadFull(p.random, buffer); err != nil {
				return "", err
			}
			if int(buffer[0]) < 240 {
				result[index] = userCodeAlphabet[int(buffer[0])%len(userCodeAlphabet)]
				break
			}
		}
	}
	return string(result), nil
}

func normalizeUserCode(value string) string {
	value = strings.ToUpper(value)
	var result strings.Builder
	result.Grow(8)
	for _, char := range value {
		if strings.ContainsRune(userCodeAlphabet, char) {
			result.WriteRune(char)
		}
	}
	return result.String()
}

func displayUserCode(value string) string {
	if len(value) != 8 {
		return value
	}
	return value[:4] + "-" + value[4:]
}

func (p *Provider) authenticateDeviceClient(r *http.Request) *Client {
	clientID := r.Form.Get("client_id")
	secret := r.Form.Get("client_secret")
	basic := false
	if id, supplied, ok := r.BasicAuth(); ok {
		decodedID, errID := url.QueryUnescape(id)
		decodedSecret, errSecret := url.QueryUnescape(supplied)
		if errID != nil || errSecret != nil {
			return nil
		}
		clientID, secret, basic = decodedID, decodedSecret, true
	}
	client := p.client(clientID)
	if client == nil {
		return nil
	}
	if client.Secret == "" {
		if basic || secret != "" {
			return nil
		}
		return client
	}
	if secret == "" || !authn.EqualSecret(client.Secret, secret) {
		return nil
	}
	return client
}

func supportsGrant(client *Client, grant string) bool {
	return client != nil && contains(client.GrantTypes, grant)
}

func singleFormValues(form url.Values, keys ...string) bool {
	for _, key := range keys {
		if len(form[key]) > 1 {
			return false
		}
	}
	return true
}

type devicePollResult uint8

const (
	pollUnknown devicePollResult = iota
	pollPending
	pollSlowDown
	pollDenied
	pollApproved
)

func (p *Provider) pollDevice(client *Client, deviceCode string) (devicePollResult, *issuedCode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sweepLocked()
	device := p.devicesByCode[deviceCode]
	if device == nil || device.clientID != client.ID {
		return pollUnknown, nil
	}
	now := p.now()
	if now.Before(device.nextPollAt) {
		device.interval += 5 * time.Second
		device.nextPollAt = now.Add(device.interval)
		return pollSlowDown, nil
	}
	device.nextPollAt = now.Add(device.interval)
	switch device.status {
	case devicePending:
		return pollPending, nil
	case deviceDenied:
		delete(p.devicesByCode, device.deviceCode)
		delete(p.devicesByUserCode, device.userCode)
		return pollDenied, nil
	case deviceApproved:
		delete(p.devicesByCode, device.deviceCode)
		delete(p.devicesByUserCode, device.userCode)
		return pollApproved, &issuedCode{clientID: device.clientID, subject: device.subject,
			scopes: append([]string(nil), device.scopes...), issuedAt: now, expiresAt: now.Add(p.codeTTL), authTime: device.authTime}
	default:
		return pollUnknown, nil
	}
}

func (p *Provider) handleDevicePage(w http.ResponseWriter, r *http.Request) {
	source := remoteHost(r)
	if p.verificationLimited(source) {
		p.renderDeviceEntry(w, "Too many attempts. Wait before trying again.")
		return
	}
	code := normalizeUserCode(r.URL.Query().Get("user_code"))
	if code == "" {
		p.renderDeviceEntry(w, "")
		return
	}
	device := p.deviceByUserCode(code)
	if device == nil {
		p.recordVerificationFailure(source)
		p.renderDeviceEntry(w, "The code is invalid or expired.")
		return
	}
	p.renderDeviceApproval(w, device, "")
}

func (p *Provider) handleDeviceSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		p.renderDeviceEntry(w, "The code is invalid or expired.")
		return
	}
	source := remoteHost(r)
	if p.verificationLimited(source) {
		p.renderDeviceEntry(w, "Too many attempts. Wait before trying again.")
		return
	}
	code := normalizeUserCode(r.Form.Get("user_code"))
	device := p.deviceByUserCode(code)
	if device == nil || !authn.EqualSecret(device.csrf, r.Form.Get("csrf")) {
		if device != nil {
			p.recordPendingVerificationFailure(device.deviceCode)
		}
		p.recordVerificationFailure(source)
		p.renderDeviceEntry(w, "The code is invalid or expired.")
		return
	}
	if r.Form.Get("deny") != "" {
		p.completeDeviceDecision(device, "", true)
		p.renderError(w, http.StatusOK, "The device request was denied. You may return to the device.")
		return
	}
	user := p.user(r.Form.Get("subject"))
	client := p.client(device.clientID)
	if user == nil || client == nil {
		p.renderDeviceApproval(w, device, "Select a user from the roster.")
		return
	}
	device.scopes = p.grantableScopes(device.scopes, client, user)
	p.completeDeviceDecision(device, user.Subject, false)
	p.renderError(w, http.StatusOK, "The device is authorized. You may return to the device.")
}

func (p *Provider) recordPendingVerificationFailure(deviceCode string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	device := p.devicesByCode[deviceCode]
	if device == nil {
		return
	}
	device.verificationFailures++
	if device.verificationFailures >= maxVerificationTries {
		device.status = deviceDenied
		delete(p.devicesByUserCode, device.userCode)
	}
}

func (p *Provider) deviceByUserCode(code string) *pendingDevice {
	if len(code) != 8 {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sweepLocked()
	device := p.devicesByUserCode[code]
	if device == nil || device.status != devicePending {
		return nil
	}
	copy := *device
	copy.scopes = append([]string(nil), device.scopes...)
	return &copy
}

func (p *Provider) completeDeviceDecision(candidate *pendingDevice, subject string, denied bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	device := p.devicesByCode[candidate.deviceCode]
	if device == nil || device.status != devicePending || !authn.EqualSecret(device.csrf, candidate.csrf) {
		return
	}
	if denied {
		device.status = deviceDenied
		return
	}
	device.status = deviceApproved
	device.subject = subject
	device.scopes = append([]string(nil), candidate.scopes...)
	device.authTime = p.now()
}

func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (p *Provider) verificationLimited(source string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sweepLocked()
	attempt := p.verificationAttempts[source]
	return attempt != nil && attempt.count >= maxVerificationTries
}

func (p *Provider) recordVerificationFailure(source string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sweepLocked()
	attempt := p.verificationAttempts[source]
	if attempt == nil {
		attempt = &verificationAttempt{startedAt: p.now()}
		p.verificationAttempts[source] = attempt
	}
	attempt.count++
}

var deviceEntryTemplate = template.Must(template.New("device-entry").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Authorize a development device</title></head><body><main><p>Development identity provider &mdash; no password is checked</p>{{if .}}<p>{{.}}</p>{{end}}<form method="get"><label>Code shown on the device <input name="user_code" autocomplete="one-time-code"></label><button type="submit">Continue</button></form></main></body></html>`))

var deviceApprovalTemplate = template.Must(template.New("device-approval").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Authorize a development device</title></head><body><main><p>Development identity provider &mdash; no password is checked</p><h1>Authorize a development device</h1><p>Confirm code <strong>{{.Code}}</strong>. Client <code>{{.ClientID}}</code> requests <code>{{.Scopes}}</code>. Another device will receive access.</p>{{if .Message}}<p>{{.Message}}</p>{{end}}<form method="post"><input type="hidden" name="user_code" value="{{.RawCode}}"><input type="hidden" name="csrf" value="{{.CSRF}}">{{range .Users}}<button type="submit" name="subject" value="{{.Subject}}">Approve for {{.DisplayName}} ({{.Subject}})</button><br>{{end}}<button type="submit" name="deny" value="1">Deny</button></form></main></body></html>`))

type deviceApprovalView struct {
	Code, RawCode, CSRF, ClientID, Scopes, Message string
	Users                                          []loginUserView
}

func (p *Provider) renderDeviceEntry(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	_ = deviceEntryTemplate.Execute(w, message)
}

func (p *Provider) renderDeviceApproval(w http.ResponseWriter, device *pendingDevice, message string) {
	view := deviceApprovalView{Code: displayUserCode(device.userCode), RawCode: device.userCode, CSRF: device.csrf,
		ClientID: device.clientID, Scopes: strings.Join(device.scopes, " "), Message: message}
	for _, user := range p.Users() {
		view.Users = append(view.Users, loginUserView{Subject: user.Subject, DisplayName: user.DisplayName})
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	_ = deviceApprovalTemplate.Execute(w, view)
}
