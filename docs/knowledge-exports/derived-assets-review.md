# Derived Asset Pipeline

Profile: `review`

| ID | Type | Title |
| --- | --- | --- |
| `requirement:derived-asset-pipeline` | `requirement` | Derived Asset Pipeline |
| `data:project-config` | `data` | Project Configuration |
| `data:public-asset-manifest` | `data` | Public Asset Manifest |
| `decision:asset-transform-toolchain` | `decision` | Asset Transform Toolchain |
| `policy:asset-transform-matrix` | `policy` | Asset Transform Matrix |
| `policy:generated-artifacts` | `policy` | Generated Artifact Ownership |
| `policy:public-asset-resolution` | `policy` | Public Asset Resolution |
| `rule:production-readiness-checks` | `rule` | Production Readiness Checks |
| `rule:project-integrity-checks` | `rule` | Project Integrity Checks |
| `system:tinybind` | `system` | TinyBind |
| `api:cli-add` | `api` | pw add |
| `api:cli-generate` | `api` | pw generate |

## requirement:derived-asset-pipeline

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
  cache_control: the stable-name default shipped, as public, no-cache with a strong per-representation validator
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
  - a hashed-name or runtime-resolved variant, so immutable caching is still unavailable and every asset costs a revalidation
  - a stylesheet url() inside a template style block, deferred rather than open
  - source maps, which no build emits today
  - the documentation, which still describes the authored tree as the served one
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

## data:project-config

popcornweb.toml contains only pw build and development tooling configuration; runtime application settings belong to configbind inputs.

```yaml
file: popcornweb.toml
schema:
  project:
    name: myapp
    main: ./cmd/myapp
    toolchain: tinygo or go, defaulting to tinygo
    database: sqlite, postgres, or mysql, defaulting to sqlite
  dev:
    watch:
      includes: []
      excludes: []
    idp:
      enabled: false
      config: devidp.toml
      port: 0 for an automatically reserved loopback port; api:cli-init writes a fixed one, because the issuer it appears in is part of the account identity
    otel:
      enabled: true
      port: 0 for an automatically reserved loopback port
      max: 0 for the system:localotelviewer retention default
  generate:
    handlers: [handlers] as scaffolded, per decision:explicit-generation-sources
    templates: [handlers, templates] as scaffolded, because a page template sits beside its handler
    queries: [queries] as scaffolded
    config: [cmd/myapp] as scaffolded
    pages: [pages] as scaffolded for a project with a concept:page-tree, and empty otherwise
    dynamo: [records] as scaffolded for a project with requirement:dynamodb-store, and empty otherwise
  migration:
    dir: migrations
    auto: true for api:cli-dev only
  seed:
    auto: true for api:cli-dev only
  assets:
    tailwind:
      enabled: false
      input: assets/app.css
      output: public/generated/app.css
      minify: true for api:cli-build
optional_extensions:
  - generated output rules
  - generated test policy
  - build tags and targets
  - build output location
rules:
  - api:cli-generate reads each source kind only under the generate purpose that owns it, and warns about a .pw.html, .pw.sql, or .pw.dynamo outside its purpose
  - every generate purpose key is required except generate.pages and generate.dynamo; an empty list states that the purpose generates nothing
  - a missing generate.dynamo means the empty list, for the same reason generate.pages does; requirement:dynamodb-store is a capability a project acquires rather than one it always had
  - project.database names the SQL engine only, and says nothing about requirement:dynamodb-store, which is configured at runtime and never here
  - a missing generate.pages means the empty list, because a project scaffolded before requirement:discovered-page-routing has no concept:page-tree and no way to acquire one silently
  - a generate.pages entry is a tree root, so it is neither nested in another root nor listed under generate.templates or generate.handlers
  - the scaffolded directory names are defaults, not identity: handlers and pages are what api:cli-init writes, and every consumer reads the purpose list instead of the name, so renaming a tree is moving the directory and editing its entry
  - a generated package name follows the directory it is in, so a renamed tree compiles without an edit to its sources
  - a generate entry is relative, names an existing directory, and is neither duplicated nor nested inside another entry of the same purpose
  - one generate.templates entry holds the requirement:nested-html-templates document shell, and a second one is an error
  - api:cli-dev regenerates from the generate purposes but watches per decision:developer-loop-watch-scope
  - dev.watch.includes adds relative files or glob patterns, and dev.watch.excludes skips directory subtrees
  - project.toolchain records the compiler api:cli-init scaffolded for and rejects any other value
  - a missing project.toolchain means tinygo, because every project scaffolded before the key used api:serve-mux
  - project.database records the requirement:database-engine-selection engine and rejects any other value
  - a missing project.database means sqlite, because it was the only engine that existed before the key
  - project.database is a generation input, not a runtime one; the effective engine still comes from the rule:rdb-dsn-resolution scheme, and the two must agree
  - migration.dir locates data:migration-source and is a tooling path, not a runtime database value
  - migration.auto only enables the api:cli-dev apply step and never enables application startup apply
  - seed.auto only enables the api:cli-dev reseed step and never seeds from application startup, api:cli-migrate, or a build
  - seed.auto has no directory key beside it; the datasets are the api:cli-seed default location, and its --dir flag stays the way a one-off run points elsewhere
  - dev.idp only affects api:cli-dev and locates data:devidp-config
  - dev.idp.port defaults to an automatically reserved port because api:cli-dev injects the resolved issuer into the application
  - dev.idp.enabled true requires the data:devidp-config file to exist
  - dev.otel only affects api:cli-dev and configures requirement:dev-telemetry-viewer
  - dev.otel.port defaults to an automatically reserved port because api:cli-dev injects the resolved endpoint, as it does for dev.idp
  - dev.otel.max bounds retained records per signal and zero keeps the viewer default
  - relative paths resolve from the config file directory
  - unknown keys are errors
  - command flags override config values
  - missing config is an error except for api:cli-init
  - server, session, security, middleware, and observability runtime values are forbidden, and so is a database connection value; project.database names an engine, never a DSN or a credential
  - enabled Tailwind validates requirement:tailwind-css-integration and decision:tailwind-host-toolchain
  - Tailwind plugins and their options belong to the CSS entry through requirement:tailwind-plugin-integration
  - the CLI must already be available from the entered Devbox environment
runtime_configuration:
  owner: api:runtime-configuration
  file_selection: policy:config-file-resolution
  inputs:
    - TOML selected by configbind
    - environment
    - CLI flags
```

## data:public-asset-manifest

One build-produced record per served URL, listing every representation behind it with the bytes, digest, and cache metadata api:public-asset-middleware answers with.

```yaml
scope: concept:derived-public-tree, every file under dist/public
entry:
  url: mount-relative slash path, the value a rewritten reference names
  source: authored path it came from, or absent for a generated artifact
  cache_control: the header to send, per requirement:derived-asset-pipeline caching
  representations:
    - path: file under dist/public
      media_type: Content-Type to send
      content_encoding: empty or zstd
      bytes: encoded length
      sha256: digest of the emitted bytes
      etag: quoted strong tag derived from that digest
      preference: build-declared order among media types of one URL
form:
  emitted: generated Go table beside the embed, so nothing parses a manifest at startup and TinyGo pays no decoder
  also_written: a JSON copy for api:cli-doctor and for a caller inspecting a build, never read at runtime
  determinism: identical inputs produce identical entries in a stable order, so policy:generated-artifacts --check compares it
rules:
  - a URL with no entry is 404 even when a file exists under dist/public, so serving is manifest-driven rather than filesystem-driven
  - an entry with no representation is a build error
  - each representation carries its own strong ETag, per policy:public-asset-negotiation
  - the digest is over emitted bytes, so an identity and a zstd representation never share a tag
  - the middleware computes no digest and reads no file metadata per request, which removes the current per-request sha256 and read
  - the manifest is embedded with the tree it describes and is never fetched or reloaded
  - a media-type set larger than one enables policy:public-asset-media-negotiation for that URL only
open_questions:
  - whether the local-override path of policy:public-asset-resolution keeps a manifest at all, or drops to today's per-request digest
  - whether an entry records its transform and settings, which would make a stale-output diagnostic possible
```

