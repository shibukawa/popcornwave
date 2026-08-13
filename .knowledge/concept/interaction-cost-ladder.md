---
id: concept:interaction-cost-ladder
type: concept
title: Interaction Cost Ladder
---
Interactivity in a concept:classic-web-style application is bought in ordered tiers, and a design takes the cheapest tier that answers the interaction rather than the most capable one.

```yaml
premise: the framework ships no hydration, client router, or client state store, so interaction technique is an application choice; this ladder is the map that choice reads
governs: what an application spends on the people using it; development tooling is priced separately by decision:dev-browser-runtime-scope, because its one user is the developer and its cost never ships
tiers:
  platform:
    cost: no dependency, no build step, no framework surface
    covers: dialogs, popovers, disclosure, client-side validation echo, navigation transitions, prefetch
    owner: browser
  css_components:
    cost: one Tailwind plugin per requirement:daisyui-integration
    covers: appearance and theming of markup the application already writes
    constraint: structure and accessibility semantics stay in application HTML
  server_fragments:
    cost: one swap library in the document shell
    covers: regions whose new content only the server knows
    surface: api:html-fragment-response under requirement:html-fragment-rendering
  authored_islands:
    cost: application-owned browser JavaScript
    covers: local state and events no server round trip can answer
    boundary: custom elements, matching how requirement:framework-script-assets registers the framework runtime; from system:tinybind v0.3.3 each one must be declared per requirement:custom-element-registration, so reaching this tier now costs a configuration entry as well as the JavaScript
    mutation_address: api:page-action-endpoint inside concept:page-tree, whose attribute the application still intercepts today; requirement:action-invocation-runtime is what moves that into the framework and requirement:scriptless-action-forms is what keeps the tier from costing the scriptless path
    authored_javascript_lives_in_the_component: the script block of system:tinybind v0.5.5 is where it is written and requirement:client-signal-registry is what mounts and releases it per instance, so this tier's cost is the JavaScript and no longer the plumbing around it
    handlers: requirement:component-script-event-binding lets the markup name an exported function, which is what removes the selector pairing an island otherwise writes for every control
  framework_updates:
    cost: one configuration section and one shell script reference, both already scaffolded
    covers: a region the server re-renders, addressed and applied by the framework rather than by a library the application chose
    surface: flow:partial-refresh over requirement:navigation-delta-rendering, requirement:reloadable-component-endpoint, and requirement:action-response-update
    sits_above: server_fragments, which it does not replace; a project already using a swap library keeps working and gains nothing it has to adopt
    what_it_adds_over_a_swap_library: boundary identity, validators so unchanged markup is not sent, supersession and ordering, form-state and focus preservation, and a documented fallback on every failure
    runtime: requirement:unified-update-runtime, one asset for every capability
  unavailable:
    - full-document hydration
    - client-side routing
    - optimistic mutation without a server round trip
    - concept:client-component, which is not delivered today
rules:
  - a tier is justified by an interaction the tier below cannot express, not by familiarity
  - platform features degrade rather than break, so a feature lacking universal support is used as enhancement over a working baseline
  - the framework names no swap library and no CSS plugin, so any tier choice stays an application decision
  - browser support claims carry a verification date, since they age faster than framework behaviour
framework_couplings:
  head_hoisting: scoped styles and head contributions reach the document only on the page path, so styles for swapped regions belong to the loaded page per decision:fragment-head-rejection
  waiting_state: requirement:async-html-rendering fallbacks never reach the browser on the fragment path, so a swapped region's waiting state is the swap library's or CSS's
  shared_markup: one server component called by both the page route and the fragment route keeps the two renderings identical under decision:implicit-document-shell
  script_placement: a site-wide module is admitted once in the document shell, since a swapped region cannot reach the head and re-running a per-fragment tag is nothing's job
  script_braces: script content is literal unless a brace reads as a template insertion, so JavaScript shorthand property syntax collides and doubling the brace, an intrinsic, or a file under the public directory resolves it; a head contribution block stays verbatim throughout
```
