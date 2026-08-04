---
id: requirement:derived-asset-pipeline
type: requirement
title: Derived Asset Pipeline
---
The build converts authored assets, writes every result plus its cache metadata into concept:derived-public-tree, embeds that tree, and rewrites the template references whose URLs moved.

```yaml
review_gate: implemented 2026-08-04, needs user approval as built
source:
  - user design discussion 2026-08-04
  - the system:tinybind reference-hook and derived-asset seam, which ships the mechanism this requirement drives
decided:
  output: dist/public holds every served file
  embed: the api:cli-init scaffolded public.go embeds dist/public rather than public
  metadata: data:public-asset-manifest is embedded with it, so cache headers need no per-request work
  conversion_side: host build only, never a request
ownership:
  upstream: matching an element and attribute, rewriting a static value, diagnostics with template position, artifact declaration, the conversion cache, and the read-set digests
  here: the transforms themselves, the format policy of policy:asset-transform-matrix, output names, the tree, the manifest, delivery, and the switch
  reason: system:tinybind takes no codec and no compiler into its module, so a project wanting a converter pays for it in the generate command this project builds
shipped_upstream_at_v0_3_1:
  verified: read against the upstream tree on 2026-08-04; every item below was confirmed in source, not inferred from the upstream catalog
  version_note: the seam landed in one commit tagged v0.3.1, not v0.3.3 as this first recorded; system:tinybind pins v0.3.3, which is later, so nothing here was blocked by the error
  matching: ReferenceHook Element and Attribute as exact lowercase names, with an optional Match, which is all the img src and script src cases need
  head_reach: a hook reaches a script or link named in a head declaration, so a typescript entry point is covered wherever it is written
  results: ReferenceResult Value and Skip with a Reason, cached like any other outcome
  produced_files: ReferenceResult.Files as ProducedFile with a name, media type, and bytes, plus DerivedAssetDir and ArtifactDerivedAsset, so a source map that no attribute names is still declared
  read_set: ReferenceResult.Read and the recorded dependency file, so editing an imported module regenerates
  caching: CacheKey returning ConversionInputs sources and a params string, stored under ConversionCacheDir
  dedup: one transform call per distinct value across the run
  dynamic_values: DynamicReference reports an expression-valued attribute, and StrictReferenceHooks makes it an error
  switch_identity: ReferenceHook marshals its registration, so adding or removing a hook regenerates instead of leaving stale output
  errors: name collisions, root escapes, and diagnostics carrying template file, line, and column
upstream_constraints_to_build_against:
  found: reading the seam on 2026-08-04, none of them blocking, all of them cheap to hit by accident
  hyphenated_elements_refused: ValidateReferenceHooks rejects an element name containing a hyphen before any template is read, because that space belongs to the builtin element whitelist; structure_invariance names img src and nothing else, so nothing here wants it today
  one_hook_per_occurrence: two hooks whose Match both claim one value fail generation naming both, decided at use rather than registration, so the png and jpeg transforms registered on img src need mutually exclusive Match predicates rather than an ordered fallthrough
  produced_file_needs_a_destination: returning Files with DerivedAssetDir unset is ErrDerivedAssetDir, so the switch of this requirement configures the directory and the hooks together or neither
  empty_value_is_an_error: a transform returning an empty Value is a generation error; declining is Skip with a Reason, which is what policy:asset-transform-matrix already calls a first-class skip
  escaping_is_the_seam's: a rewritten value is escaped for quote and both angle brackets and nothing else, so a transform returns a plain URL and never pre-escapes one
  purity_is_load_bearing: File and Pos on a request are for diagnostics only; the run-wide memo and the on-disk cache are both keyed without them, so a transform deciding output from the template position silently serves one page's answer to another
upstream_requests:
  delivered_in_v0_3_5:
    head_contribution:
      asked: a way for a conversion to declare a head tag, for the stylesheet a css module produces and no attribute can name
      shipped: ReferenceResult.Head, entries restricted to link, script, and style, deduplicated per component, replayed from the conversion cache
      used_here: the typescript build returns the companion stylesheet's link beside its rewrite
    parallel_conversion:
      asked: converting outside the sequential template compile, since a cold cache converts serially
      shipped: GenerateOptions.ConversionWorkers, excluded from the hashed options so the runner never reaches the output
      used_here: set from the core count, capped, and never configured, because it cannot change what is generated
  not_needed:
    element_result: refused here by policy:asset-transform-matrix, so the unbuilt markup-replacing result costs nothing
    srcset_support: ReferenceResult.Value replaces the whole attribute string, so a descriptor list can be parsed and reassembled by the transform with no upstream change
    external_tool_identity: a tool path and version belong in the CacheKey params string, which already joins the hash
    mount_table: resolving a URL to an authored directory stays this project's rule, per policy:public-asset-resolution
    report_formatting: GenerateResult.Rewrites carries the data and formatting is a caller concern
  latent:
    hyphenated_element_attributes:
      what: matching an attribute on a hyphenated passthrough element, which the seam refuses at registration
      status: not a request today, because structure_invariance keeps the reference sites to img src
      becomes_a_request_when: policy:asset-transform-matrix ever names an attribute on a component-supplied element as a reference site
      already_open_upstream: it is the first of the element-reference-hook open questions, so it would be a nudge rather than a new ask
as_built:
  build: pw build clears dist/public, walks the authored tree, copies or transforms each file, places what the hooks produced, writes the zstd sidecars, and emits the manifest last
  staging: hooks write to dist/derived and the tree builder moves those files, so clearing the served tree never deletes what pw generate produced
  manifest: a generated Go table in the public.go package plus dist/manifest.json for tooling; the middleware reads it and computes no digest per request
  embed: the scaffold embeds dist/public, and a project still embedding the authored tree is refused with the two lines to change
  serving: manifest-driven, so a URL the build did not declare is 404 whatever the tree holds; Vary names Accept only where a URL carries more than one media type
  cache_control:
    invented_urls: a file the build produced carries the digest of its own bytes in its name and is answered with a year and immutable, which is honest because different bytes are a different URL
    authored_urls: a file that kept the name its author wrote revalidates, since nothing rewrites a stylesheet link or an authored script reference and the next build may put different bytes behind it
    why_not_everything: hashing a name only works where every reference to it is rewritten, and a link href is not
  source_maps: the script build emits a linked map, named from the bundle's digest; the digest is taken over the bundle without its sourceMappingURL comment, and the comment is written back naming the map that digest produced
  conversions: img src png and jpeg to webp, script src typescript to javascript, css minified with its url() references rewritten, and an optional avif sibling of every served image
  encoders: pinned by the api:cli-add images capability, which writes the devbox packages and the switch together; a machine with none declines and reports rather than failing
  head_contribution: a css companion of a typescript build declares its own link, through the upstream field v0.3.5 added
  read_set_is_inputs_only: the build tool reports its outputs beside its inputs, and recording an output as a dependency makes it unverifiable, which regenerates every run while appearing to cache
  module_tag_check: the script build emits a module and generation refuses a built entry under a classic tag, naming the template file and line; it runs in api:cli-generate rather than in the asset build, so a generate on its own reports it and a --check run sees it, and it is the one place that does
  variant_cache: a media variant is produced by the tree walk, which the upstream conversion cache never sees, so it has one of its own under the same directory, keyed by the source digest with the format, the axis, the quality, and the tool identity
  staging_is_cleared: generation clears the staging directory before writing, because everything found there reaches the served tree and a file produced for a deleted source would otherwise ship forever
  retention: a converted source is dropped only when the literal-occurrence scan finds no reference the build could not rewrite, and a retention is reported
  development: pw dev runs the same conversions and serves dist/public from disk, so a rewritten reference resolves there too
  checks: pw doctor reports a tree older than its sources and an enabled image conversion whose encoder is absent
still_missing:
  - a stylesheet url() inside a template style block, deferred rather than open
  - an authored stylesheet or script keeps its name, so only a produced file is immutable; widening that means rewriting link href too
out_of_scope:
  srcset: refused, per policy:asset-transform-matrix
  existing_project_migration: a project scaffolded before this states its own embed path and ignore rules; the build names the two lines to change and nothing rewrites them
upstream_requests:
  delivered_in_v0_3_5:
    head_contribution:
      asked: a way for a conversion to declare a head tag, for the stylesheet a css module produces and no attribute can name
      shipped: ReferenceResult.Head, entries restricted to link, script, and style, deduplicated per component, replayed from the conversion cache
      used_here: the typescript build returns the companion stylesheet's link beside its rewrite
    parallel_conversion:
      asked: converting outside the sequential template compile, since a cold cache converts serially
      shipped: GenerateOptions.ConversionWorkers, excluded from the hashed options so the runner never reaches the output
      used_here: set from the core count, capped, and never configured, because it cannot change what is generated
  not_needed:
    element_result: refused here by policy:asset-transform-matrix, so the unbuilt markup-replacing result costs nothing
    srcset_support: ReferenceResult.Value replaces the whole attribute string, so a descriptor list can be parsed and reassembled by the transform with no upstream change
    external_tool_identity: a tool path and version belong in the CacheKey params string, which already joins the hash
    mount_table: resolving a URL to an authored directory stays this project's rule, per policy:public-asset-resolution
    report_formatting: GenerateResult.Rewrites carries the data and formatting is a caller concern
  latent:
    hyphenated_element_attributes:
      what: matching an attribute on a hyphenated passthrough element, which the seam refuses at registration
      status: not a request today, because structure_invariance keeps the reference sites to img src
      becomes_a_request_when: policy:asset-transform-matrix ever names an attribute on a component-supplied element as a reference site
      already_open_upstream: it is the first of the element-reference-hook open questions, so it would be a nudge rather than a new ask
missing_here:
  - no hook is registered anywhere; the generate command passes no ReferenceHooks, DerivedAssetDir, or ConversionCacheDir
  - the pw generate artifact filter drops ArtifactDerivedAsset through its default branch, so a produced file would be discarded today
  - no transform exists for any kind in policy:asset-transform-matrix
  - flow:derived-asset-build does not exist; flow:public-asset-build writes sidecars into the authored tree
  - data:public-asset-manifest is not produced and api:public-asset-middleware digests every response itself
  - public.go embeds public, and concept:project-layout scaffolds it once and never rewrites it, so an existing project needs a migration
  - data:project-config has no asset keys beyond the three tailwind ones
  - rule:production-readiness-checks and rule:project-integrity-checks carry no derived-tree checks
not_an_upstream_concern:
  css_urls:
    conclusion: unchanged; background-image needs nothing from upstream, because the stylesheet pass here owns those bytes
    reason_corrected: the earlier reason, that the upstream non-goals refuse css permanently, is not what the seam says; css is a non-goal of the reference-hook seam and a url() pass is recorded upstream as a later item with a natural home in its scoped-component-style requirement
    what_actually_divides_them: that later item is about a style block inside a template, and a standalone stylesheet in the authored tree is outside every upstream requirement, so the two never contend
    watch: if upstream ever builds the style-block url pass, an inline block and a linked stylesheet would be rewritten by different owners under different rules, which is the day to reconcile them rather than now
  tree_manifest_and_delivery: dist/public, data:public-asset-manifest, the embed, and every response header are this project's alone
  every_converter: no codec and no compiler enters the upstream module, per decision:asset-transform-toolchain
caching:
  problem: a name derived from its source is stable, so bytes behind it can change and the response cannot claim immutable
  default: stable names, strong ETag from data:public-asset-manifest, and a revalidating Cache-Control, which is a 304 per asset per visit and strictly better than today's absent header
  immutable_options:
    hashed_name:
      form: the digest enters the file name, so the URL is genuinely immutable
      cost: the rewritten reference is compiled into generated Go, so any byte difference between a development and a production encode changes generated output and policy:generated-artifacts --check compares against one profile only
    runtime_resolution:
      form: the template names a logical URL and the render reads the manifest, as requirement:framework-script-assets already does for its own script
      cost: a lookup per reference at render time and a manifest the runtime must hold, against a URL that no longer depends on when it was built
    recommendation: ship the stable-name default first, since it is the one that needs no decision about the other two
configuration:
  binding: data:project-config assets, beside the existing tailwind keys
  shape: per-kind enablement and settings, one switch that registers the hooks or does not
  regeneration: the options value is hashed, so flipping a switch regenerates rather than leaving stale output
structure_invariance:
  rule: the built page has the element tree the author wrote; only attribute values and file bytes change
  scope: policy:asset-transform-matrix holds the element and attribute list, which for images is img src and nothing else
  reason: a structural rewrite breaks css combinators and structural javascript that the build cannot see, so no diagnostic could catch it
acceptance:
  - a project registering no transform embeds a byte-identical copy of its authored tree and serves exactly what it serves today
  - a png a template references from an img is served as webp, the png is absent from the built tree, and no reference names it
  - a background-image in a stylesheet names the converted file, and the stylesheet keeps its own URL
  - an image named only by a script or a meta tag is unchanged, still present, and reported as retained
  - one image referenced by both an img and a stylesheet converts once and produces one manifest entry
  - no response contains an element the author did not write
  - a conversion that loses to its source leaves the reference alone, reports the reason, and never runs twice
  - a minified css keeps its URL, so no template changes and no generated Go moves
  - editing a file a build read but no template names regenerates that output and only it
  - a second build with nothing changed converts nothing and produces byte-identical output
  - every served response takes its ETag and Cache-Control from the manifest and computes no digest
  - a build failure leaves the previous tree intact and embeds no partial one
non_goals:
  - conversion, resizing, or transcoding at request time
  - a bundler or a format table inside system:tinybind, which its own non-goals exclude
  - serving the authored tree in any mode
  - fetching an asset from a CDN origin at build time
```