## decision:asset-transform-toolchain

Every converter reachable from system:pw-cli is pure Go or a wasm module a pure-Go runtime executes; anything else is an external tool on the decision:tailwind-host-toolchain terms.

```yaml
status: accepted 2026-08-04
constraint:
  source: requirement:cli-distribution builds with cgo disabled and cross-compiles five targets from one runner with no C toolchain
  consequence: a cgo image codec would end single-runner cross-compilation for every target at once, which is a larger loss than any encoder is worth
options_in_order:
  in_process_pure_go:
    preferred: true
    identity: covered already, because the generation input hash includes the generator executable
  in_process_wasm:
    form: an encoder compiled to wasm and run by a pure-Go runtime inside pw
    cost: encode throughput and memory against a native encoder, paid once per asset and then cached by the conversion cache
    identity: the module bytes ship inside the executable, so the executable hash still covers it
  external_binary:
    form: a Devbox-pinned tool invoked per conversion, as Tailwind already is
    terms: reject a missing or incompatible tool before writing anything, and never install or download implicitly
    identity: the resolved path and reported version are hashed explicitly, because the executable hash does not see them and a silent tool upgrade would be a --check failure with no stated cause
javascript_and_css:
  chosen: esbuild, a pure-Go library, for the typescript build and the stylesheet minify
  fit: its metafile lists what it read, which is the read set requirement:derived-asset-pipeline reports, so nothing infers a dependency graph
  status: adopted; a css module imported by an entry becomes the companion file the head contribution loads
images:
  chosen: cwebp and avifenc, on the external-binary terms above, with the resolved path and reported version in every cache key
  provisioning: the api:cli-add images capability writes both packages into devbox.json and turns the conversion on together, so a project cannot end up with the switch and not the tools
  platform_fallback:
    tool: sips, which ships with macOS
    writes: lossy avif only
    does_not_write: webp, which it reads only; measured 2026-08-04, where a webp conversion exited zero and produced no file
    lossless_is_a_lie:
      measured: 2026-08-04, sips accepts "formatOptions lossless" and writes a lossy file anyway; a png round tripped through it returned with pixels differing by up to 8 across most of the image, where avifenc --lossless returned byte-identical
      consequence: the candidate list is per axis as well as per format, and sips is unreachable from the lossless one
      why_it_matters: a png is authored exact, so a screenshot, a diagram, or a logo re-encoded lossily is a visible defect that no size check and no diagnostic downstream would catch
    consequence: the candidate list is per format and per axis rather than per platform, and a listed tool that cannot honor either would be worse than no fallback
    identity: sips carries no version of its own, so the OS is what identifies it in a cache key
  axis_is_the_source's:
    rule: a png converts on the lossless axis and a jpeg on the lossy one, for both webp and avif
    avif_is_capable: avif has a lossless mode and avifenc --lossless is genuinely lossless, so preferring avif for a png costs nothing in fidelity
    stock_mac_consequence: with neither encoder installed, a jpeg still gains a lossy avif variant and a png gains nothing, which the build report names per file
  no_encoder:
    behavior: the conversion declines with a reason and the authored image ships as written
    why_not_a_failure: an unconverted image is a larger page rather than a broken one, unlike a script build, whose absence would leave a page naming a file nobody produced
    reported: rule:production-readiness-checks, as a warning naming the formats this machine cannot write
  measured_2026_08_04:
    what: the cgo-free Go encoder available today, nativewebp v1.3.0, lost the size comparison against Go's own png encoder on a solid fill, a checkerboard, a gradient, and noise, at both default and best compression
    consequence: an in-process encoder that always declines is an encoder that does nothing, so the external tier is where this actually lands
    contrast: libwebp beat png on every one of the same images, including noise
  refused: a wasm-backed encoder whose loader prefers a system library when one is present, since two machines would then produce different bytes and --check would fail on the difference between two correct builds
  criteria:
    - cgo-free and clean on every requirement:cli-distribution target
    - encode quality and speed acceptable at build time, since a cold conversion is serial
    - license compatible with the Apache-2.0 artifact and its existing MPL-2.0 notice
    - webp first, because policy:asset-transform-matrix needs it for both the png and the jpeg case, and avif only where it earns its bytes
  note: decode is stdlib for png and jpeg, so only the encode side is open
rules:
  - no codec, compiler, or wasm runtime enters the application runtime; every one of them is a host build dependency of system:pw-cli
  - a project registering no transform links none of it at runtime and pays nothing at build time
  - a conversion is deterministic for one toolchain version, and a version change is a hashed input rather than a silent difference
```

## policy:asset-transform-matrix

What a source file becomes, whether its authored bytes still ship, and what else the conversion drags with it, stated per file kind because the answers disagree.

