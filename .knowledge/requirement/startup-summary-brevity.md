---
id: requirement:startup-summary-brevity
type: requirement
title: Startup Summary Brevity
---
The startup summary must show what this deployment decided, which needs an applicability lever and a volume lever; policy:startup-summary emits what they leave.

```yaml
priority: should
intent: an operator reads policy:startup-summary and finds the decisions without filtering it by eye
upstream: system:tinybind states both levers; this states Popcorn Wave's obligation to adopt them
measured_case:
  source: this repository, session.backend=redis and auth.mode=oidc_only, auth and session enabled
  file_set_keys: 9
  before:
    lines: 125, every one carrying a value
    from_default: 116
    from_file: 9
    by_section: auth 49, observability 18, session 17, html 16, server 11, security 7, middleware 6, ratelimit 1
  after:
    provenance: 99, which is what api:cli-doctor renders
    boot_summary: 61, which is provenance less the 38 rated entries
    by_section: auth 29, observability 18, html 16, server 11, session 11, security 7, middleware 6, ratelimit 1
  note: >
    smaller than the 189 lines system:tinybind measured, which counted a deployment
    with a configured connection set; the ratio of default to decided is the same
  reproducing_it: >
    no harness is committed for this; the numbers came from a throwaway test that
    loaded a fixture TOML and counted result.Provenance() against bootEntries.
    pw/bootlog_test.go holds the behavior, not the counts, because a line count is
    a fact about one fixture and would fail on every field the framework adds
two_levers:
  inapplicable_keys:
    removes: a variant subtree the selected mode or backend did not select
    kind_of_problem: correctness
    why: >
      an inert setting printed beside a live one reads as in force; a reader who
      trusts auth.jwt.leeway under mode=oidc_only has been told something false
    measured: 26 of 125 lines
    subtrees: auth.jwt 12, auth.passkey 3, session.rdb 2, session.dynamo 2, session.firestore 1, session.cookie_store 1
    plus: 5 more under auth.assurance, whose two feature switches gated nothing until this work
    adoption: decision:config-verbosity-tag-adoption value_conditions
    state: implemented
  unremarkable_keys:
    removes: a key rated as detail whose winning place is the default layer
    kind_of_problem: volume
    why: the dominant mass after the first lever is still 95 of 104 lines sitting at defaults
    ceiling: 90 lines
    measured: 38 marked, landing the summary at 61
    not_taken: >
      16 further lines rank high by volume and are security postures, feature
      switches, or addresses; see decision:config-verbosity-tag-adoption
      not_rated_though_ranked_high
    adoption: decision:config-verbosity-tag-adoption summary_ratings
    state: implemented
why_both:
  - the levers cut along different axes and neither subsumes the other
  - an inapplicable key set from a file survives a default filter and still misleads
  - an unremarkable key is not an inert one; it applies, and api:cli-doctor still prints it
shared_safety_property:
  statement: neither lever ever removes a value a source set
  applicability: hides a subtree by the parent's value, and the subtree is inert whoever set it
  volume: requires the default layer to have won, so any key a file, env, or flag set survives
surface_split:
  short: policy:startup-summary, which skips the rated entries
  complete: api:cli-doctor, which renders every entry it is handed
  neither_shows: a key the applicability lever dropped, or one policy:log-emission hides
  one_call: both surfaces read the same provenance slice; only the skip differs
non_goals:
  - a shorter summary at the cost of omitting a key that is in force
  - hiding a key from the TOML and .env scaffolds, which render before any load and must stay discoverable
  - a hand-maintained allowlist of interesting keys
  - changing what the bound struct receives, what validation rejects, or which CLI flags exist
related:
  - policy:startup-summary
  - decision:config-verbosity-tag-adoption
  - api:cli-doctor
  - api:runtime-configuration
  - data:loaded-configuration
  - policy:log-emission
  - rule:dsn-redaction
  - system:tinybind
acceptance:
  - session.backend=redis leaves no session.rdb, session.dynamo, session.firestore, or session.cookie_store line in either surface
  - auth.mode=oidc_only leaves no auth.passkey or auth.jwt line in either surface
  - auth.mode=passkey_only leaves auth.passkey visible, which the three-mode reading in system:tinybind would have hidden
  - session.cookie and session.keyring stay under every server backend, because they serve all of them
  - a key set by file, env, or flag appears in the startup summary whatever its rating
  - a rated subtree with one file-set leaf prints that leaf and nothing else of the subtree
  - api:cli-doctor prints every rated key, from the same provenance call the summary skipped them in
  - an unrated key at its default appears on both surfaces
  - the record form of policy:startup-summary skips the same entries the tree form does
```
