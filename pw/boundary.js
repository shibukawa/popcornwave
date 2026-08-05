// Popcorn Wave boundary runtime.

const modeHeader = "Pw-Response-Mode";
const liveMode = "live";

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

function bracket(id, fragment, html) {
	const start = document.createComment("tb:" + id);
	const end = document.createComment("/tb:" + id);
	const holder = document.createDocumentFragment();
	holder.appendChild(start);
	holder.appendChild(fragment);
	holder.appendChild(end);
	applied.set(id, { start: start, end: end, html: html });
	return holder;
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
	carryClientState(outgoing, fragment);
	for (const gone of outgoing) gone.remove();
	range.end.parentNode.insertBefore(fragment, range.end);
	range.html = html;
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
// form values the render did not mean to change.
function carryClientState(outgoing, fragment) {
	const live = new Map();
	const values = new Map();
	for (const node of outgoing) {
		if (!node.querySelectorAll) continue;
		collectPreserved(node, live);
		collectFormState(node, values);
	}
	if (live.size) restorePreserved(fragment, live);
	if (values.size) restoreFormState(fragment, values);
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
		if (control.type === "checkbox" || control.type === "radio") {
			if (control.checked !== control.defaultChecked) into.set(key, { checked: control.checked });
			continue;
		}
		// A file input's value cannot be set from script at all, so it is not
		// restorable by value; it belongs in a preserved island or outside the
		// region, which requirement:unified-update-runtime records as a gap.
		if (control.type === "file") continue;
		if (control.value !== control.defaultValue) into.set(key, { value: control.value });
	}
}

function restoreFormState(fragment, values) {
	const controls = fragment.querySelectorAll ? fragment.querySelectorAll("input, textarea, select") : [];
	for (const control of controls) {
		const held = values.get(control.name || control.id);
		if (!held) continue;
		if ("checked" in held) {
			// A changed default is the server asserting a new value, and it wins.
			if (control.checked === control.defaultChecked) control.checked = held.checked;
			continue;
		}
		if (control.value === control.defaultValue) control.value = held.value;
	}
}

// swapElement replaces one addressed element with rendered markup, carrying the
// client state across. It is what a delta operation, a redraw, and an action
// response all land through, so a region arrives the same way whichever asked.
export function swapElement(target, html) {
	if (!target || !target.parentNode) return false;
	const holder = document.createElement("template");
	holder.innerHTML = html;
	carryClientState([target], holder.content);
	target.replaceWith(holder.content);
	return true;
}

export function applyBoundary(id, fragment, html) {
	if (replaced) return false;
	const range = applied.get(id);
	if (range && !range.end.parentNode) {
		// An enclosing live boundary re-rendered and took this range with it.
		// The replacement subtree carries this boundary's placeholder again,
		// under the same id, so the lookup below finds it.
		applied.delete(id);
	} else if (range) {
		// A reconnect re-executes the page, so every boundary it renders is
		// delivered again, including the ones that settled once and have not
		// changed. Re-inserting identical nodes would restart animations, drop
		// focus and selection inside the region, and make a screen reader
		// announce a log nobody added to, so an identical delivery is left alone.
		if (html !== undefined && html === range.html) return true;
		refill(range, fragment, html);
		return true;
	}
	const placeholder = document.getElementById(id);
	if (!placeholder) return false;
	placeholder.replaceWith(bracket(id, fragment, html));
	return true;
}

export function applyHTML(id, html) {
	const holder = document.createElement("template");
	holder.innerHTML = html;
	return applyBoundary(id, holder.content, html);
}

export function replaceDocument(fragment) {
	replaced = true;
	stopLive();
	document.body.replaceChildren(fragment);
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
	return applied.size > 0 || document.querySelector("tb-boundary") !== null;
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
		const response = await fetch(location.href, {
			headers: withCSRF({ [modeHeader]: liveMode }),
			credentials: "same-origin",
			cache: "no-store",
			redirect: "error",
			signal: controller.signal,
		});
		if (!response.ok || !response.body) return { stop: false, retry: false };
		const reader = response.body.getReader();
		const decoder = new TextDecoder();
		let buffer = "";
		let closed = null;
		for (;;) {
			const chunk = await reader.read();
			if (chunk.done) break;
			buffer += decoder.decode(chunk.value, { stream: true });
			let newline = buffer.indexOf("\n");
			while (newline >= 0) {
				const line = buffer.slice(0, newline);
				buffer = buffer.slice(newline + 1);
				newline = buffer.indexOf("\n");
				if (!line) continue;
				let record;
				try {
					record = JSON.parse(line);
				} catch (error) {
					console.error("Popcorn Wave: unreadable live record", error);
					continue;
				}
				const outcome = handleRecord(record);
				if (outcome === "opened") opened = true;
				if (outcome === "stop") {
					controller.abort();
					return { stop: true, retry: false };
				}
				if (record.control === "closed") closed = record;
			}
		}
		if (closed && closed.reason === "done") return { stop: true, retry: false };
		if (closed) return { stop: false, retry: true, retryAfter: closed.retry_after_ms };
		// The stream ended with no terminal record, so it was cut off rather
		// than finished. Reconnecting is safe: the page executes again and
		// delivers the current state of every live region.
		return { stop: false, retry: false, opened: opened };
	} catch (error) {
		if (controller.signal.aborted) return { stop: true, retry: false };
		return { stop: false, retry: false };
	} finally {
		if (connection === controller) connection = null;
	}
}

function handleRecord(record) {
	if (record.control === "open") {
		// Boundary ids name positions in generated code. A deployment that
		// changed that code addresses a document this screen is not showing, so
		// applying anything from it would put content in the wrong place.
		if (documentVersion && record.version && record.version !== documentVersion) {
			reloadOnce("the server was deployed while this page was open");
			return "stop";
		}
		return "opened";
	}
	if (record.control === "reload") {
		reloadOnce("the server asked for a reload");
		return "stop";
	}
	if (record.control === "navigate") {
		const target = resolveNavigable(record.url);
		if (target) {
			stopLive();
			location.assign(target);
		}
		return "stop";
	}
	if (record.control === "closed") return "closed";
	if (typeof record.id === "string" && typeof record.html === "string") {
		if (!applyHTML(record.id, record.html)) {
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
	// An unknown record comes from a newer server. Ignoring it keeps an older
	// client working instead of tearing down a connection it could still use.
	return "ignored";
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