```yaml
axes:
  source_ships: whether the authored bytes reach concept:derived-public-tree
  url_change: whether a template reference has to be rewritten, which decides whether a generation hook is involved at all
  extra_outputs: files the build writes that no reference names
  head_effect: whether serving the result needs a tag the author did not write
  negotiation: whether more than one representation lives under one URL
kinds:
  css:
    becomes: minified css, with static url() references rewritten to whatever their targets became
    source_ships: false
    url_change: false
    hook_needed: false
    reason: same name and same media type, so the tree walk replaces the bytes and no template is touched
    extra_outputs: none
    head_effect: none
    url_rewriting:
      why_here: the upstream seam rewrites template attributes and excludes css by its own non-goals, so a background-image can only be reached by the stylesheet pass this project owns
      scope: a static url() token in a stylesheet under the authored tree
      resolution: relative to the stylesheet's own location, never to the page that loads it
      out_of_scope: a url built from a custom property or any value not literal at build time, which is left alone and reported
      quoting: the token ends at the parenthesis outside quotes, since taking the first one would cut url("a (1).png") in half and emit a stylesheet that no longer parses
      ambiguous_absolute: an absolute reference matching two sources by suffix is left alone, because the mount prefix is runtime configuration and guessing would rewrite the wrong file
      inline_style_blocks: deferred 2026-08-05; a style block inside a template is not covered, and system:tinybind already rewrites those blocks for scoped styles, so a url pass there has a home and no design
      image_set: not generated, because policy:public-asset-media-negotiation answers the same question on the response and needs no css syntax
  png:
    becomes: webp, lossless axis; an avif variant, when produced, is lossless too
    source_ships: false when every reference to it was rewritten
    url_change: true
    hook_needed: true
    reference_sites: img src, and a css url() the stylesheet pass rewrites; nothing else
    naming: replace, so a.png becomes a.webp; the append form of the upstream seam is not used here because the source is not kept
    skip: a result larger than the source is a first-class skip with a reason, and the reference stays on the png
    negotiation: none by default; avif lossless is often no better than webp lossless on flat images
  jpeg:
    becomes: webp
    source_ships: false when every reference to it was rewritten
    url_change: true
    hook_needed: true
    reference_sites: same as png
    avif:
      produced: optional, as an additional representation of the same URL rather than a second URL
      attaches_to: whatever URL the image ends up at, converted or not, so a machine that can write avif and not webp still has something to offer under the URL the page already names
      chosen: per request from Accept, per policy:public-asset-media-negotiation
      why_the_server_and_not_the_markup: expressing the fallback in markup means a picture element, which this policy refuses to generate, so the choice has nowhere to live but the response
      cost: the URL says .webp while the bytes may be avif, so Content-Type is the only truth
  svg:
    becomes: optionally minified svg
    source_ships: false when minified, true otherwise
    url_change: false
    negotiation: none
    note: already covered by policy:public-asset-precompression, so the zstd sidecar is the larger win
  typescript_and_javascript:
    becomes: a bundled js for a ts entry, or a minified js for an authored js
    source_ships: false
    url_change: true for ts, false for a js minified in place
    hook_needed: true for ts; the js minify is tree-walk work like the css one
    naming: replace, so app.ts becomes app.js
    output_form:
      chosen: an es module, which is what a browser entry point is today and what the authored source is already written as
      requires: type=module on the tag
      why_not_decided_from_the_tag: the seam converts one distinct value once and replays it for every occurrence, so an output chosen from a template position would serve one page's answer to another; the transform is forbidden from reading the position for exactly this reason
      enforced_by: a build-time scan of every authored template, which refuses a built entry under a classic tag and names the file and line
      why_a_scan_is_enough: the constraint cannot be decided per occurrence and does not need to be; it only has to be checked, and every template is readable at once from the build
      rejected: an iife wrapper, which would run under either tag and silently give up top-level await to avoid a check that costs one pass over the templates
      unbundled_js: an authored js is transformed rather than bundled, so a module stays a module and its imports are untouched
    read_set: imports mean the digest of the named file is not the input; every file the build read is reported and hashed
    extra_outputs:
      source_map: written and named by no attribute
      css_companion: a css module imported by the entry produces a stylesheet the page must also load
    head_effect: the css companion needs a link the author never wrote, contributed by the conversion itself through the head field system:tinybind added in v0.3.5
  already_compact:
    kinds: webp, avif, woff2, mp4, wasm, archives
    becomes: itself
    source_ships: true
    action: verbatim copy, no precompression per policy:public-asset-precompression
  text_data:
    kinds: html, json, txt, xml, webmanifest, map
    becomes: itself
    source_ships: true
    action: copy and precompress
image_scope:
  in: img src, which is the only element this policy converts
  css: a url() the stylesheet pass rewrites, which reaches background-image without any markup change
  out: link rel icon, meta content, an image a script builds, and anything else naming an image, all of which pass through verbatim
  another_origin: a reference carrying a scheme, a protocol-relative prefix, or a data url is never claimed by a hook, since claiming it means resolving it under public and failing generation over a URL that was always correct
  no_picture:
    rule: the build never produces a picture, a source, or any other element the author did not write
    reason: wrapping an img changes descendant and sibling structure, so css combinators, flex and grid item identity, and structural javascript can all stop matching, and the build sees neither the global stylesheet nor the scripts to warn about it
    author_written: a picture the author wrote is ordinary markup; its img src is converted like any other and its source elements are left alone
  srcset:
    status: refused 2026-08-05
    expressible: a rewrite replaces the whole attribute string, so a transform could parse the descriptor list and reassemble it with no upstream change
    why_not: every URL in the list is a separate conversion and a partial failure has to leave the whole list authored, which is a second failure mode for an attribute a project writes when it is already making density decisions by hand
    consequence: a srcset keeps naming authored files, and those files stay in the tree because source_retention sees them
source_retention:
  rule: an authored source is dropped only when every reference the build can see was rewritten, and kept otherwise
  detection: a literal occurrence scan over the authored tree and the templates reports every remaining mention of a dropped URL
  unrewritable_reference: an occurrence the build cannot rewrite, such as one in a script or a meta tag, keeps the source in the tree instead of failing the build
  reported: both the drop and the retention are named in the build report, since a silently retained source looks like a conversion that did not happen
rules:
  - a kind whose URL does not change needs no template reference and therefore no generation hook, which is why the tree walk exists beside the hook
  - one source converts once no matter how many sites name it, so an image referenced by both an img and a stylesheet produces one output and one manifest entry
  - a transform never both rewrites a reference and leaves its source in the tree, unless source_retention kept it deliberately
  - skip is a result with a reason, not an error and not a silent no-op, and a skipped conversion is cached so a losing encode runs once
  - classification is this project's judgment; the upstream seam classifies nothing and ships no format table
  - a name never depends on encoder settings, per decision:derived-tree-development
  - a lossless source stays lossless in every representation of it, so an encoder that cannot hold the axis is not used for that source rather than used with a degraded result
  - determinism is per kind: identical bytes and settings produce identical outputs and identical names
non_goals:
  - generating a picture element or changing the element tree in any way
  - resizing, art direction, and density variants, which are authoring decisions rather than build ones
  - font subsetting
  - rewriting references inside javascript or markdown; css is in scope only through the stylesheet pass above
  - conversion at request time
```

## policy:generated-artifacts

Generated Go files are reproducible application build inputs owned by api:cli-generate.

```yaml
pattern:
  ordinary: "{source-base}_pw_gen.go"
location: beside the owning Go, .pw.html, or .pw.sql source
contents:
  - request binders and OpenAPI fragments
  - typed HTML renderers
  - typed SQL functions
  - optimized serializers
  - optional generated tests
  - main bootstrap blank imports for document and public registration
rules:
  - include a generated-code header
  - never edit manually
  - never begin generated filenames with an underscore
  - exclude from version control with the init-scaffolded **/*_pw_gen.go ignore rule
  - recreate during application builds before Go compilation
  - api:cli-generate --check must pass in CI
  - replace atomically
  - delete stale package-local SQL runtime files removed by decision:tinybind-sql-runtime
authority:
  source: handwritten Go types, handlers, literal routes, .pw.html, and .pw.sql
  generator: pinned system:tinybind version
```

## policy:public-asset-resolution

Public asset lookup preserves source consistency while allowing an explicit working-directory override of embedded assets.

```yaml
production:
  source_order:
    - when server.public.read_local is true and ./public contains the original regular file, select the local layer
    - otherwise select the embedded PublicFS layer
development:
  source: decision:development-public-assets
path:
  - strip the normalized configured mount prefix
  - percent-decode once through net/http request semantics
  - clean as a slash-separated relative fs.ValidPath
  - reject empty ambiguity, dot segments, backslashes, NUL, and traversal
representation:
  - inspect only the selected layer
  - choose {path}.zstd when policy:public-asset-negotiation allows it and the matching sidecar exists
  - otherwise choose the original file
security:
  - reject a symbolic-link local root, symbolic links below it, and non-regular local files
  - never fall back to an embedded sidecar for a local original
  - never expose directory listings, dot-prefixed path segments, or .zstd URLs
  - resolve a directory only as its index.html when present
```

## rule:production-readiness-checks

The PW05xx data:diagnostic-check entries: the pre-launch checklist as something that runs, so the list cannot quietly go stale the way a documentation page does.

