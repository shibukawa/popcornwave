---
id: api:testutil-passkey
type: api
title: Test Passkey Authenticator
---
testutil binds a requirement:contrib-passkey-test authenticator to an api:test-run server and drives api:authentication-endpoints with it, so flow:passkey-enrollment and flow:passkey-login run in a test without a browser.

```yaml
package: github.com/shibukawa/popcornwave/testutil
testing_interface: decision:testutil-testing-interface
rung: the ceremony rung of decision:test-authentication-seams, so a test that only needs an authenticated request uses api:testutil-auth instead
endpoints: api:passkey-endpoints
surface:
  - WithVirtualAuthenticator(options ...AuthenticatorOption) RunOption
  - WithAuthenticatorOrigin(origin string) overrides the derived origin
  - WithAuthenticatorFault(fault) makes the next ceremony produce an invalid response
  - WithBootstrapCredential(loginID, secret string) seeds a data:account-bootstrap-credential
  - (*Server).Authenticator() returns the bound authenticator
  - (*Server).NewAuthenticator(t) returns an additional independent authenticator for a second device or user
  - (*Server).Enroll(t, client) completes flow:passkey-enrollment for an already authenticated client
  - (*Server).PasskeyLogin(t) completes flow:passkey-login and returns a client carrying the session cookie
run_option_order:
  - reserve the loopback port as api:test-run already does
  - derive the origin from the reserved port and the RP ID from its host
  - write both into the copied data:authentication-runtime-config and into the authenticator
  - continue with the api:test-run and api:test-seed order
origin_binding:
  problem: an origin contains the port and an RP ID must not, and neither value exists before the port is reserved
  rp_id: the host alone, normally localhost, which WebAuthn admits because http://localhost is a secure origin
  origin: http://localhost with the reserved port, admitted by the loopback exception in policy:passkey-security
  effect: a test writes no host and no port, and the configuration under test keeps its production shape
client:
  form: an http.Client with a cookie jar, because enrollment and login each carry session and CSRF cookies across two requests
  csrf: the helpers read and replay the policy:csrf-protection token instead of disabling it
  reuse: the returned client is the one the test uses for subsequent authenticated requests
mode_coverage:
  oidc_only: no authenticator is started
  oidc_passkey: combines with api:testutil-idp in one run, so a test enrolls a passkey after an OIDC login
  passkey_only: WithBootstrapCredential gives flow:passkey-only-registration its entry point
lifecycle:
  - the authenticator holds in-memory keys only and needs no cleanup beyond the run
  - credentials do not outlive the server, which matches a fresh device per test
rules:
  - no authenticator is created unless WithVirtualAuthenticator is supplied
  - helpers post to the configured endpoint paths, never to hardcoded ones
  - no helper bypasses a ceremony or writes a data:passkey-credential row directly
  - tests assert on session state and stored credential rows, not on authenticator internals
```
