---
id: requirement:dev-console-launcher
type: requirement
title: Development Console Launcher
---
A page served by api:cli-dev carries one floating control that opens requirement:dev-console, so the console is reached from the application the developer is already looking at rather than from a URL printed at startup and since scrolled away.

```yaml
audience: actor:application-developer
scope: api:cli-dev only, through the reserved pwdev build mode of policy:dev-console-boundary
default: enabled
configuration: data:project-config dev.console.launcher, whose enabled key switches it and whose corner key places it
admission: decision:dev-launcher-admission, which is what separates this control from the debug toolbar requirement:dev-error-overlay refuses
state: data:dev-loop-state, read from the stream flow:dev-overlay-delivery already opens
runtime_class: decision:dev-browser-runtime-scope development class
mark: data:dev-launcher-mark
delivery:
  module: the same pwdev module requirement:dev-error-overlay is delivered by, because both need the console address and one stream, and a second module would open a second one
  switch: dev.console.launcher.enabled false removes the control and leaves the overlay; dev.console.overlay.enabled false removes the overlay and leaves the control; both false inject no console address and serve no development module at all
  corner_delivery: the resolved corner reaches the module the way the console address does, baked into the bytes it is served as, so the page is told nothing and the module reads nothing out of the document
  isolation: its own shadow root, separate from the overlay's, so the application's stylesheet cannot restyle it and it cannot restyle the application
  address: the console URL is baked into the module bytes, as it already is for the overlay, so nothing is injected into a document the framework does not own
placement:
  configured: dev.console.launcher.corner, one of the four corners, fixed, offset one rem from both edges
  default: bottom left
  why_that_default: the bottom right is where applications put their own floating things — a chat launcher, a scroll-to-top, a cookie control — so the framework taking it would collide with the developer's own work on the page it is meant to help with
  why_a_key_at_all: a corner that is free on one project is occupied on the next, and no default can be right for every layout. The escape hatch had been dismissal, which is per page and temporary; a project whose bottom left is permanently covered needs to say so once
  collisions_by_corner:
    bottom-left: the browser's own link-target bubble, while a link is hovered. The launcher sits above it and the bubble wins the very bottom edge, which is why this corner is still the default: the collision is transient
    bottom-right: application floating controls, which is the common case and the reason this is not the default
    top-left: a sticky header or a brand, which is not transient
    top-right: a sticky header's own controls, an account menu most often
  why_four_and_not_free:
    corner: a value that travels with the project, visible to everyone working on it and reviewed like any other configuration
    free_position: drag state, held per browser, invisible to the rest of the team, and a mis-click hazard on a control whose whole behavior is a link
  mirroring: the hover label and the dismiss control open toward the middle of the viewport rather than toward the nearer edge, so neither runs off screen in any corner
  invariant: the corner changes where the control sits and nothing else; it is still fixed, still outside the document flow, and still adds no wrapper to the application's markup
  scaffolded: api:cli-init writes the corner into data:project-config beside dev.console.port, naming the other three in a comment. The key works absent and the scaffold writes it anyway, because a default corner is met by finding a button over one's own layout, and an empty section says nothing about how to move it
  why_not_every_key: the switch is not scaffolded beside it; turning the launcher off is a thing a project does once and can read in the documentation, where the corner is a thing it adjusts while looking at the page
  stacking: below the overlay, and removed entirely while a failed record is showing, because the overlay covers the viewport and carries its own console link; a shape ghosting through a diagnostic is noise
form:
  element: an anchor with an href, not a click handler on a div
  why: middle click, modifier click, and the browser's own open-in-new-tab all work without the framework implementing any of them, and the control is keyboard reachable because it is a link
  target: the console index, which names every pane and is therefore the way in to all of them
  window: a named tab, so returning to the console reuses the tab already open rather than stacking one per click
  no_rel: rel=noopener is deliberately absent, because a browser ignores a target name when it is set and opens a fresh tab every time. Nothing is lost by leaving it off: the console is a different loopback port, so the opener it receives is cross-origin and can reach nothing
  label: an accessible name naming the console; shown as text beside the mark on hover and on focus, so the control is discoverable without a click and reachable without a mouse
  size: a 44px hit target with the mark inset, which is the smallest target that is comfortable and the smallest at which the mark still reads as a face
  footprint: the fixed box is the button and nothing more. The label and the dismiss control are positioned out of flow, because a flex row holding them keeps their width while they are invisible, and the box then swallows clicks along a strip of the application nobody can see
  continuity: the gap between the button and what it reveals is padding inside the revealed group rather than a margin outside it, so the hover target runs unbroken from one to the other; a dead zone between them collapses the reveal as the pointer crosses it
status:
  healthy: the mark, and nothing else
  starting: a quiet ring while the loop is between two working applications, because the page on screen is already stale and nothing else on it says so
  failed: no launcher; requirement:dev-error-overlay owns the viewport
  motion: the ring is the only animation, and prefers-reduced-motion replaces it with a static tint
dismissal:
  control: revealed on hover and on focus, never occupying the resting state
  scope: the browsing session, held in session storage under a framework-reserved key, so a developer who dismissed it to click the button underneath does not dismiss it again on every reload
  why_not_persistent: a permanent hide is dev.console.launcher.enabled false and a permanent move is dev.console.launcher.corner, both versioned with the project and visible to everyone working on it, where a local storage key is invisible and survives a developer forgetting they set it
  still_useful: the corner key answers a collision the project has every run; dismissal answers the one a developer has for the next few minutes, under the control they happen to be testing
  return: closing the tab restores it
rules:
  - the launcher answers no question about the application; it reads the one record it already receives and links to the console for everything else
  - it adds no route to the application, per policy:dev-console-boundary
  - it renders nothing until the module loads, so a page whose document shell dropped the requirement:external-boundary-runtime reference simply has no launcher
  - a failure to reach the console leaves the link in place; a launcher that removed itself on a transport error would be missing exactly when the console is worth opening
  - the control is inert on the application's own layout: fixed positioning in a shadow root, no wrapper element, and nothing added to the document flow
  - it intercepts pointer events over its own 44px and nowhere else, which the footprint rule above is what makes true
non_goals:
  - a panel listing requests, queries, routes, or configuration on the application page; those are requirement:dev-telemetry-viewer and the requirement:dev-console panes, and decision:dev-launcher-admission is where the line is drawn
  - opening a console pane inside the application page, in a frame or otherwise
  - dragging the control, or any position finer than the four corners; the key already answers the collision a drag would, and it answers it for everyone on the project rather than for one browser
  - moving out of the way on its own by watching what is under it, which is a layout engine solving a problem one key already solves
  - any presence in an api:cli-build artifact
acceptance:
  - pw dev with no launcher configuration serves a page whose bottom left carries the control, and clicking it opens the console index
  - each of the four corner values places the control in that corner, with the label and the dismiss control opening inward
  - an unrecognised corner value fails configuration loading and names the four it accepts, rather than falling back to the default
  - a second click focuses the tab already open rather than opening another
  - a click one hundred pixels from the button reaches the application, in every corner and whether or not the label has been revealed
  - the control is reachable by keyboard and names the console to a screen reader
  - a failed record hides the control and the overlay's own console link is the way in
  - dismissing it hides it for the session and a new tab has it back
  - dev.console.launcher.enabled false serves no launcher and leaves the overlay working
  - dev.console.launcher.enabled false with dev.console.overlay.enabled false leaves every served page byte-identical to a production render
  - a binary produced by api:cli-build serves no launcher, no mark, and no reference to either
open:
  - the starting ring shipped; whether it earns its code depends on how long a rebuild actually holds a page stale in practice, which is a question for use rather than for review
```