```yaml
premise:
  form: the checklist a project would otherwise read once and forget, expressed as checks against the diagnosed token
  scope: dev_only, so the whole group is silent while the token is dev
  boundary: readiness here means what the framework configured; it says nothing about capacity, backups, or infrastructure
exposure:
  openapi-exposed:
    trigger: data:server-runtime-config openapi.enabled true
    severity: warning
    reason: policy:operational-endpoints protects it like an application route, so this asks whether that was intended rather than declaring a hole
  debug-logging:
    trigger: observability minimum_level below info
    severity: warning
    relation: the same condition rule:configuration-advisories reports as verbose-log-level-outside-dev, listed once and cited here
assets:
  tailwind-minify-off:
    trigger: assets.tailwind.enabled true with minify false
    severity: warning
    reference: requirement:tailwind-css-integration
  stylesheet-stale:
    trigger: the generated CSS older than a .pw.html or CSS source it is built from
    severity: error
    reason: flow:tailwind-css-build failing a production build on stale output is the rule; this reports it before the build
  precompression-stale:
    trigger: a public asset newer than its .zstd sidecar, or a sidecar whose source is gone
    severity: warning
    reference: flow:public-asset-build, which api:cli-build runs, so this fires on a tree built by hand
  public-serving-from-disk:
    trigger: server public.read_local true
    severity: warning
    relation: the same condition as local-public-read-outside-dev, cited rather than duplicated
crawlers:
  status: deferred until the framework owns the artifacts
  robots-missing-or-permissive:
    intent: a non-prod deployment serving anything other than a disallowing robots.txt, and a prod one serving none
    blocked_on: no framework-owned robots.txt exists yet, so nothing declares what is expected
  sitemap-and-social-metadata:
    intent: a declared sitemap route and per-page social metadata
    blocked_on: data:route-table gives the page list, and the metadata shape is not designed
rules:
  - a check in this group cites the catalog that owns the condition instead of restating it, so one setting produces one finding
  - the group is a view over checks rather than a second severity system; nothing here is fatal at startup
  - a deferred entry stays listed with what blocks it, because the value of the checklist is that its gaps are visible
```

## rule:project-integrity-checks

The PW01xx data:diagnostic-check entries: whether the project's declared shape, its toolchain, and its generated artifacts still agree with each other.

```yaml
project_shape:
  main-package-missing:
    trigger: data:project-config project.main names a path that does not exist or is not a main package
    severity: error
    reason: every other host command builds it, so this failure is reported once here rather than as five confusing ones
  migration-dir-missing:
    trigger: data:project-config migration.dir names a directory that does not exist while the database capability is present
    severity: error
    remedy: api:cli-add database, which writes the directory and its starter schema
  generate-purpose-empty-with-sources:
    trigger: a generate purpose lists no directory while sources of its kind exist in the project
    severity: warning
    reason: decision:explicit-generation-sources means an unlisted directory is silently not generated from
generated_artifacts:
  orphan-generated-file:
    trigger: a {source-base}_pw_gen.go whose .pw.html or .pw.sql source no longer exists
    severity: error
    reason: the orphan still compiles and its registrations still run, so a deleted page keeps serving and a deleted query keeps building; nothing else in the toolchain reports this
    remedy: delete the generated file, which api:cli-generate does for a source inside its purpose
    note: this is the failure mode most specific to generating beside the source, which is why it is an error rather than a warning
  generated-older-than-source:
    trigger: a generated file older than the source it was generated from
    severity: warning
    remedy: pw generate
    relation: api:cli-generate check mode is the authority on content drift; this check is the cheap timestamp form that also fires when the check cannot run
  generated-outside-purpose:
    trigger: a generated file, .pw.html, or .pw.sql outside every generate purpose
    severity: warning
    reference: the same condition api:cli-generate warns about, reported here so one command sees it
  generated-files-not-ignored:
    trigger: the project is a git work tree and *_pw_gen.go is neither ignored nor tracked consistently
    severity: note
    reason: policy:generated-artifacts makes them reproducible output, and a project should say once whether it commits them
toolchain:
  go-version-mismatch:
    trigger: the Go version in devbox.json disagrees with the go directive in go.mod
    severity: warning
    reason: the build that succeeds in the shell and the build that succeeds in CI are then different builds
  tinygo-baseline-unmet:
    trigger: data:project-config project.toolchain is tinygo and the pinned TinyGo version is below decision:tinygo-042-baseline, or its supported host Go range excludes the pinned Go
    severity: error
    reference: rule:tinygo-runtime-compatibility
  outside-devbox-shell:
    trigger: devbox.json exists and the current process environment is not the devbox shell
    severity: note
    reason: api:cli-dev expects the tools that shell provides, so a missing tool later is easier to read as this
  declared-service-missing:
    trigger: configuration selects a service, such as a Valkey endpoint or a session backend needing one, while devbox.json declares no such service
    severity: warning
    remedy: api:cli-add redis-valkey
    reason: api:cli-add and doctor are a pair; a capability added by hand is exactly where configuration and dependency separate
  tailwind-toolchain-missing:
    trigger: data:project-config assets.tailwind.enabled is true while devbox.json pins no decision:tailwind-host-toolchain package, or the configured input file does not exist
    severity: error
    reference: requirement:tailwind-css-integration
  port-unavailable:
    trigger: the configured server port is already bound on loopback
    scope: the dev token only, because another host's port says nothing about this one
    severity: warning
    bound: one non-blocking local check; no remote address is contacted
rules:
  - every check here reads files and the process environment only, per decision:host-side-diagnostic-analysis
  - a check about a capability is skipped when requirement:incremental-project-capabilities reports the capability absent, so a project without a database is not told about migrations
  - no check inspects application Go code for style or correctness, per the requirement:project-diagnostics scope boundary
```

## system:tinybind

TinyBind is the generated binding, configuration, response, validation, streaming, and OpenAPI engine wrapped by the Popcorn Web public APIs.

