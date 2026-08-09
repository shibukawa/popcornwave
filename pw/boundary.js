// Popcorn Wave boundary runtime.

const modeHeader = "Pw-Response-Mode";
const liveMode = "live";

// The validator of the content each boundary is showing travels back to the
// server on this header, and the server leaves alone every region whose
// validator still matches what it just rendered.
//
// It is what makes a reconnect cost what changed rather than the whole page. A
// live connection re-executes the page and re-renders every boundary on it,
// including the ones that settled once and never change, so without this a
// dropped connection or a lifetime rollover re-transfers the screen to say
// nothing happened.
//
// The values are opaque and are never compared here. This side stores what
// arrived and hands it back; only the server can say whether it still holds.
const liveManifestHeader = "Pw-Live-Manifest";

// The CSRF token travels in a cookie rather than in the page, and it is read
// here at the moment a request is issued rather than once at load. That is the
// whole difference: a value written into the document is fixed at render, so a
// session rotation — a login, a privilege change — would never reach a screen
// that is already open, and its next request would be refused with nothing on
// screen explaining why.
//
// The names are constants because this script cannot read them off its own tag:
// a module script has no document.currentScript. They are pw.CSRFCookieName and
// the header the middleware reads, and the Go side names the same two.
const csrfCookieName = "pw_csrf";
const csrfHeaderName = "X-CSRF-Token";

function csrfToken() {
	// A cookie value cannot contain a semicolon or a space unquoted, and this
	// one is base64url, so splitting on "; " is exact rather than a heuristic.
	const prefix = csrfCookieName + "=";
	for (const entry of document.cookie.split("; ")) {
		if (entry.startsWith(prefix)) return entry.slice(prefix.length);
	}
	return "";
}

// withCSRF adds the token to a header set. Every request this runtime issues
// goes through it, so a request that should carry one cannot be written without
// it by forgetting a call site.
function withCSRF(headers) {
	const token = csrfToken();
	if (token) headers[csrfHeaderName] = token;
	return headers;
}

// Every piece of module state is declared here, above the custom element
// definitions, and that placement is load-bearing. Defining an element upgrades
// the ones the parser already inserted, synchronously, inside the define call —
// so a callback that reads a binding declared further down the module reads it
// before its initializer has run, and throws. The document marker is always
// already in the DOM by then, which makes this the ordinary case rather than a
// race.

// replaced records that the document has been given up on. A replacement is
// terminal: the page it patched no longer exists, so a completion arriving
// after it belongs to a document that is gone.
let replaced = false;

// documentComplete records that the terminal marker arrived. Nothing about the
// transport can say this: a chunked document cut off mid-stream is end of file
// to the parser, which fires DOMContentLoaded and load exactly as it does for a
// complete one.
let documentComplete = false;
let documentVersion = "";

// The live connection: one request to this page's own URL, carrying the mode
// header, re-issued whenever it ends for a reason that is not terminal. The
// server holds no subscription state, so a reconnect is the same request as the
// first connection.
let connection = null;
let running = false;

// applied remembers where each boundary's content sits, as the pair of comment
// nodes that bracket it, plus the HTML currently between them.
//
// A settled boundary could simply replace its placeholder, because it settled
// once. A live boundary is re-rendered for as long as its subscription lives,
// so its content needs an address that survives the first delivery. Comments
// are that address: they are inert, invisible to CSS and to layout, and they
// bracket a range rather than wrap it, so a delivery of several top-level nodes
// needs no container element the author never wrote.
//
// Every boundary is bracketed, not only the live ones, because nothing on the
// wire says which is which: htmlbind allocates the same placeholder markup and
// yields the same id-and-HTML pair for both.
const applied = new Map();

function pruneApplied() {
	for (const [id, range] of applied) {
		if (!range.end.isConnected) applied.delete(id);
	}
}

// The comment pair bracketing one boundary's content. Since system:tinybind
// v0.4.8 a progressive render writes the same pair around an await boundary's
// fallback, so the spelling is a contract with the server rather than a private
// choice, and a range this client creates and one the document arrived with are
// the same thing.
function openMarker(id) {
	return "tb:" + id;
}

function closeMarker(id) {
	return "/tb:" + id;
}

