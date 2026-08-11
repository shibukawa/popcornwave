---
id: requirement:page-scope-emission
type: requirement
title: Page Scope Emission
---
Generation emits the scope marker and the module reference a page's own script needs, deciding scope from where the script was written rather than from anything the author has to declare.

```yaml
status: superseded in granularity 2026-08-11 by system:tinybind v0.5.5, which shipped the authoring block and the ownership field on a component axis rather than a page one
superseded_by_upstream:
  what_shipped: a marked script block at the top of a component declaration, extracted as today, and Scope on htmlbind.Asset naming the declaring component
  the_axis_changed: page is a caller word and the module has no definition of one, so the lifetime attaches to the declaring component and a page and a layout become special cases of one rule
  four_assumptions_here_were_wrong:
    position_alone: a top-level script is equally the shape of markup carrying a RawJavaScript or JsonForScript insertion, which ships and is tested, so an explicit marker is what selects the reading and the position rule would have been a breaking change
    withholding_the_head_tag: the tag's position decides when the file loads and the block's position decides its lifetime, and the two are independent; a head tag loading a module once is right, because evaluation happens once regardless
    per_layer_asset_sets: that fixes the wrapper case and not the component case, since a conditional component's asset sits in its own layer's set either way; the fix is that an instance which did not render has no attribute and no manifest entry, so the conservative set is safe as a catalog and never as a mount list
    a_body_contribution_channel: not needed at all, because the instance attribute, the manifest component_id, and the delta's insert and remove operations already carry every join the client needs
  what_this_framework_then_built:
    extraction_writes: the artifact filter of api:cli-generate named neither the stylesheet nor the script kind, so an extracted block produced a URL in the generated component and no file at all; both kinds are kept now and the two public-asset shapes route separately, since a conversion owns its whole name and stages outside the served tree while an extracted block is a base and an extension belonging in the public tree its URL was computed against
    no_configuration_was_needed: the generator defaults write under public/generated, which is where this layout already looks, so the standing gap was this framework's filter rather than an unset option
    page_tree_closed: system:tinybind v0.5.6 added routetree.GenerateTree, returning the extracted assets in a list of their own beside the Go, plus the URL base and attribute prefix the tree never threaded; wired here, and a page declaring a block now writes its file
    two_silent_drops_were_ours: the artifact filter naming neither asset kind, and then grouping a tree's assets under the project root, which is not a page directory and so admits nothing; both produced a page referencing a module that answered 404, with no diagnostic either time
    still_blocked: the per-instance lifecycle, because Asset.Scope is the declared name and the render's ComponentID is package-qualified, so no wire field can join them until one identity space wins; this framework asked for normalizing Scope up and for an attribute rather than a manifest field, since the manifest is bounded at 8192 and silently dropped whole past it
  what_is_still_true_here: the responsibility line, the named setup export with a returned teardown, and the module-once-setup-per-entry distinction, all of which upstream adopted or left to this framework
owner: system:tinybind generation, with the configuration this framework has never set
what_it_produces: '<pw-page hash="…" module="…"></pw-page>' inside the page's own rendered subtree
position_decides_scope:
  rule: a script in a head contribution is site-wide; a script written in a template body belongs to that template
  why_it_reads_well: it is decided where the author already thinks about it, and it needs no attribute, no marker, and nothing to remember
  it_matches_the_template_structure: a shell template carries the head and a page template does not, so every script in a page template is page-scoped without the rule having to say so twice
  precedent: the single-file-component shape, where a script beside markup belongs to that markup
  unchanged_for_the_head: decision:runtime-tag-injection and the head contributions of requirement:component-asset-extraction keep behaving exactly as they do
the_module_semantics_are_not_negotiable:
  temptation: treat the body script's top level as the setup, the way a compiled single-file component does, so the author writes the per-entry code directly
  why_not_here: an ES module's top level is evaluated once, so per-entry code at the top level would need generation to wrap the block in a function — and a static import cannot appear inside one, so the wrap only works for a block with no imports, or needs a real JavaScript parser to hoist them
  what_that_would_be: rewriting authored JavaScript, which requirement:client-signal-registry already refuses for the same reason it is refused here
  chosen: the block is a module and says what runs per entry by exporting it
  shape: |
    <script type="module">
      import { formatTime } from "./util.js";   // once, and imports work
      export function setup(page) {             // every entry
        page.on("app.tick", (e) => { … });
        return () => { … };                     // optional teardown
      }
    </script>
  named_not_default:
    why: it is greppable, it says what it is, and a module that default-exports something of its own is not mistaken for a page
    teardown_is_returned: a second export would have to reach setup's locals through module scope, and module scope is shared across every visit — the one place a page's state must not live
  no_setup_is_legal: a block exporting none simply runs once, the first time the page is entered, which is a real thing to want and not an error
  what_is_kept_from_the_single_file_shape: the position rule, which is the part that carries the ergonomics; the export is what keeps it honest about a module being evaluated once
the_module_path_is_already_decided:
  by: requirement:component-asset-extraction, whose naming is the generation unit, the asset kind, and a content hash, written under the requirement:public-asset-delivery tree
  not_a_new_mechanism: the page hash and the extracted URL are known at the same moment generation processes the template, so neither has to be looked up from the other
  blocked_on: api:cli-generate setting PublicDir and PublicURLBase, which it never has; extraction is shipped upstream and unasked-for here, and that gap is older than this requirement
head_tag_is_not_emitted_for_a_page_script:
  rule: the extracted file is produced as usual, and its reference goes on the element rather than into the head
  why_it_matters_most: it removes the need for a page-hash intrinsic entirely, because the module never names its own hash — the element does
  head_does_not_accumulate: a navigation delta never removes a head tag, so head-referencing every page a session visits grows the head for the life of the tab
  load_is_deferred: the import happens at first entry and resolves from the module map on every entry after, so a page never visited costs nothing
  extraction_disabled: leaves the block inline in the body, where the browser runs it once with no scope at all; a project with page-scoped scripts and extraction off is a startup failure rather than a silent difference
one_script_per_page:
  rule: at most one page-scoped block per template, reported as a diagnostic rather than merged
  why: the element names one module, and bundling several blocks into one would reorder their evaluation against each other
  not_a_limit_in_practice: it is the single-file-component shape, where one block per unit is what an author writes anyway
the_hash:
  value: a digest of the declaring directory, the same material the action endpoint path already uses at sha256 of the directory and the handler name
  stable: an unchanged project regenerates the same hash, which is what lets a cached module keep matching the element that names it
  not_secret: it identifies a template and grants nothing; the endpoint it resembles is public input too
placement_inside_the_subtree:
  required: the element must sit where a navigation delta replaces it, or its disconnect never fires and the scope never closes
  constraint: requirement:navigation-delta-rendering gives a boundary exactly one root element, so the marker is a child of that root rather than a sibling
  cost_of_an_element_there: an empty element is a flex item and shifts :first-child, which is why api:html-boundary-protocol chose comment nodes for its own ranges
  alternative_worth_weighing: the hash as an attribute on the boundary root element generation already writes, with the runtime recomputing the active set at the swaps it performs; no DOM footprint at all, at the price of the lifecycle no longer being the platform's
acceptance:
  - a page template with one body script produces one extracted module and one element naming it
  - the module's exported setup runs on every entry and its top level exactly once
  - a page never visited never fetches its module
  - a script in a head contribution keeps reaching the head, unscoped, exactly as today
  - a project with no page-scoped script regenerates byte-identical output
open_questions:
  - whether a non-page component with a body script gets a scope of its own, which would generalize the element past pages and make its name wrong
  - whether the element or the boundary-root attribute is the emitted form, given the layout cost of an element inside a flex container
  - whether a later script-setup form is worth a JavaScript parser in the toolchain, which is the only way the top-level shape can hoist imports correctly
```