```yaml
module: github.com/shibukawa/tinybind-go
html_template_baseline: v0.1.15
html_async_baseline: v0.1.20
html_live_baseline: v0.2.8, required by requirement:live-html-rendering; v0.2.7 introduced live boundaries and v0.2.8 answered the first of the integration requests raised against them
html_update_baseline: v0.3.3; v0.3.0 added the htmlupdate package, v0.3.1 handed the asset and every name to the caller per requirement:tinybind-runtime-ownership, v0.3.2 carried head on the action response, and v0.3.3 closed every remaining seam of requirement:tinybind-update-composition-seams and made CSRF module native; adopted by decision:update-runtime-convergence
route_tree_baseline: v0.2.6
current: v0.3.5, which carries the reference-hook head contribution and concurrent conversion, renames the redraw handler to Options.Redraw, drops the module-owned protocol version, and carries the generator crash fix below and, from requirement:module-native-csrf, writes the token into every unsafe form itself
newer_tag_not_taken: v0.3.4 exists and changes only that repository's knowledge catalog and its author documentation, so the pin stays where it is until a code change earns the bump
public_wrappers:
  - api:request-binding
  - api:html-response
  - api:api-response
  - api:typed-stream
  - api:problem-response
  - api:runtime-configuration
defects:
  unguarded_position_lookup:
    status: fixed in v0.3.2, which is why this module first moved off v0.2.10
    was: three call sites dereferenced Fset.File(f.Pos()) after guarding f, pkg, and Fset for nil, and that call is the one that returns nil
    sites: generator/plan.go, generator/configbind.go, and generator/dynamobind.go
    fix: each now takes the handle and checks it, which is what generator/configbind_doc.go already did three files away
    symptom_it_removed: a nil pointer dereference in go/token.(*File).Name, taking the calling process down
    trigger: a Go file in a generated directory that does not parse, most often a zero-byte one an editor has created and not yet written into
    mechanism: packages.Load returns a syntax entry for a file it could not parse, that entry reports token.NoPos, and a FileSet lookup of NoPos is nil; measured against golang.org/x/tools with Popcorn Web out of the picture on 2026-08-02
    downstream_containment_kept: api:cli-generate unparsable_source and its recover stay, because the pre-check names the file and the line where the generator would only name the directory, and the recover bounds every generation panic rather than this one
asset_transform_seam:
  shipped: v0.3.1, in one commit that carried the hooks, the cache, the produced files, and the recorded dependency file together; read against the upstream tree on 2026-08-04
  correction: this said v0.3.3 until 2026-08-04, which was the version pinned here rather than the version that shipped it; nothing depends on the difference, since the pin is later than the seam
  design_lives_upstream: its build-time-asset-transforms concept, its element-reference-hook and derived-asset-generation requirements, and its transform-seam-ownership decision, none of which are restated here
  surface: GenerateOptions.ReferenceHooks and StrictReferenceHooks, ConversionCacheDir, DerivedAssetDir, ArtifactDerivedAsset, and GenerateResult Produced, Rewrites, and ReadSet
  results: value and skip; the markup-replacing element result is designed and not built
  head_contribution: v0.3.5 added ReferenceResult.Head as link, script, and style entries, deduplicated per component, cached with the conversion, and restricted so a hook cannot rewrite the document
  concurrency: v0.3.5 added GenerateOptions.ConversionWorkers, excluded from the hashed options because it changes wall clock and never bytes
  bookkeeping: produced files are declared artifacts, and the read set is recorded per run so an edited import regenerates
  division: the module matches, rewrites, and records; the caller owns every codec, format, name, and switch, per requirement:derived-asset-pipeline
  module_non_goals: bundling, minification, a format table, and any runtime negotiation
generator:
  extensible_analysis: requirement:httpbinder-extensible-route-analysis
  openapi:
    - package fragments register during import
    - AssembleOpenAPI returns one deterministic merged JSON document; YAML output was dropped
  configuration:
    - generated definitions register during import
    - ScaffoldTOML and ScaffoldEnv merge every package definition
  html:
    - .pw.html components generate immutable htmlbind.Fragment values
    - components with an unnamed slot also generate htmlbind.Wrapper binders
    - api:render-html-chain composes wrappers around a leaf
    - "`external async` declarations bind Go functions returning a value and an error"
    - "`async T` parameters and record fields become htmlbind.Pending[T], surfaced by api:async-html-value"
    - await, fallback, and recover clauses compile to boundaries described by api:html-boundary-protocol
    - the generated plan carries a constant HasAwaitBlock flag used by decision:automatic-async-render-selection
    - the async render path emits placeholders and bare fragments; completion framing and the client runtime belong to the framework
    - "`external live` declarations bind Go functions of the shape func(ctx, args...) iter.Seq2[T, error], with the context mandatory rather than optional"
    - a live binding sits in an ordinary await clause, so there is no second clause keyword and one clause may mix a live binding with a settle-once one
    - "RenderChain renders a live boundary from its first delivery; RenderChainAsync commits the first delivery and unsubscribes; RenderChainLive keeps delivering and does not end"
    - the entries that must answer bound how long a boundary may show nothing, and running out leaves the fallback rather than rendering recover
    - HasLiveBlock reports whether a chain keeps changing after the document ends, a subset of HasAwaitBlock
    - Content.AppendJSON writes one delivery as a JSON record, escaped for a script context as well as a JSON one
    - boundary ids became positional, so a nested boundary is tb-1-1 and the same chain executed again produces the same ids; api:html-boundary-protocol carries what that changes here
    - "the live entry does not enforce that the body writer is discarded, so passing a real writer produces an endless document response"
    - a live failure reaches the error reporter after the delivery lock is released, from v0.2.8; before that a blocking reporter held the clause's goroutines
    - nothing states which boundary is live, so requirement:live-boundary-liveness-signal is still answered by the framework's own bookkeeping
    - a live render executes the whole composed chain, so requirement:live-mode-plan-slice is still paid per reconnect
  html_update:
    - the htmlupdate package holds every net/http concern of partial updates, so htmlbind stays free of it and generated plans keep working on TinyGo and WebAssembly targets
    - every layout and page of a rendered chain is an update boundary automatically; an ordinary component call is not, and the document shell never is
    - a boundary must render exactly one root element, and a component that cannot is simply not a boundary rather than a generation error
    - two keyed digests per boundary, the frame validator over its own bytes excluding nested boundaries and the input validator over its declared parameters; the frame one is the authority for omitting a boundary
    - a delta skips transmission and never execution, so only a component opting into output caching skips its own render
    - Options carries the validator key, the header prefix, the path prefix, the build identity, and the manifest size cap, and pw wraps it as api:html-update-options
    - Negotiate resolves anything unrecognized to a complete document, which is what lets a live token share the header per decision:update-runtime-convergence
    - Mount installs the runtime asset and the redraw endpoint under one path prefix; pw serves its own merged asset and takes only the redraw route
    - a reloadable modifier on a component declaration generates a typed query decoder and a registration value, consumed by requirement:reloadable-component-endpoint
    - Registry.Register panics on a repeated kind, because the kind covers name, parameters, and markup but not the package
    - WantsUpdate, WriteUpdate, WriteUpdateStatus, and WriteNavigate are the action-response surface requirement:action-response-update branches on
    - the generator gained a data attribute prefix option naming the boundary attributes, which pw sets to its own brand
    - from v0.3.1 a render option names the async placeholder element and the boundary identifiers from that same prefix, so one document no longer holds two spellings
    - from v0.3.1 the browser runtime source, its asset form, and its naming configuration are exported, and serving it is switchable, so a framework composes it into its own asset instead of copying it
    - the runtime is a factory reading its attribute prefix, header namespace, endpoint prefix, and installed name from that configuration; only the protocol version stays compiled in, and an empty installed name installs no global
    - the author-written preserve and ignore markers follow the configured prefix, so no application template carries the module's name
    - Mount takes a one-method router interface satisfied by api:serve-mux, registration returns an error beside a must-variant, and an options validator reports every unusable option at once
    - a failure callback receives every refused redraw with a kind, status, message, cause, and the component and instance it named, so a refusal reaches api:error-renderer and requirement:modern-observability
    - the redraw response carries a keyed ETag with a private, no-cache policy, so an unchanged region answers 304; the policy, the query bound, and the stream media type are all options
    - builtin element registration is unimplemented at v0.3.0 and lands in v0.3.3, so a framework-supplied element had no registration seam until then
    - a synchronous external declaring a leading context.Context receives the render context, and one returning html lowers to a slot, which was the interim shape planned for a framework CSRF element
    - style and script blocks extract to content-hashed files under a configured public directory, unused until requirement:component-asset-extraction sets the options
  route_tree:
    - the routetree package discovers a directory tree and writes the registrations, which is the opposite direction from the registered-router analysis above
    - one run covers one tree; requirement:discovered-page-routing wraps it and flow:page-route-generation drives it
    - reserved file names, generated file names, called symbols, five named blocks, and three whole-file templates are all configurable, which is the seam decision:page-render-binding uses
    - the render block carries every in-scope identifier by name, including a chain that is nil for a page with no ancestor layout, so a framework reshapes the call instead of renaming it
    - the router type and its constructor are their own symbols, separate from the package supplying Request, and an empty constructor name omits the constructor
    - the composer entry takes a configurable writer type and an optional request parameter, with imports derived from the signature
    - a query input declared with a trailing question mark binds a pointer, so an absent value is distinguishable from a zero one
    - the discovered tree reports its package list, which is what lets a framework run request binding over route packages
    - discovery skips files carrying tinybind's own generated header, plus any header prefix the run registers, and deliberately does not skip another tool's generated code
    - the emitted header is settable and pairs with an accessor returning the prefix to register, so a branded output cannot drift from what discovery recognizes
    - the failure entry a generated handler writes through is a symbol, so a framework naming it something else needs no template override
    - a package with no bindable model reports a wrapped sentinel error, so a loop over many packages can skip it with errors.Is
    - an action resolver answers a server-action name the current tree does not hold, which is what would let a flat template use one
    - htmlbind.Signatures and htmlbind.ActionRefs expose component parameter types and template action references for a framework generating around a template
  sql:
    - decision:tinybind-sql-runtime owns statement plans and shared execution
    - declared sql.exec, sql.one, sql.optional, or sql.many selects Exec or Query
    - incompatible SQL result contracts fail generation
    - a SQL dialect option selects the target engine, required from v0.2.2 with no default
    - the dialect carries the placeholder style, dollar for postgresql and question for mysql and sqlite
    - postgresql, mysql, and sqlite from v0.2.3, which covers every decision:server-sql-support-tier first-class engine
  dynamo:
    - the dynamobind runtime package and a generator mode, from v0.2.8, consumed by requirement:dynamodb-generation
    - a dynamo struct tag names the attribute and its options, and an unknown option is a generation error rather than a silently ignored string
    - each tagged type yields EncodeItem, DecodeItem, ItemKey, and a table definition constructor, with compile-time assertions against the runtime interfaces
    - codec emission is usage-directed, so a type gets only the halves a discovered call needs; the key builder and the table constructor are emitted whenever a partitionkey tag exists
    - the table constructor carries the name, the partition key, and the optional sort key; billing mode, capacity, and secondary indexes are left zero
    - dispatch is static, so no registry or init entry is emitted and an unused type links nothing
    - the operation helpers take a driver client argument and pass driver errors, retries, and page boundaries through untouched, which is why decision:dynamodb-no-runtime-abstraction wraps none of it
    - from v0.2.9 a query declaration file generates one named function per access pattern, consumed by requirement:dynamodb-typed-queries
    - from v0.2.10 the client is carried in the context and set with a client setter, so no entry of the package takes one
    - the same setter takes an optional table resolver function, run inside every runtime entry, which is the seam rule:dynamodb-table-naming installs
    - a declaration carries a required table clause, so a generated query names neither a client nor a table
    - a missing client is a named error rather than a panic, so an entry reached without the middleware fails as an ordinary error
    - the declaration suffix is configurable through DynamoTemplatePattern and the output file name through DynamoQueryName, so Popcorn Web brands both without renaming anything after generation
    - a declared query's attribute names are checked against the tags, and every attribute is aliased unconditionally so no reserved word reaches an expression literally
    - the string key-condition form remains as an unchecked escape hatch
    - table definition emission is suppressible as the named feature item-table, and the whole mode as item-codec
    - single-table design is a stated non-goal, so one struct owns one table
    - a version tag for optimistic locking and a ttl tag are proposed, the latter blocked on the driver
    - no update or condition expression is generated, and secondary index tags are deferred
    - no generation option selects a framework resolver, unlike the SQL executor resolver, because resolution moved into the runtime and left no generated call site to redirect
constraints:
  - a route tree directory name must be a legal Go import path element, per rule:page-directory-naming
  - generator executes with host Go
  - generated mapping path avoids runtime field reflection
  - route discovery analyzes same-package registrations recognized by versioned adapters
  - normal handwritten application code does not import TinyBind
compatibility:
  route_tree_v0_2_4: the routetree package and server-action lowering are additive, so the pin moves from v0.2.3 without touching an existing handler, template, or query
  route_tree_v0_2_5:
    additive: every new seam is additive and the default output is byte-identical, so the pin moves again without regenerating differently
    behavior_change: discovery now skips tinybind's own generated files, which removes registrations a run could previously read back out of a generated registry
  route_tree_v0_2_6:
    additive: the header and the failure selector become configurable with defaults matching what v0.2.5 emitted
    resolves_for_pw: api:cli-generate writes its own generated header, which the v0.2.5 filter could not recognize; registering the prefix is now the supported answer, so page tree output keeps the Popcorn Web brand
  sql_v0_2_2: the SQL dialect became a required generation input, so a run that discovers a .pw.sql without one is a configuration error rather than a silent postgresql assumption
  sql_v0_2_3: the sqlite dialect is additive and emits the question placeholders sqlite already generated through mysql, so naming it changes no generated output
  dynamo_v0_2_8:
    additive: dynamobind is a new package and a new generator mode, so nothing an existing project generates changes
    module_graph: the module now requires system:tinygodriver v1.1.3, because a runtime package imports the DynamoDB client rather than only an example doing so
  dynamo_v0_2_9:
    additive: the query declaration is a new source kind and a new output file, so a project generating only codecs regenerates identically
    answers: the downstream Popcorn Web request, whose allocation decision:dynamodb-framework-scope records
    closes: the read-path drift requirement:dynamodb-generation could not close on its own
  dynamo_v0_2_10:
    breaking: every runtime entry lost its client parameter and a declaration gained a required table clause, so v0.2.9 call sites and declarations both need editing
    scope_for_pw: nothing was released against v0.2.9, so the change costs an edit to these concepts rather than to a project
    size: about 37 KB on a TinyGo wasip1 build, from the context value and the assertion reading it back
    answers: the second downstream request, and answers it by removing the seam rather than adding one
  v0_3_2:
    taken_for: the unguarded position lookup above, which crashed api:cli-generate on a file an editor had created and not yet written into
    arrives_with: the boundary emission requirement:navigation-delta-rendering consumes, whose activation is opt-in per component except for generated route layouts, which take it automatically
    effect_on_pw: a concept:page-tree component now emits a boundary marker attribute and one update-manifest entry; the rendered document gains an attribute and loses nothing
    measured: one page tree fixture regenerated, and the rest of the suite passed unchanged, so no Popcorn Web source needed editing
    superseded_by: v0.3.3 and the adoption decision:update-runtime-convergence records, so the markers are no longer inert; requirement:module-native-csrf is the half taken first
  html_v0_1_15: generated HTML APIs are not source-compatible with earlier direct-writer output
  html_v0_1_19: async parameters and async render entry points are additive, so existing templates and call sites keep compiling after regeneration
  html_v0_1_20: Content.WriteTo narrows to the bare fragment and the module injects no client runtime, so an async caller must supply framing and a runtime it previously inherited
  html_v0_3_0:
    additive_on_the_wire: a project that never sends the render header renders and serves exactly as it did, so the pin moves without regenerating differently
    generated_output: boundary attributes and validators appear on layout and page roots, which changes generated markup for every page even when no update is enabled
    duplicated_here: the module shipped a browser runtime, a header namespace, and an endpoint prefix that overlap what this framework already owns; decision:update-runtime-convergence decides what happens to each
    not_a_drop_in: the shipped runtime was built for the upstream names, so adopting the transport without adopting the names would have cost an adapted copy of its source
    superseded_by: v0.3.1, so this version is never the one to pin
  html_v0_3_1:
    answers: requirement:tinybind-runtime-ownership in full, which is what makes the transport adoptable without a copy
    generated_go: unchanged, so the pin moves without regenerating differently
    breaking_for_a_direct_user:
      - the preserve and ignore attributes default to the module's short prefix rather than its full name
      - the query bound and stream media type constants were renamed as defaults, having become options
      - registration returns an error rather than panicking
    breaking_here: none, because nothing was released against v0.3.0
    upstream_correction: the embedded runtime was an interim shape its own rollout requirement had recorded, whose exit was never scheduled, rather than a reversed boundary; the effect downstream was the same and requirement:tinybind-runtime-ownership carries the correction
    still_interim_upstream: the module serves an asset by default and retires that only when its own runtime bootstrap selects and injects one; this framework declares caller ownership, so the default never applies here
  html_v0_3_2:
    additive: an action response gained a head field and the rest is documentation, so nothing generated or served changes for a project that does not use it
    action_head: each written region's own contributions are collected and deduplicated across the set; the browser already installed a delta's head before applying operations, so only the server was never filling it
    live_transport_confirmed: the module's document render settles a live boundary in place and finishes the response, and a second connection carries deliveries, which is this framework's own shape rather than a divergence
    live_token_still_absent: no live token is parsed on either side, and the shipped upstream runtime sends the navigation token for both the first connection and every reconnect; filed as a must-priority requirement recommending a live token, which is this framework's existing choice
    still_open: the redraw response carries no head, the slot-carried fragment head defect, and what a fragment response owes a caller it cannot deliver to, each filed as its own requirement rather than settled
  html_v0_3_3:
    answers: every remaining item of requirement:tinybind-update-composition-seams, and moves CSRF into the module
    live_mode: a live token of its own with its own negotiated mode, so subscriptions stay open only in that mode; termination reasons name final, live-pending, failed, done, and retry, a retry may carry a server-side delay hint, the head record carries the build, and a cancelled context closes as retry rather than done
    live_handoff: a response header, a delta body field, and a stream terminator each say whether a live connection is expected, and none appears when the page has no live boundary
    adopted_from_here: the done-versus-retry distinction, the build on the opening record, and resetting the attempt count on a healthy close were this framework's shipped behaviour, offered as input and taken
    live_validators: a delivery carries none and the opening delta does, which answers the question that item left open
    live_defect_fixed_upstream: the live entry had set subscriptions unconditionally, so an ordinary navigation delta on a live route never terminated
    redraw_head: the registry reports the head and assets of every published component for the shell to install once, and a redraw that contributes head announces it on a response header; the body stays a bare subtree
    asset_set: an asset value on the plan with fragment, wrapper, and merge accessors, readable before rendering and folded through slots
    slot_head: the plan reaches fragments carried in parameter structs, so head, sources, assets, and capability flags all fold; a project declaring no html parameter regenerates identically
    vary_axes: a composition reports the request properties its output varies on, so a response can set an honest Vary header rather than guessing
    builtin_elements: a framework registers hyphenated elements that lower to plan steps at generation time, with the value never entering template scope
    csrf: consumed by requirement:module-native-csrf
    protocol_version: deliberately left at 1, because nothing has shipped under it and spending a bump would cost the first real deployment one wasted fallback
  breaking_v0_3_3:
    hyphenated_elements: the namespace is closed, so a project writing Web Components must declare them; requirement:custom-element-registration carries what this framework owes projects
    cached_unsafe_form: a component holding an unsafe form can no longer be output-cached, which policy:csrf-protection records
    scaffolds_unaffected: no template this framework scaffolds writes a hyphenated element or caches a form, so a scaffolded project regenerates identically
  not_built_upstream_v0_3_3:
    - the opaque builtin element shape, declined because the trust assertion would move into framework code
    - a builtin element inside a head declaration, so head placement means only that the body position is refused
    - the embedded asset byte table and the caller-supplied URL function, which is what requirement:component-asset-extraction needs for a TinyGo target
    - a server-side lifetime bound calling the retry seam, left to the caller
    - the redraw head bound as an option, since registration cannot reach the options value
  known_flake_upstream: a superseded live delivery can report a stale value into a reused placeholder, reproduced on the baseline and predating v0.3.3, so a cancellation-ordering race rather than a regression
  other_v0_3_1_features:
    template_formatters: formatters for the html, sql, and dynamo template languages, unevaluated here and a candidate for api:cli-generate or a project check
    asset_transform_hooks: build-time rewriting of referenced assets through registered hooks, which is the seam requirement:component-asset-extraction would build on if that work is taken up
```

