---
id: api:render-html-chain
type: api
title: TinyBind HTML Render Chain
---
TinyBind composes generated document and layout wrappers around one generated page fragment.

```yaml
owner: system:tinybind htmlbind package
signature: "htmlbind.RenderChain(io.Writer, []htmlbind.Wrapper, htmlbind.Fragment, ...htmlbind.Option) error"
async_signature: "htmlbind.RenderChainAsync(context.Context, io.Writer, []htmlbind.Wrapper, htmlbind.Fragment, ...htmlbind.Option) iter.Seq2[htmlbind.Content, error]"
probe: "htmlbind.HasAwaitBlock([]htmlbind.Wrapper, htmlbind.Fragment) bool unions the chain members"
order:
  wrappers: outermost first
  leaf: innermost page fragment
generated_values:
  fragment: "<Name>(<Name>Params) htmlbind.Fragment"
  wrapper: "Bind<Name>(<Name>Params) htmlbind.Wrapper for a component with an unnamed slot"
behavior:
  - validate the leaf and wrappers before writing
  - merge head contributions in composition order
  - fill each wrapper unnamed slot with the next wrapper or leaf
  - render an empty wrapper list like htmlbind.Render
  - RenderChain blocks on every await binding and writes a complete settled document
async_behavior:
  - rendering starts on the first pull of the sequence
  - assembly and parameter validation errors yield before any byte is written
  - the initial pass writes the shell, merged head, and every fallback, then flushes
  - the merged head carries component contributions only; nothing is injected
  - each later item is one settled boundary, in completion order
  - Content.WriteTo writes the bare fragment, so the caller writes its own framing around it per api:html-boundary-protocol
  - only the ranging caller writes the response and calls htmlbind.Flush
  - the sequence is single-use and single-consumer; stopping early ends the render
  - after the initial pass the status is committed and a later error is only loggable
chain_composition:
  coordinator: one per render call, shared by every chain member
  identifiers: one counter across the whole chain, so a document, layout, and page boundary cannot collide
  siblings:
    fact: a slot may not appear inside an await block, so an inner member always renders in the initial pass rather than inside a boundary goroutine
    effect: boundaries from different chain members start together and settle concurrently
    cost: wall clock is the slowest single boundary, not the sum across members
  nesting:
    fact: an await inside another await primary subtree registers only when that subtree renders
    effect: nested boundaries serialize, and their timeouts add up
    delivery: the outer replacement carries the inner placeholder, and the inner completion applies to it normally
  head: MergeHead drops exact duplicates outermost first, so two members declaring one stylesheet emit one tag
options:
  - WithCache
  - WithContext
  - WithErrorReporter
  - WithAsyncTimeout
  - WithConcurrencyLimit
compatibility: v0.1.15 generated Fragment and Wrapper APIs replace earlier direct-writer template APIs and require regenerated call sites; v0.1.19 adds the async entry points and htmlbind.Pending parameters; v0.1.20 narrows Content.WriteTo to the bare fragment and stops injecting a client runtime
visibility: internal implementation behind api:html-response and decision:implicit-document-shell
```
