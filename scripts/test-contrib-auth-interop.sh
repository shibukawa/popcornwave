#!/bin/sh

# Focused OAuth/OIDC fixture interoperability gate.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

export GOCACHE=${GOCACHE:-/tmp/petitweb-go-interop-cache}
go_bin=${GO:-go}
tinygo_bin=${TINYGO:-tinygo}

if ! command -v "$go_bin" >/dev/null 2>&1; then
	 echo "missing Go executable: $go_bin" >&2
	exit 2
fi

oauth_tests='TestBeginAndHandleCallbackConsumesState|TestAuthorizationParamsCannotOverrideManagedParameters|TestBeginAuthorizationRejectsInvalidScopeTokens|TestRejectsRedirectAndInvalidEndpoint|TestTwoIndependentAuthorizationServerFixtures|TestClientSecretPostAndTokenErrorAreBounded|TestOversizedCallbackDoesNotConsumeState|TestTransactionValidatorDoesNotRunBeforeStateCorrelation'
oidc_tests='TestDiscoveryAuthorizationAndNonceBoundIDToken|TestOIDCClientRejectsUnboundedTokenOptions|TestOIDCRejectsNonBearerTokenType|TestDiscoveryEndpointValidatorCoversAllProviderEndpoints|TestDiscoveryEndpointValidatorErrorIsSanitized|TestDiscoveryEndpointValidatorCannotRewriteEndpoints|TestSecondIndependentProviderFixture|TestUnknownKeyRefreshesOnce|TestProviderRefreshSerializesConcurrentRequests|TestAuthorizedPartyRejectsMalformedPresentClaim'

echo '== OAuth fixture interoperability =='
"$go_bin" test ./contrib/oauth -run "^(${oauth_tests})$"

echo '== OIDC fixture interoperability =='
"$go_bin" test ./contrib/oidc -run "^(${oidc_tests})$"

if command -v "$tinygo_bin" >/dev/null 2>&1; then
	 echo "== TinyGo $($tinygo_bin version) fixture interoperability =="
	"$tinygo_bin" test -run "^(${oauth_tests})$" ./contrib/oauth
	"$tinygo_bin" test -run "^(${oidc_tests})$" ./contrib/oidc
else
	 echo "missing TinyGo executable: $tinygo_bin" >&2
	 exit 2
fi