## api:cli-add

pw add installs a framework capability into an existing project, so a choice declined at api:cli-init is not a decision the project is stuck with.

```yaml
usage: "pw add [capability]"
requirement: requirement:incremental-project-capabilities
mode: decision:post-init-scaffold-wizard
inputs:
  capability: preselects the first wizard step; omitting it lists the capabilities the project still lacks
  answers: the capability-specific questions, asked in the wizard only
questions:
  capability: single-select over the capabilities the project does not already carry
  database_engine:
    asked_when: the answers reach the database, directly or through a dependency
    choices: sqlite, postgres, and mysql per requirement:database-engine-selection
    default: sqlite
  database_dsn:
    asked_when: an engine has been chosen
    default: the requirement:database-engine-selection DSN for that engine
  oidc_provider:
    asked_when: auth is selected
    choices: requirement:contrib-devidp local emulator, or an external provider left for the operator to fill in
    mode: oidc, the only authentication mode with an implementation
  review: lists every file to create, every configuration section to append, and every follow-up command
capabilities:
  devbox:
    writes:
      - devbox.json carrying the toolchain project.toolchain records, plus the Tailwind pin when it is enabled
      - devbox.lock
    consumer: api:cli-dev starts the services it declares, and skips the step entirely for a project without the file
  database:
    writes:
      - data:middleware-runtime-config rdb section in every environment configuration file present
      - the migration directory, holding the same data:migration-source scaffolded_version_1 api:cli-init writes when the project has none, which creates no table
      - the same commented-out starter .pw.sql api:cli-init writes, and the generate.queries entry that opens the purpose for it
      - data:project-config project.database naming the selected engine, which api:cli-generate reads as its SQL dialect
      - the development server package in devbox.json for a server engine, when the project has that environment
    dialect: the starter migration and .pw.sql are written for the selected engine, per requirement:database-engine-selection
    manual: the engine blank import, because the entry point is application-owned
    enables: data:migration-source, api:migration-runner, and .pw.sql generation
    leaves_alone: an existing migration set, which is the application's own schema
  redis-valkey:
    requires: devbox, because the answer writes nothing but a package in that environment
    writes:
      - Valkey package in devbox.json, which api:cli-dev exposes as the development server
      - the endpoint an application passes to requirement:contrib-auth-state-redis and to session.backend redis
  dynamo:
    requires: nothing, per requirement:dynamodb-store; it combines with any database answer including none
    writes:
      - data:dynamodb-runtime-config section in every environment configuration file present
      - the records directory holding one dynamo-tagged starter type and one .pw.dynamo declaration, and the data:project-config generate.dynamo entry that opens the purpose for both
      - the amazon/dynamodb-local package in devbox.json, which api:cli-dev exposes as the development server, following the redis-valkey model
      - the local endpoint and placeholder credentials in config.dev.toml only
    key_may_be_absent: generate.dynamo is optional like generate.pages, so this edit may have to add the key rather than replace it
    manual: the api:dynamo-package import, because concept:application-entry-point is application-owned
    enables: requirement:dynamodb-generation and requirement:dynamodb-migration
    writes_no_migration: the schema is the generated table set, per decision:dynamodb-desired-state-migration, so no migration file and no version range is involved
  auth:
    requires: database, because its login ceremony and allowlist tables live there whichever backend stores the sessions
    session_backend:
      installed: rdb, the backend that fits a project already carrying a database
      alternatives: api:cli-init offers the cookie and redis backends of requirement:state-storage-tiers
    writes:
      - rule:framework-owned-tables migration files for the session store and the authentication tables
      - data:authentication-runtime-config section in every environment configuration file
      - account resolver source that api:authentication-endpoints calls
      - data:devidp-config roster and data:project-config dev.idp when the local emulator is selected
    imports: the application already links plugin/auth through its account resolver, so only the api:session-backend-plugin blank import is printed as a manual step
  discovered:
    writes:
      - the concept:page-tree root with the same starter page, layout, and dynamic route example api:cli-init writes
      - the data:project-config generate.pages entry that opens the purpose for it
    detection: the generate.pages entries, because a tree no purpose lists is a directory nothing generates from
    key_may_be_absent: generate.pages is the one optional purpose, so this is the only capability whose edit may have to add its key rather than replace it
    manual: the api:page-registry Register call, because concept:application-entry-point is application-owned
    requirement: requirement:discovered-page-routing
  registered:
    writes:
      - the handler package, its flow:handler-registration mux and accessor, and one route example
      - the generate.handlers entry, and the same directory added to generate.templates, because a page template sits beside the handler that renders it
    for: a project scaffolded with the discovered-only answer of decision:page-router-scaffold-choice
    manual: the mux wiring in concept:application-entry-point
  tailwind:
    writes:
      - assets.tailwind section in data:project-config for requirement:tailwind-css-integration
      - pinned decision:tailwind-host-toolchain package in devbox.json
      - assets/app.css entry point
    manual: the stylesheet link belongs in the application-owned document shell, so it is printed rather than injected
    without_devbox: the requirement is printed instead of pinned, naming the standalone CLI and its minimum version rather than the Devbox package identifier, because there is no package list to write to
detection: the requirement:incremental-project-capabilities probes; no capability list is recorded anywhere
versioning:
  migration_version: the next free version in the project migration directory
  rationale: an application that already applied 00001 through 00007 must not have those renumbered, so no version range is reserved for the framework
  identity: the name stem published by the owning package, which makes the file recognizable at any version
configuration_edits:
  form: append the missing section to each existing environment configuration file
  reason: operator comments and hand-tuned values must survive the edit
  conflict: an existing section of the same name stops the command
rules:
  - require a data:project-config project root
  - refuse a capability the project already carries, naming the file that proves it
  - detect an installed capability by migration name stem or configuration section rather than by version
  - offer a missing required capability together with the one selected, and refuse the pair when it is declined
  - never rewrite or renumber an existing migration
  - never overwrite an application-owned file; report the conflict and stop
  - write nothing when any step would fail, so a partial capability cannot reach a project
  - run api:cli-generate after a capability that adds generated sources
  - print the commands that finish the installation, starting with api:migration-runner
  - reject a capability whose mode has no implementation, matching api:cli-init
relations:
  init: api:cli-init writes the same files for a new project, from the same capability catalog
  flow: flow:capability-addition
  layout: concept:project-layout
  sibling: api:cli-new adds sources rather than capabilities, and names them after what they are rather than after the router that serves them
exit:
  success: 0
  canceled_wizard: 0 with a canceled notice and no files written
  no_terminal: nonzero with usage
  already_present_or_conflict: nonzero with the path and the reason
```