// findFence returns the pair the document was written with, for a boundary this
// client has not settled yet.
//
// A comment carries no id and no selector reaches it, so this walks. The walk is
// over comment nodes only, and it runs once per boundary — the range is kept in
// applied afterwards, and every later delivery to the same boundary refills it
// without searching again.
//
// The open marker is remembered rather than matched pairwise, because boundaries
// nest: an outer fence opens, an inner one opens and closes inside it, and the
// outer one closes last. Taking the nearest open before the matching close is
// what pairs them correctly, and the ids differ, so only markers naming this
// boundary are ever considered.
function findFence(id) {
	const open = openMarker(id);
	const close = closeMarker(id);
	const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_COMMENT);
	let start = null;
	while (walker.nextNode()) {
		const comment = walker.currentNode;
		if (comment.data === open) start = comment;
		else if (comment.data === close && start) return { start: start, end: comment };
	}
	return null;
}

// liveManifest is what this screen claims to be showing, as the pairs the next
// connection carries.
//
// A range whose end marker has left the document is skipped: an enclosing
// boundary re-rendered and took it along, so the content it names is not on
// screen and claiming it would leave the region empty until something else
// changed. A range with no validator is skipped for the same reason in reverse
// — nothing here can compute one, so an unclaimed boundary is simply delivered.
function liveManifest() {
	const pairs = [];
	for (const [id, range] of applied) {
		if (range.digest && range.end.isConnected) pairs.push(id + ":" + range.digest);
	}
	return pairs.join(",");
}

// seedManifest takes the validators of what the document itself committed, off
// the terminal marker.
//
// Without it the first connection of every page view re-transfers the whole
// screen, because the document delivered its boundaries through the parser and
// the connection that follows has no idea which bytes are already there.
function seedManifest(value) {
	if (!value) return;
	for (const entry of value.split(",")) {
		const cut = entry.indexOf(":");
		if (cut <= 0) continue;
		const range = applied.get(entry.slice(0, cut));
		if (range) range.digest = entry.slice(cut + 1);
	}
}

function refill(range, fragment, html) {
	let node = range.start.nextSibling;
	const outgoing = [];
	while (node && node !== range.end) {
		const next = node.nextSibling;
		outgoing.push(node);
		node = next;
	}
	// A live boundary replaces content a user may have been typing into, so the
	// same carrying-across an update does applies here.
	const settle = carryClientState(outgoing, fragment);
	for (const gone of outgoing) gone.remove();
	range.end.parentNode.insertBefore(fragment, range.end);
	settle();
	range.html = html;
	pruneApplied();
}

// The client state a server render cannot know about, carried from the outgoing
// nodes into the replacement before it lands.
//
// It lives here rather than beside the update runtime because both halves of
// this asset replace content: a delta swaps a region and a live delivery refills
// one, and a user who lost their typing would not care which did it. One
// implementation is also the only way the two cannot drift.

// composing is true while an input method has an unconfirmed composition open.
// Replacing the element under it would commit or discard whatever the user was
// midway through spelling.
let composing = false;
document.addEventListener("compositionstart", () => { composing = true; });
document.addEventListener("compositionend", () => { composing = false; });

export function compositionActive() {
	return composing;
}

// resolveNavigable returns the absolute form of a navigation target the browser
// can follow without running script, or null for one it cannot.
//
// location.assign executes a javascript: URL rather than navigating to it, so
// every sink that reaches it needs this. The server refuses such a target too;
// this is the half that holds when a record arrives from somewhere the server
// did not write it, and it costs one comparison.
//
// Resolving first is what keeps the rule short. A relative target resolves
// against the page and therefore inherits the page's own scheme, so it needs no
// case of its own — after resolution there is only ever an absolute URL to
// check, and the allowlist mirrors internal/safeurl on the Go side.
export function resolveNavigable(value, base) {
	if (typeof value !== "string" || value === "") return null;
	let resolved;
	try {
		resolved = new URL(value, base || document.baseURI);
	} catch {
		return null;
	}
	switch (resolved.protocol) {
		case "http:":
		case "https:":
		case "mailto:":
		case "tel:":
			return resolved.href;
		default:
			return null;
	}
}

// A marked region is moved rather than re-rendered: a third-party widget, a
// canvas, a media element mid-playback. The server does not own what is inside
// it and cannot reproduce it.
export function preserveAttributeName(attr) {
	return "data-" + attr + "-preserve";
}

