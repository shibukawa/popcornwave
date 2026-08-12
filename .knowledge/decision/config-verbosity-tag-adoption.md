---
id: decision:config-verbosity-tag-adoption
type: decision
title: Config Verbosity Tag Adoption
---
Which framework config fields carry a value condition and which carry a detail rating, so requirement:startup-summary-brevity is answered at the declaration rather than in a renderer.

```yaml
status: accepted
state: implemented
mechanism: system:tinybind config tags, read at generation time
was_before_adoption:
  dependon: 87 tags, all the emptiness form, so no variant subtree was ever hidden
  summary: none, so every provenance entry arrived unrated
  enum: 4 tags, none on a variant-selecting parent
result:
  measured_on: requirement:startup-summary-brevity measured_case
  provenance: 125 entries before, 99 after
  boot_summary: 125 lines before, 61 after
  split: 26 lines removed by the applicability lever, 38 marked and skipped by the volume lever
value_conditions:
  form: 'dependon:"<parent>=<value>,<value>" or dependon:"<parent>!=<value>"'
  replaces: 'the dependon:".enabled" now on each variant struct; the enabled gate arrives transitively through the parent'
  why_transitive: the parent itself carries dependon on the feature switch, so a condition on the parent inherits it
  session:
    parent: session.backend
    values: rdb, cookie, dev-volatile, dev-persist, redis, dynamo, firestore
    rdb: '.backend=rdb'
    redis: '.backend=redis'
    dynamo: '.backend=dynamo'
    firestore: '.backend=firestore'
    cookie_store: '.backend=cookie'
    keyring: '.backend!=cookie'
    cookie: unchanged; the token cookie travels under every backend
    why_not_equal_on_keyring: >
      a keyring serves every server backend, so an equal list would have to gain
      each backend that ships and a forgotten one hides a secret that is in force
    dev_backends: >
      dev-volatile and dev-persist select no struct of their own, so no tag names
      them; they are covered by the keyring's not-equal form
  auth:
    parent: auth.mode
    values: oidc_only, oidc_passkey, passkey_only, jwt_only
    oidc: '.mode=oidc_only,oidc_passkey'
    passkey: '.mode=oidc_passkey,passkey_only'
    jwt: '.mode=jwt_only'
    source_of_truth: the usesOIDC, usesPasskey, and usesJWT predicates in plugin/auth/config.go, which already state the same mapping
    correction: >
      the worked example in system:tinybind lists three modes and gives passkey
      only oidc_passkey; this deployment has four, and passkey_only would lose its
      own subtree under that reading
  ratelimit:
    parent: ratelimit.backend
    values: memory, redis
    redis: '.backend=redis'
  unchanged:
    - 'the emptiness form everywhere it already reads correctly, which is every subtree gated by a bool switch'
    - 'auth.backend, which selects no struct of its own today'
  gap_found_while_adopting:
    where: auth.assurance.hint and auth.assurance.presence
    what: >
      both declare an Enabled switch, and neither had a dependon on any sibling,
      so five keys printed as in force while their feature was off
    fix: 'dependon:".enabled"' on every sibling of both switches
    cost: 5 lines, and the same misreading the value conditions exist to stop
    how_it_surfaced: >
      reading the default-sourced keys of each rating candidate before rating it;
      a subtree whose own switch is off and whose knobs still print is visible
      there and invisible in a tag census
    audit: >
      the same check over every config struct with an Enabled field found no
      other instance, so this was the last one
enum_prerequisite:
  rule: a parent named by a value condition declares enum, and generation rejects a value that is not a member
  why: >
    a mistyped value hides a whole subtree silently and forever, and it is the one
    failure of this feature a reader cannot diagnose from the output
  to_add:
    - 'auth.mode: enum:"oidc_only,oidc_passkey,passkey_only,jwt_only"'
    - 'session.backend: enum:"rdb,cookie,dev-volatile,dev-persist,redis,dynamo,firestore"'
    - 'ratelimit.backend: enum:"memory,redis"'
  note: >
    each of the three already lists its values in help text and auth.mode also
    validates them in Go, so the tag records what three places state informally
  also_worth_it: auth.backend and observability.stdout_format, which name no condition but gain the same generation-time check
summary_ratings:
  form: 'summary:"omit"'
  meaning: the key is detail rather than headline; it leaves the short surface only while nothing but the default layer set it
  polarity: opt-in, so a field nobody rates stays visible and a forgotten tag costs length rather than a hidden setting
  prefer_struct_level: one tag on a nested struct field covers every leaf under it
  test_applied:
    rate: a knob that bounds how something behaves — a timeout, a size, an interval, a diagnostic verbosity
    do_not_rate:
      - a key whose default is a security posture, since a reader checks those at their defaults
      - a feature switch, which is the reason its subtree is present or absent
      - an address or path the deployment answers on
      - a storage identity the deployment chose, such as a table or a kind
  applied:
    struct_level: observability.query 9, observability.trace 5, auth.bootstrap 3, html.cache 2, session.redis 2
    leaf_level: 'server 6 timeouts and the body cap, html 9 bounds, auth.recent_auth_max_age, middleware.request_timeout'
    total: 38 lines marked in the measured case
  not_rated_though_ranked_high:
    security.headers: 6 lines, and every one of them is the posture a reader opens the summary to check
    auth.oidc: >
      8 lines including admission, auto_provision, and allow_loopback_http; the
      permissive answers are exactly the ones that must not go unread
    session.cookie: 5 lines, of which secure, http_only, and same_site are the cookie's security posture
    server.public: 4 lines naming a mount the deployment answers on, plus two disclosure switches
    auth.session: 3 lines stating how long a proof of identity stays good
    the_switches: middleware.rdb, observability.otel, security.csrf, html.update, each one line naming whether a feature is on
    consequence: >
      the summary lands at 61 rather than the 45 a mechanical reading of the
      ranking would have reached; the 16 lines are the price of not hiding a
      posture, and they are the right price
  correction: >
    an earlier pass here ranked candidates by how many default lines each covers
    and proposed the top 16. Ranking finds the volume, not the judgment: four of
    those 16 are security postures and four more are feature switches. The ranking
    is where to look, and the test above is what decides.
  already_free: >
    a field with no default and no source never reaches provenance at all, which
    is why session.keyring contributes no line today
rejected:
  render_side_filter:
    spelling: a renderer that drops every default-sourced key
    why_not: >
      it hides settings a reader came to read; an applicable key at its default is
      often the answer, and only the author knows which default is load-bearing
  hand_maintained_allowlist:
    why_not: it drifts the moment a field is added, and the drift is invisible
related:
  - requirement:startup-summary-brevity
  - policy:startup-summary
  - api:cli-doctor
  - api:runtime-configuration
  - data:loaded-configuration
  - data:session-runtime-config
  - data:rate-limit-runtime-config
  - decision:auth-backend-selection
  - decision:assurance-scope-oidc-only
  - system:tinybind
```