## api:cli-generate

pw generate scans Go, .pw.html, and .pw.sql sources and emits all required application mapping and codec code beside its source.

```yaml
usage: pw generate [--check]
inputs:
  - pw.Parse[T] call sites
  - route registrations
  - .pw.html files
  - .pw.sql files
  - reachable JSON types
  - concept:page-tree roots, their reserved files, and their optional page.go
  - dynamo-tagged struct declarations and their dynamobind call sites
  - .pw.dynamo query declarations
flow: flow:generation-pipeline
discovery_scope:
  per_purpose: the data:project-config generate.handlers, generate.templates, generate.queries, generate.config, generate.pages, and generate.dynamo lists, per decision:explicit-generation-sources
  effect: a directory contributes only the artifact kinds whose purpose lists it, so a query directory is never analyzed for routes
  pages_unit: a generate.pages entry is walked as one concept:page-tree per flow:page-route-generation, not as a directory of independent sources
  fixed: the project.main directory and the project-root public.go
  required: the keys have no default, so a project without them fails to load
sql_dialect:
  source: data:project-config project.database
  effect: .pw.sql sources compile to the placeholder syntax of that engine, per flow:sql-generation
  no_default: the value is passed through rather than assumed, because a wrong dialect fails at the first query rather than at generation
  outside: warn and ignore a .pw.html, .pw.sql, or stale generated file found outside its purpose; Go sources are not reported
  consumers: api:cli-new derives its default destination from this scope, and api:cli-dev regenerates from it
artifacts:
  from_generate_handlers:
    - request binding
    - optimized JSON codecs
    - OpenAPI fragments
  from_generate_templates: typed HTML renderers
  from_generate_queries: context-based SQL functions
  from_generate_config: configuration and subcommand binding
  from_generate_pages:
    - compiled page and layout components
    - the route decoder of each page
    - the api:page-registry and data:page-route-table of each tree root
    - api:page-action-endpoint registrations
    - request binders for the route packages, so an action can call pw.Parse
    - no OpenAPI, per decision:dual-router-coexistence
  from_generate_dynamo:
    - item codecs, key builders, and table definitions, per requirement:dynamodb-generation
    - the decision:dynamodb-table-registry list in the project.main package
    - no SQL dialect input, because there is no engine variant to compile for
  from_every_purpose: data:route-table, the exported view of the same route analysis
  optional: generated tests
unparsable_source:
  rule: a Go file that does not parse is reported by name, line, and column, and its directory is skipped for that run
  reason: api:cli-dev regenerates the moment a file appears, so it routinely reads one an editor has created and not yet written into
  upstream_defect: system:tinybind walks such a file to a nil position and panics, found by generating over a zero-byte source on 2026-08-02
  containment: a panic anywhere in a generation request becomes an error, because one escaping would take the developer loop, the application it supervises, and the services it started down with it
  transient: the next watched change regenerates, so a file caught mid-save costs a message rather than a restart
check_mode:
  writes: none
  failure: generated content differs or is missing
behavior:
  - read a source only where the purpose that owns its kind lists its directory
  - walk each generate.pages root once, reporting every discovery problem in that walk rather than only the first
  - use the pw emitter of decision:page-render-binding for every page tree artifact, so generated pages call api:page-render-runtime rather than system:tinybind
  - run request binding over the packages a discovered tree reports, skipping the ones the generator reports nothing to generate for
  - register the Popcorn Web generated header prefix with every discovery pass, so nothing this command wrote is analyzed as a source on the next run
  - keep, per directory, only the artifacts whose purpose lists that directory
  - warn once per .pw.html, .pw.sql, or stale generated file found outside its purpose, naming the path and the key
  - use system:tinybind route and call analysis behind the pw API
  - process sources and packages in stable lexical order
  - stop on parse or generation error
  - format generated Go source
  - replace destination files atomically after all generation succeeds
  - emit {source-base}_pw_gen.go beside each source
```

## Review Checklist

- [ ] Scope is correct.
- [ ] Missing references are resolved.
- [ ] Policies and permissions are explicit.
- [ ] Generated output is not written back as source.