let preserveAttr = preserveAttributeName("tb");

export function setPreserveAttribute(name) {
	preserveAttr = name;
}

// carryClientState moves preserved islands into the replacement and restores
// the form state the render did not mean to change.
//
// It returns what has to happen after the replacement is in the document.
// Focus is the reason: focusing a node that is still inside a fragment does
// nothing at all, silently, so the one thing a user notices most is the one
// thing that cannot be finished here.
function carryClientState(outgoing, fragment) {
	const live = new Map();
	const values = new Map();
	for (const node of outgoing) {
		if (!node.querySelectorAll) continue;
		collectPreserved(node, live);
		collectFormState(node, values);
	}
	if (live.size) restorePreserved(fragment, live);
	if (!values.size) return settleNothing;
	const focused = restoreFormState(fragment, values);
	return focused ? () => placeFocus(focused) : settleNothing;
}

function settleNothing() {}

// Only a text-like control has a selection. Reading one from a control that
// does not throws rather than answering, so the probe is guarded rather than
// the type being enumerated: which input types carry a selection has changed
// before and is not worth a list here.
function selectionOf(control) {
	try {
		if (control.selectionStart === null || control.selectionStart === undefined) return null;
		return [control.selectionStart, control.selectionEnd, control.selectionDirection || "none"];
	} catch {
		return null;
	}
}

function placeFocus(target) {
	const control = target.control;
	if (!control.isConnected || !control.focus) return;
	// preventScroll because where the viewport belongs is a decision the update
	// runtime has already made, and refocusing must not overrule it.
	control.focus({ preventScroll: true });
	if (!target.selection || !control.setSelectionRange) return;
	try {
		control.setSelectionRange(target.selection[0], target.selection[1], target.selection[2]);
	} catch {
		// A control whose type changed under the swap has no selection to set.
	}
}

function collectPreserved(root, into) {
	if (root.getAttribute && root.hasAttribute(preserveAttr)) {
		into.set(root.getAttribute(preserveAttr), root);
	}
	for (const marked of root.querySelectorAll("[" + preserveAttr + "]")) {
		into.set(marked.getAttribute(preserveAttr), marked);
	}
}

function restorePreserved(fragment, live) {
	for (const hole of fragment.querySelectorAll("[" + preserveAttr + "]")) {
		const kept = live.get(hole.getAttribute(preserveAttr));
		if (kept) hole.replaceWith(kept);
	}
}

// A control's value is compared against its own default rather than against the
// replacement's. That comparison is the whole rule: a value equal to its default
// is one the user never touched, so the new default wins; a value that differs
// is the user's typing, and an update that did not assert a new default must not
// discard it.
function collectFormState(root, into) {
	const controls = root.querySelectorAll ? root.querySelectorAll("input, textarea, select") : [];
	for (const control of controls) {
		const key = control.name || control.id;
		if (!key) continue;
		const held = {};
		// Focus and the caret are carried whether or not the value moved. A user
		// whose cursor is still in the search box has changed nothing about it,
		// and that is exactly the case a region swap loses: a page that updates
		// as it is typed puts the caret at the end of nowhere on every keystroke.
		if (control === document.activeElement) {
			held.focused = true;
			held.selection = selectionOf(control);
		}
		if (control.type === "checkbox" || control.type === "radio") {
			if (control.checked !== control.defaultChecked) held.checked = control.checked;
		} else if (control.type === "file") {
			// A file input's value cannot be set from script at all, so it is not
			// restorable by value; it belongs in a preserved island or outside the
			// region, which requirement:unified-update-runtime records as a gap.
		} else if (control.value !== control.defaultValue) {
			held.value = control.value;
		}
		if (held.focused || "checked" in held || "value" in held) into.set(key, held);
	}
}

// Returns the control focus belongs on once the replacement has landed, or null.
// There is at most one: a document has one active element.
function restoreFormState(fragment, values) {
	let focused = null;
	const controls = fragment.querySelectorAll ? fragment.querySelectorAll("input, textarea, select") : [];
	for (const control of controls) {
		const held = values.get(control.name || control.id);
		if (!held) continue;
		if ("checked" in held) {
			// A changed default is the server asserting a new value, and it wins.
			if (control.checked === control.defaultChecked) control.checked = held.checked;
		} else if ("value" in held && control.value === control.defaultValue) {
			control.value = held.value;
		}
		if (held.focused && !focused) focused = { control: control, selection: held.selection };
	}
	return focused;
}

// parseFragment turns rendered markup into nodes. It is separate from the swap
// below because a caller filling the holes of a decomposed fragment has to reach
// inside it before it lands: a node moved into a hole after insertion is a node
// moved twice, and a reparented iframe reloads on every move.
export function parseFragment(html) {
	const holder = document.createElement("template");
	holder.innerHTML = html;
	return holder.content;
}

// swapNode replaces one addressed element with prepared nodes, carrying the
// client state across. It is what a delta operation, a redraw, and an action
// response all land through, so a region arrives the same way whichever asked.
//
// The carry is in two halves: what has to move before the nodes land — a
// preserved island, a control's value — and what can only be restored once they
// are in the document, which is focus and the caret.
export function swapNode(target, fragment) {
	if (!target || !target.parentNode) return false;
	const settle = carryClientState([target], fragment);
	target.replaceWith(fragment);
	settle();
	pruneApplied();
	return true;
}

// swapElement is swapNode for a caller holding markup rather than nodes.
export function swapElement(target, html) {
	return swapNode(target, parseFragment(html));
}

export function applyBoundary(id, fragment, html, digest) {
	if (replaced) return false;
	const range = applied.get(id);
	if (range && !range.end.isConnected) {
		// An enclosing live boundary re-rendered and took this range with it.
		// The replacement subtree carries this boundary's placeholder again,
		// under the same id, so the lookup below finds it.
		applied.delete(id);
	} else if (range) {
		// The server suppresses an unchanged delivery, so this is the case it
		// cannot: a reconnect to a process whose validator key differs, which
		// renders the same bytes and cannot tell they are already here.
		// Re-inserting identical nodes would restart animations, drop focus and
		// selection inside the region, and make a screen reader announce a log
		// nobody added to, so an identical delivery is left alone — and the new
		// validator is taken, or this screen would keep claiming one the server
		// no longer recognizes and keep being sent the same bytes.
		if (html !== undefined && html === range.html) {
			range.digest = digest;
			return true;
		}
		refill(range, fragment, html);
		range.digest = digest;
		return true;
	}
	// Not settled yet, so the range is the one the document arrived with: the
	// fence around this boundary's fallback.
	//
	// The markers stay. A settled boundary would not need them again, but a live
	// one is re-rendered for as long as its subscription lives, and nothing on
	// the wire says which this is — the same fence and the same id-and-HTML pair
	// serve both. Keeping them costs two comment nodes and is what lets the
	// second delivery find the region the first one wrote.
	const fence = findFence(id);
	if (!fence) return false;
	const opened = { start: fence.start, end: fence.end, html: html, digest: digest };
	applied.set(id, opened);
	refill(opened, fragment, html);
	return true;
}

export function applyHTML(id, html, digest) {
	const holder = document.createElement("template");
	holder.innerHTML = html;
	return applyBoundary(id, holder.content, html, digest);
}

export function replaceDocument(fragment) {
	replaced = true;
	stopLive();
	document.body.replaceChildren(fragment);
	applied.clear();
}

customElements.define("tb-apply", class extends HTMLElement {
	connectedCallback() {
		const id = this.getAttribute("for");
		this.remove();
		if (!id) return;
		const quoted = id.replace(/["\\]/g, "\\$&");
		const template = document.querySelector('template[data-tb-boundary="' + quoted + '"]');
		if (!template) return;
		applyBoundary(id, template.content, template.innerHTML);
		template.remove();
	}
});

customElements.define("tb-apply-document", class extends HTMLElement {
	connectedCallback() {
		this.remove();
		const template = document.querySelector("template[data-tb-document]");
		if (!template) return;
		// replaceChildren drops the template along with the page it replaces,
		// and every pending placeholder with it.
		replaceDocument(template.content);
	}
});

customElements.define("tb-stream-end", class extends HTMLElement {
	connectedCallback() {
		documentComplete = true;
		documentVersion = this.getAttribute("version") || "";
		const state = this.getAttribute("state");
		// The marker is the last markup of the document, so every tb-apply has
		// already run and every range this names is in place.
		seedManifest(this.getAttribute("manifest"));
		this.remove();
		// This document arrived whole, so a reload attempted for a truncated
		// one is over and the next truncation is allowed to reload again.
		clearReloadGuard();
		// final means nothing more is coming and live means the screen keeps
		// changing. Asking either way would cost a whole page execution on the
		// server for a page that has nothing to deliver.
		if (state === "live") startLive();
	}
});

// checkDocumentEnd decides whether this document ended or was cut off.
//
// readyState says when the question can be answered and the marker says what
// the answer is: while it is loading, bytes are still arriving and the marker
// may be among them; once it is complete, no more bytes are coming and the
// marker either arrived or never will.
//
// A document that streamed nothing carries no marker either, so it is excluded
// rather than reloaded. Its boundaries — the placeholders still on screen, or
// the ranges already applied — are what say this response was one that streams.
function documentStreamed() {
	return applied.size > 0 || anyFence();
}

// anyFence reports whether this document was written with boundary markers at
// all, settled or not.
//
// The markers outlive settling now — a live boundary needs its range for every
// delivery after the first — so their presence says the response was one that
// streams rather than that something is still pending. That is the question
// being asked: a document that streamed nothing carries no terminal marker
// either, and reloading it would be reloading a page that arrived whole.
function anyFence() {
	const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_COMMENT);
	while (walker.nextNode()) {
		if (walker.currentNode.data.startsWith("tb:")) return true;
	}
	return false;
}

function checkDocumentEnd() {
	if (document.readyState !== "complete") return;
	if (documentComplete || !documentStreamed()) return;
	reloadOnce("the document was truncated");
}

document.addEventListener("readystatechange", checkDocumentEnd);
// The module may have been imported after the document settled, in which case
// no further readystatechange is coming.
checkDocumentEnd();

// reloadOnce recovers a screen that cannot be repaired in place. The guard is
// what keeps a server that truncates every response from turning that into a
// reload loop the user cannot escape.
function reloadGuardKey() {
	return "pw:reloaded:" + location.pathname + location.search;
}

function reloadOnce(reason) {
	try {
		if (sessionStorage.getItem(reloadGuardKey())) {
			console.error("Popcorn Wave: " + reason + "; reload already attempted");
			return;
		}
		sessionStorage.setItem(reloadGuardKey(), "1");
	} catch (error) {
		// Storage can be unavailable in a private or partitioned context. A
		// missing guard is worth less than a stuck screen, so recover anyway.
	}
	console.warn("Popcorn Wave: " + reason + "; reloading");
	stopLive();
	location.reload();
}

function clearReloadGuard() {
	try {
		sessionStorage.removeItem(reloadGuardKey());
	} catch (error) {
		// Nothing to clear if storage is unavailable.
	}
}

// readRecords yields the records of a newline-delimited stream.
//
// One reader, because both this framework's stream shapes are the same framing
// and reading them twice is how the two come apart. A line that will not parse
// ends the sequence: a stream whose framing is broken has no next record worth
// guessing at, and a caller distinguishes that from a clean end by whether it
// saw a terminator.
export async function* readRecords(body) {
	const reader = body.getReader();
	const decoder = new TextDecoder();
	let buffer = "";
	for (;;) {
		const chunk = await reader.read();
		if (chunk.done) return;
		buffer += decoder.decode(chunk.value, { stream: true });
		let newline = buffer.indexOf("\n");
		while (newline >= 0) {
			const line = buffer.slice(0, newline);
			buffer = buffer.slice(newline + 1);
			newline = buffer.indexOf("\n");
			if (!line.trim()) continue;
			try {
				yield JSON.parse(line);
			} catch (error) {
				return;
			}
		}
	}
}

export function stopLive() {
	running = false;
	if (connection) {
		connection.abort();
		connection = null;
	}
}

export function startLive() {
	if (running) return;
	running = true;
	runLive();
}

async function runLive() {
	let attempts = 0;
	while (running) {
		const outcome = await connectOnce();
		if (!running || outcome.stop) return;
		// A response the server closed at its lifetime bound is healthy, so
		// backing off exponentially would stall a working screen. Only a failure
		// or a truncated stream escalates.
		if (outcome.retry) {
			attempts = 0;
			await sleep(jitter(outcome.retryAfter || 500));
			continue;
		}
		attempts += 1;
		await sleep(jitter(Math.min(30000, 500 * Math.pow(2, attempts))));
	}
}

async function connectOnce() {
	const controller = new AbortController();
	connection = controller;
	let opened = false;
	try {
		const headers = withCSRF({ [modeHeader]: liveMode });
		// Built at connect time rather than kept as a running value, because
		// what is on screen changes between connections: a navigation delta
		// rewrote a region, an enclosing boundary took a nested one with it.
		const manifest = liveManifest();
		if (manifest) headers[liveManifestHeader] = manifest;
		const response = await fetch(location.href, {
			headers: headers,
			credentials: "same-origin",
			cache: "no-store",
			redirect: "error",
			signal: controller.signal,
		});
		if (!response.ok || !response.body) return { stop: false, retry: false };
		let closed = null;
		for await (const record of readRecords(response.body)) {
			const outcome = handleRecord(record);
			if (outcome === "stop") {
				controller.abort();
				return { stop: true, retry: false };
			}
			if (record.r === "end") closed = record;
		}
		if (closed && closed.reason === "done") return { stop: true, retry: false };
		if (closed) return { stop: false, retry: true, retryAfter: closed.retryMs };
		// The stream ended with no terminal record, so it was cut off rather
		// than finished. Reconnecting is safe: the page executes again and
		// delivers the current state of every live region.
		return { stop: false, retry: false };
	} catch (error) {
		if (controller.signal.aborted) return { stop: true, retry: false };
		return { stop: false, retry: false };
	} finally {
		if (connection === controller) connection = null;
	}
}

function handleRecord(record) {
	if (record.r === "head") {
		// Boundary ids name positions in generated code. A deployment that
		// changed that code addresses a document this screen is not showing, so
		// applying anything from it would put content in the wrong place. An
		// unstamped build sends nothing here, which disables the check rather
		// than reloading every screen whenever the server restarts.
		if (documentVersion && record.build && record.build !== documentVersion) {
			reloadOnce("the server was deployed while this page was open");
			return "stop";
		}
		// A delivery whose content reaches a component the document never
		// carried needs its tags before its markup, which is the same ordering
		// the navigation delta makes normative.
		installLiveHead(record.head);
		return "opened";
	}
	if (record.r === "await") {
		if (typeof record.id !== "string" || typeof record.html !== "string") return "ignored";
		if (!applyHTML(record.id, record.html, typeof record.v === "string" ? record.v : undefined)) {
			// The same page executed again produces the same ids, so an id this
			// screen does not hold means the page's structure changed — a panel
			// added to a dashboard somebody has been watching. Placing it
			// correctly means rendering a document this client did not render,
			// so the honest move is to start over rather than guess.
			reloadOnce("the page structure changed");
			return "stop";
		}
		return "applied";
	}
	if (record.r === "end") return "closed";
	if (record.r === "reload") {
		reloadOnce("the server asked for a reload");
		return "stop";
	}
	if (record.r === "navigate") {
		const target = resolveNavigable(record.url);
		if (target) {
			stopLive();
			location.assign(target);
		}
		return "stop";
	}
	// An unknown record comes from a newer server. Ignoring it keeps an older
	// client working instead of tearing down a connection it could still use.
	return "ignored";
}

// installLiveHead adds the tags a delivery needs and nothing else.
//
// It is this half's own rather than the update runtime's, because the update
// runtime is built from a configuration this half never sees. The rule it holds
// is the one that matters: a tag already in the head is left alone, so a
// reconnect that repeats the whole head installs nothing.
function installLiveHead(tags) {
	if (!tags || !tags.length) return;
	for (const tag of tags) {
		const holder = document.createElement("template");
		holder.innerHTML = tag;
		for (const node of Array.from(holder.content.children)) {
			if (node.tagName === "TITLE") continue;
			let present = false;
			for (const existing of document.head.children) {
				if (existing.outerHTML === node.outerHTML) present = true;
			}
			if (!present) document.head.appendChild(node);
		}
	}
}

function jitter(delay) {
	return Math.round(delay * (0.75 + Math.random() * 0.5));
}

function sleep(delay) {
	return new Promise((resolve) => setTimeout(resolve, delay));
}

// A tab going away should release the response rather than leave the server
// rendering for a screen nobody is looking at.
window.addEventListener("pagehide", stopLive);
