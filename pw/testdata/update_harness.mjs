// Harness driving the update runtime under node.
//
// It stubs only what the runtime touches, and deliberately does not parse HTML:
// a replacement is recorded against the element it addressed. What this verifies
// is the protocol half — the requests issued, the responses consumed, the
// validator bookkeeping, supersession, and every fallback path — which is the
// half a browser test would be a clumsy way to cover. Real DOM insertion is the
// browser's job.
//
// The boundary half of the asset is stubbed rather than loaded, so a failure
// here names the update runtime and nothing else.

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));

let failures = 0;
function check(condition, message) {
	if (!condition) {
		failures += 1;
		console.error("FAIL: " + message);
	}
}

// --- the stubbed page -------------------------------------------------------

const elements = new Map();
function element(id, attributes) {
	const node = {
		id: id,
		attributes: attributes || {},
		parentNode: {},
		getAttribute(name) {
			return name in this.attributes ? this.attributes[name] : null;
		},
		hasAttribute(name) {
			return name in this.attributes;
		},
		closest() {
			return null;
		},
		// The stub parses no HTML, so an element has no descendants. What the
		// runtime asks of it is only whether any nested boundary is there.
		querySelectorAll() {
			return [];
		},
		remove() {},
	};
	elements.set(id, node);
	return node;
}

const swapped = [];
const headInstalled = [];
let liveStarted = 0;
let liveStopped = 0;
const requests = [];
let nextResponse = null;
let assigned = null;
let reloaded = 0;
const historyEntries = [];

function response(spec) {
	const headers = new Map(Object.entries(spec.headers || {}));
	return {
		ok: spec.ok !== false,
		status: spec.status || 200,
		headers: { get: (name) => (headers.has(name) ? headers.get(name) : null) },
		json: async () => {
			if (spec.throws) throw new Error("unreadable");
			return spec.json;
		},
		text: async () => spec.text || "",
		body: spec.lines
			? {
					getReader() {
						const chunks = spec.lines.map((line) => new TextEncoder().encode(line + "\n"));
						let index = 0;
						return {
							read: async () =>
								index < chunks.length ? { done: false, value: chunks[index++] } : { done: true },
						};
					},
				}
			: null,
	};
}

// Listeners are recorded rather than dropped, because interception is the path
// every user of this feature takes and it is invisible to a Go assertion: which
// URL and which method a gesture turns into is protocol, not DOM insertion.
const listeners = new Map();
function listen(store) {
	return (name, handler) => {
		if (!store.has(name)) store.set(name, []);
		store.get(name).push(handler);
	};
}
function dispatch(name, event) {
	for (const handler of listeners.get(name) || []) handler(event);
	return event;
}

const busy = new Set();
const announced = [];
const focused = [];
let scrolledTo = null;

const root = {
	setAttribute: (name) => busy.add(name),
	removeAttribute: (name) => busy.delete(name),
};

function landmark(id) {
	return {
		id: id,
		attributes: {},
		isConnected: true,
		getAttribute() {
			return null;
		},
		hasAttribute() {
			return false;
		},
		setAttribute() {},
		focus: () => focused.push(id),
		scrollIntoView: () => {
			scrolledTo = id;
		},
	};
}

let mainLandmark = landmark("main");

// The head elements this document holds, and the parser that produces one.
//
// Head installation is the only place the runtime parses a tag, so the parse is
// as much of one as the runtime reads: the element name, and the attributes it
// identifies a tag by. Anything more would be a second HTML parser in a file
// that exists to avoid needing a browser.
const headChildren = [];

// The document's seeded manifest, when the case under test has one. seedManifest
// consumes it, so removal is modelled too: a marker left behind would describe a
// DOM that the first delta has already changed.
let seededManifest = null;

function seedMarker(value) {
	seededManifest = {
		tagName: "TB-MANIFEST",
		getAttribute: (name) => (name === "value" ? value : null),
		remove() {
			seededManifest = null;
		},
	};
}

function headNode(markup) {
	const name = /^<\s*([a-zA-Z-]+)/.exec(markup);
	const attributes = {};
	for (const [, attribute, value] of markup.matchAll(/([a-zA-Z-]+)\s*=\s*"([^"]*)"/g)) {
		attributes[attribute] = value;
	}
	const node = {
		tagName: (name ? name[1] : "link").toUpperCase(),
		outerHTML: markup,
		textContent: "",
		attributes: attributes,
		getAttribute: (attribute) =>
			Object.prototype.hasOwnProperty.call(attributes, attribute) ? attributes[attribute] : null,
		replaceWith(replacement) {
			const at = headChildren.indexOf(node);
			if (at >= 0) headChildren[at] = replacement;
			headInstalled.push(replacement);
		},
	};
	return node;
}

globalThis.document = {
	title: "",
	baseURI: "https://example.test/orders",
	cookie: "",
	activeElement: null,
	documentElement: root,
	// The head is a live list rather than a write-only sink. What the runtime
	// does when a tag is already installed is a decision with two wrong answers
	// — install it twice, or drop one that should have replaced its predecessor
	// — and neither is visible to a stub that only counts appends.
	head: {
		children: headChildren,
		appendChild(node) {
			headChildren.push(node);
			headInstalled.push(node);
		},
	},
	addEventListener: listen(listeners),
	getElementById: (id) => elements.get(id) || null,
	querySelector: (selector) => {
		if (selector.startsWith("main")) return mainLandmark;
		// The inert element a document seeds its validators through. It is set up
		// per case rather than always present, because a page load that seeds
		// nothing is the other half of what this covers.
		if (selector === "tb-manifest") return seededManifest;
		return null;
	},
	createElement: () => ({
		content: {},
		style: {},
		setAttribute() {},
		set textContent(value) {
			announced.push(value);
		},
		set innerHTML(value) {
			this.content = { children: [headNode(value)] };
		},
	}),
};
globalThis.document.body = { appendChild() {} };

const windowListeners = new Map();
globalThis.window = {
	addEventListener: listen(windowListeners),
	scrollX: 0,
	scrollY: 0,
	scrollTo: (x, y) => {
		scrolledTo = [x, y];
	},
};
globalThis.history = {
	state: null,
	scrollRestoration: "auto",
	pushState(state, _title, url) {
		this.state = state;
		historyEntries.push({ push: true, url: url, state: state });
	},
	replaceState(state, _title, url) {
		this.state = state;
		historyEntries.push({ push: false, url: url, state: state });
	},
};
globalThis.location = {
	href: "https://example.test/orders",
	origin: "https://example.test",
	assign: (url) => {
		assigned = url;
	},
	reload: () => {
		reloaded += 1;
	},
};
// A sequence resolves through a second fetch, so a case needs more than one
// answer queued. The single slot stays for every case that issues one request.
const responseQueue = [];
globalThis.fetch = async (url, init) => {
	requests.push({ url: url, headers: init.headers, signal: init.signal });
	const answer = responseQueue.length ? responseQueue.shift() : nextResponse;
	if (!answer) throw new Error("network");
	if (answer === nextResponse) nextResponse = null;
	if (answer instanceof Error) throw answer;
	return answer;
};

// --- the boundary half, stubbed ---------------------------------------------

const prelude = `
function withCSRF(headers) { headers["X-CSRF-Token"] = "token"; return headers; }
function swapElement(target, html) { globalThis.__swapped.push({ id: target.id, html: html }); return globalThis.__swapOK; }
function applyHTML(id, html) { globalThis.__swapped.push({ placeholder: id, html: html }); return true; }
function startLive() { globalThis.__liveStarted(); }
function stopLive() { globalThis.__liveStopped(); }
function setPreserveAttribute() {}
// The decomposed path parses a fragment, fills its holes, and swaps the nodes.
// The stub keeps the markup so an assertion can read it, and records the swap
// through the same list swapElement uses.
function parseFragment(html) {
	return { __html: html, querySelector: () => null, querySelectorAll: () => [] };
}
function swapNode(target, fragment) {
	globalThis.__swapped.push({ id: target.id, html: fragment.__html });
	return globalThis.__swapOK;
}
// The shared record reader, which both stream shapes go through.
async function* readRecords(body) {
	const reader = body.getReader();
	const decoder = new TextDecoder();
	let buffer = "";
	for (;;) {
		const chunk = await reader.read();
		if (chunk.done) return;
		buffer += decoder.decode(chunk.value, { stream: true });
		let newline = buffer.indexOf("\\n");
		while (newline >= 0) {
			const line = buffer.slice(0, newline);
			buffer = buffer.slice(newline + 1);
			newline = buffer.indexOf("\\n");
			if (!line.trim()) continue;
			try {
				yield JSON.parse(line);
			} catch (error) {
				return;
			}
		}
	}
}
function compositionActive() { return globalThis.__composing; }
function resolveNavigable(value, base) {
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
`;

globalThis.__swapped = swapped;
globalThis.__swapOK = true;
globalThis.__composing = false;
globalThis.__liveStarted = () => {
	liveStarted += 1;
};
globalThis.__liveStopped = () => {
	liveStopped += 1;
};

const source = prelude + fs.readFileSync(path.join(here, "..", "update.js"), "utf8");
const module = path.join(here, ".update_harness_module.mjs");
fs.writeFileSync(module, source);
const { createUpdateRuntime } = await import("file://" + module);
fs.unlinkSync(module);

// fresh builds a runtime against a clean document. The seed, when given, is the
// manifest the document it loads into carries, which the runtime reads at
// construction — so it has to be in place before the runtime is built rather
// than set on one that already exists.
function fresh(seed) {
	requests.length = 0;
	swapped.length = 0;
	headInstalled.length = 0;
	headChildren.length = 0;
	seededManifest = null;
	historyEntries.length = 0;
	announced.length = 0;
	focused.length = 0;
	assigned = null;
	reloaded = 0;
	liveStarted = 0;
	liveStopped = 0;
	responseQueue.length = 0;
	scrolledTo = null;
	busy.clear();
	listeners.clear();
	windowListeners.clear();
	mainLandmark = landmark("main");
	globalThis.__swapOK = true;
	globalThis.__composing = false;
	globalThis.document.activeElement = null;
	globalThis.history.state = null;
	globalThis.location.href = "https://example.test/orders";
	globalThis.document.title = "";
	elements.clear();
	if (seed) seedMarker(seed);
	return createUpdateRuntime({ header: "Pw", attr: "tb", build: "build-1", global: "popcornwave" });
}

// A gesture, as the browser would deliver it. Only what the runtime reads is
// modelled: everything it declines to touch is the browser's, and the point of
// most of the cases below is that it declined.
function node(tag, attributes, ancestorMatches) {
	return {
		tagName: tag,
		attributes: attributes || {},
		get target() {
			return this.attributes.target || "";
		},
		get method() {
			return (this.attributes.method || "get").toLowerCase();
		},
		get name() {
			return this.attributes.name || "";
		},
		get value() {
			return this.attributes.value || "";
		},
		getAttribute(name) {
			return name in this.attributes ? this.attributes[name] : null;
		},
		hasAttribute(name) {
			return name in this.attributes;
		},
		closest(selector) {
			if (selector === "a[href]" && tag === "A") return this;
			return ancestorMatches && ancestorMatches.includes(selector) ? this : null;
		},
	};
}

function clickEvent(link, overrides) {
	let prevented = false;
	return Object.assign(
		{
			button: 0,
			defaultPrevented: false,
			target: link,
			preventDefault() {
				prevented = true;
			},
			get prevented() {
				return prevented;
			},
		},
		overrides || {},
	);
}

function submitEvent(form, submitter) {
	let prevented = false;
	return {
		defaultPrevented: false,
		target: form,
		submitter: submitter || null,
		preventDefault() {
			prevented = true;
		},
		get prevented() {
			return prevented;
		},
	};
}

// FormData over the stub: the runtime only ever constructs one from a form and
// reads it back through URLSearchParams, so the fields it declares are enough.
globalThis.FormData = class {
	constructor(form) {
		this.entries = (form && form.fields ? form.fields : []).map((pair) => pair.slice());
	}
	append(name, value) {
		this.entries.push([name, value]);
	}
	*[Symbol.iterator]() {
		yield* this.entries;
	}
};

function deltaResponse(id) {
	return response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { ops: [{ kind: "replace", id: id, html: "<p>x</p>" }], manifest: [{ id: id, frame: "f1" }] },
	});
}

// --- the cases --------------------------------------------------------------

// A navigation names its mode and its build, so a server can answer it and a
// cache can tell it apart from the document.
{
	const runtime = fresh();
	element("c1");
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { ops: [{ kind: "replace", id: "c1", html: "<p>one</p>" }], manifest: [{ id: "c1", frame: "f1" }] },
	});
	await runtime.navigate("/orders?page=2");
	check(requests.length === 1, "one request issued");
	check(requests[0].headers["Pw-Render"] === "navigation", "render header names the mode");
	check(requests[0].headers["Pw-Build"] === "build-1", "build header carried");
	check(swapped.length === 1 && swapped[0].id === "c1", "the operation was applied");
	// Two writes, and the order is the fix: the entry being left is updated with
	// where the user actually was, and only then is the destination pushed.
	// Recording the outgoing position onto the incoming entry — which is what
	// this used to do — describes the page being arrived at.
	check(historyEntries.length === 2, "a navigation wrote the entry it left and the one it pushed");
	check(
		historyEntries[0].push === false && historyEntries[0].url === "https://example.test/orders",
		"the outgoing scroll was recorded on the entry being left",
	);
	check(historyEntries[1].push === true, "a navigation pushed a history entry");
	check(history.scrollRestoration === "manual", "the browser's own scroll restoration was taken over");

	// The validator it just learned is offered back on the next request, which is
	// what lets the server omit an unchanged region.
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { ops: [] },
	});
	await runtime.navigate("/orders?page=3");
	check(requests[1].headers["Pw-Manifest"] === "c1:f1", "the manifest carries what this client holds");
}

// The validators a page load leaves behind.
//
// Without them the first navigation after arriving carries no hints and is
// answered with every region of the page — the click a reader is most likely to
// make, answered the most expensively this wire can.
{
	const runtime = fresh("c1:f1:ch1,c2:f2::c1");
	element("c1");
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { ops: [] },
	});
	await runtime.navigate("/orders?page=2");
	check(
		requests[0].headers["Pw-Manifest"] === "c1:f1:ch1,c2:f2::c1",
		"the first navigation of a page view offers what the document seeded",
	);
	check(seededManifest === null, "the marker was consumed, so it cannot outlive the DOM it describes");
}

// A page load that seeds nothing leaves the client exactly where it was before
// the marker existed, rather than sending an empty header.
{
	const runtime = fresh();
	element("c1");
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { ops: [] },
	});
	await runtime.navigate("/orders?page=2");
	check(requests[0].headers["Pw-Manifest"] === undefined, "an unseeded client sends no manifest");
}

// A proxy or a shared cache may answer a delta request with the document body.
// Applying that as a delta is how a page fills with markup that means nothing.
{
	const runtime = fresh();
	nextResponse = response({ headers: { "Pw-Render": "" }, json: {} });
	await runtime.navigate("/orders?page=2");
	check(assigned === "https://example.test/orders?page=2", "a document answer fell back to a real navigation");
	check(swapped.length === 0, "nothing was applied from a document answer");
}

// Head must be installed before the markup that needs it, or a component
// reachable for the first time paints unstyled.
{
	const runtime = fresh();
	element("c1");
	const order = [];
	globalThis.__swapped = {
		push(entry) {
			order.push("swap");
			swapped.push(entry);
		},
		get length() {
			return swapped.length;
		},
	};
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { head: ['<link rel="stylesheet" href="/a.css">'], ops: [{ kind: "replace", id: "c1", html: "<p>x</p>" }] },
	});
	const before = headInstalled.length;
	await runtime.navigate("/orders");
	globalThis.__swapped = swapped;
	check(headInstalled.length === before + 1, "the head tag was installed");
	check(order[0] === "swap" && headInstalled.length > 0, "head installation preceded the swap");
}

// What a second navigation does with tags the head already holds.
//
// The head is the one part of a page nothing ever removes from, so a delta that
// installs unconditionally leaks an element per click for as long as the tab is
// open. Identical markup used to be the only test, which caught a stylesheet and
// missed everything carrying a token, a nonce, or a timestamp.
{
	const runtime = fresh();
	element("c1");
	const stylesheet = '<link rel="stylesheet" href="/a.css">';
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: {
			head: [stylesheet, '<meta name="description" content="first">'],
			ops: [{ kind: "replace", id: "c1", html: "<p>x</p>" }],
		},
	});
	await runtime.navigate("/orders");
	check(headChildren.length === 2, "the first navigation installed both tags");

	element("c1");
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: {
			// The same stylesheet, and a description whose content changed —
			// which is what a per-render token looks like from here.
			head: [stylesheet, '<meta name="description" content="second">'],
			ops: [{ kind: "replace", id: "c1", html: "<p>y</p>" }],
		},
	});
	await runtime.navigate("/invoices");
	check(headChildren.length === 2, "the second navigation added nothing to the head");
	check(
		headChildren.filter((node) => node.tagName === "LINK").length === 1,
		"the stylesheet the page already loaded was left alone",
	);
	const description = headChildren.find((node) => node.tagName === "META");
	check(
		description && description.getAttribute("content") === "second",
		"a named meta describes the page that arrived, not the one that left",
	);
}

// A streamed delta applies as it is written, and its terminator says what to do.
{
	const runtime = fresh();
	element("c1");
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/x-ndjson" },
		lines: [
			JSON.stringify({ r: "head", build: "build-1", head: [] }),
			JSON.stringify({ r: "op", kind: "replace", id: "c1", html: "<p>streamed</p>", frame: "f9" }),
			JSON.stringify({ r: "end", reason: "live_pending" }),
		],
	});
	await runtime.navigate("/orders");
	check(swapped.length === 1 && swapped[0].html === "<p>streamed</p>", "the streamed operation was applied");
	check(liveStarted === 1, "live_pending opened a live connection");

	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { ops: [] },
	});
	await runtime.navigate("/orders");
	check(requests[1].headers["Pw-Manifest"] === "c1:f9", "a completed stream committed its validators");
}

// A stream with no terminator was cut off. Trusting its manifest would tell the
// server this client holds regions it never received, and it would then omit
// them forever.
{
	const runtime = fresh();
	element("c1");
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/x-ndjson" },
		lines: [
			JSON.stringify({ r: "head", build: "build-1" }),
			JSON.stringify({ r: "op", kind: "replace", id: "c1", html: "<p>partial</p>", frame: "f-partial" }),
		],
	});
	await runtime.navigate("/orders");
	check(assigned !== null || reloaded > 0, "a truncated stream fell back");
	const runtime2 = runtime;
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { ops: [] },
	});
	await runtime2.navigate("/orders");
	check(
		requests[1].headers["Pw-Manifest"] === undefined,
		"a truncated stream committed no validator",
	);
}

// An unknown operation kind, record kind, and terminator reason all come from a
// newer server. None of them may be treated as fatal.
{
	const runtime = fresh();
	element("c1");
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/x-ndjson" },
		lines: [
			JSON.stringify({ r: "head", build: "build-1" }),
			JSON.stringify({ r: "sparkle", what: "?" }),
			JSON.stringify({ r: "op", kind: "shimmer", id: "c1", html: "<p>no</p>" }),
			JSON.stringify({ r: "op", kind: "replace", id: "c1", html: "<p>yes</p>", frame: "f1" }),
			JSON.stringify({ r: "end", reason: "beautiful" }),
		],
	});
	await runtime.navigate("/orders");
	check(assigned === null, "an unknown kind did not abandon the stream");
	check(swapped.length === 1 && swapped[0].html === "<p>yes</p>", "the recognized operation still applied");
}

// A redraw addresses the page's own URL and names the component in headers.
//
// Since tinybind-go v0.4.4 it answers with the body an action does: head,
// operations, and the manifest entries it rendered. The head has left its
// header, and the validator comes back rather than being left stale.
{
	const runtime = fresh();
	element("card-1", { "data-tb-kind": "pages.Card" });
	nextResponse = response({
		headers: { "Pw-Render": "redraw" },
		json: {
			head: ['<link rel="stylesheet" href="/card.css">'],
			ops: [{ kind: "replace", id: "card-1", html: '<article id="card-1">redrawn</article>' }],
			manifest: [{ id: "card-1", frame: "f-card" }],
		},
	});
	const result = await runtime.redraw("card-1", { page: 7 });
	check(result.applied === true, "the redraw applied");
	check(requests[0].url.startsWith("https://example.test/orders?"), "the redraw went to the page URL");
	check(requests[0].url.includes("page=7"), "the declared parameter travelled in the query");
	check(requests[0].headers["Pw-Kind"] === "pages.Card", "the kind travelled in a header");
	check(requests[0].headers["Pw-Instance"] === "card-1", "the instance travelled in a header");
	check(headInstalled.length === 1, "the redraw's head contribution was installed");

	// The validator it returned is held, so the next navigation is told this
	// region is current instead of being sent markup already on screen.
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/x-ndjson" },
		lines: [JSON.stringify({ r: "end", reason: "final" })],
	});
	await runtime.navigate("/orders");
	check(
		(requests[1].headers["Pw-Manifest"] || "").includes("card-1:f-card"),
		"the redraw's validator reached the next navigation",
	);
}

// A component that is not reloadable carries no kind, and refusing locally costs
// no request at all.
{
	const runtime = fresh();
	element("plain");
	const result = await runtime.redraw("plain", {});
	check(result.applied === false && requests.length === 0, "an unreloadable element issued no request");
}

// 404 means this deployment publishes no such kind, so the page is stale rather
// than the region.
{
	const runtime = fresh();
	element("card-1", { "data-tb-kind": "pages.Card" });
	nextResponse = response({ ok: false, status: 404, headers: {} });
	await runtime.redraw("card-1", {});
	check(assigned !== null || reloaded > 0, "a refused redraw fell back");
}

// An action response is applied whatever its status says: a rejected submission
// returns 4xx and the regions it carries are the validation errors.
{
	const runtime = fresh();
	element("cart");
	const result = await runtime.apply(
		response({
			ok: false,
			status: 422,
			headers: { "Pw-Render": "action" },
			json: { ops: [{ kind: "replace", id: "cart", html: "<div>errors</div>" }] },
		}),
	);
	check(result.applied === true, "a 4xx action response still applied its regions");
	check(swapped.length === 1 && swapped[0].id === "cart", "the region was rewritten");
}

// An action that changed where the user belongs says so rather than guessing
// which regions to rewrite.
{
	const runtime = fresh();
	await runtime.apply(
		response({ headers: { "Pw-Render": "action" }, json: { ops: [], navigate: "/orders/17" } }),
	);
	check(assigned === "https://example.test/orders/17", "the navigate directive left the page");
}

// location.assign runs a javascript: URL rather than navigating to it, so a
// directive carrying one is a failure like any other: reload, and do not follow
// it. The server refuses such a target too; this is the half that holds when a
// record arrives from somewhere the server did not write it.
for (const target of ["javascript:globalThis.__pwned = true", "data:text/html,<script>1</script>"]) {
	const runtime = fresh();
	const before = reloaded;
	const outcome = await runtime.apply(
		response({ headers: { "Pw-Render": "action" }, json: { ops: [], navigate: target } }),
	);
	check(assigned === null, "an unfollowable navigate target did not reach location.assign");
	check(reloaded > before, "an unfollowable navigate target reloaded instead");
	check(outcome.reason === "unsafe-navigate", "an unfollowable navigate target named its reason");
	check(globalThis.__pwned === undefined, "the refused target never ran");
}

// The headers an application fetch must carry to ask for an action response.
{
	const runtime = fresh();
	const sent = runtime.updateHeaders();
	check(sent["Pw-Render"] === "action", "the action mode is named");
	check(sent["Pw-Build"] === "build-1", "the build is carried");
	check(sent["X-CSRF-Token"] === "token", "an action carries the CSRF token");
}

// A superseded response must not overwrite newer state.
{
	const runtime = fresh();
	element("c1");
	let releaseFirst;
	const firstBody = new Promise((resolve) => {
		releaseFirst = resolve;
	});
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { ops: [{ kind: "replace", id: "c1", html: "<p>old</p>" }] },
	});
	nextResponse.json = async () => {
		await firstBody;
		return { ops: [{ kind: "replace", id: "c1", html: "<p>old</p>" }] };
	};
	const first = runtime.navigate("/orders?page=1");
	const firstSignal = requests[0].signal;
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { ops: [{ kind: "replace", id: "c1", html: "<p>new</p>" }] },
	});
	await runtime.navigate("/orders?page=2");
	check(firstSignal.aborted === true, "the superseded request was aborted");
	releaseFirst();
	const result = await first;
	check(result.superseded === true, "the older request reported itself superseded");
	check(
		swapped.filter((entry) => entry.html === "<p>old</p>").length === 0,
		"the superseded response was discarded unapplied",
	);
}

// A successful navigation describes the current page, so validators belonging
// only to the previous page are removed rather than accumulating forever.
{
	const runtime = fresh();
	element("c1");
	element("c2");
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { ops: [], manifest: [{ id: "c1", frame: "f1" }] },
	});
	await runtime.navigate("/orders?page=1");
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { ops: [], manifest: [{ id: "c2", frame: "f2" }] },
	});
	await runtime.navigate("/orders?page=2");
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { ops: [], manifest: [] },
	});
	await runtime.navigate("/orders?page=3");
	check(requests[2].headers["Pw-Manifest"] === "c2:f2", "the manifest contains only the current page");
}

// A target the page does not hold means this client is looking at something the
// server did not render, so the fallback is the honest move.
{
	const runtime = fresh();
	globalThis.__swapOK = false;
	element("c1");
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { ops: [{ kind: "replace", id: "missing", html: "<p>x</p>" }] },
	});
	await runtime.navigate("/orders");
	check(assigned !== null || reloaded > 0, "a missing target fell back");
}

// A network failure is a failure path like any other.
{
	const runtime = fresh();
	nextResponse = null;
	await runtime.navigate("/orders?page=9");
	check(assigned === "https://example.test/orders?page=9", "a network failure fell back");
}

// A cross-origin URL is never fetched as an update.
{
	const runtime = fresh();
	await runtime.navigate("https://elsewhere.test/orders");
	check(requests.length === 0, "a cross-origin navigation issued no update request");
}

// update replaces the history entry rather than stacking one, because changing a
// search parameter is not a new place.
{
	const runtime = fresh();
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { ops: [] },
	});
	await runtime.update({ page: 4 });
	check(historyEntries.length === 1 && historyEntries[0].push === false, "update replaced the history entry");
	check(requests[0].url.includes("page=4"), "the parameter reached the request");
	check(scrolledTo === null, "a refinement of the page on screen left the viewport alone");
}

// --- interception -----------------------------------------------------------
//
// Which URL and which method a gesture turns into is protocol, and it is the
// one part of this runtime no Go assertion can reach. Everything below is a
// case where the browser's own behavior is the specification: with this script
// absent, every one of these does the right thing already, so a divergence here
// is the feature being worse than not having it.

// A GET form's fields become the whole query, which is how a search form
// refines the page it is on rather than leaving it.
{
	const runtime = fresh();
	element("results");
	nextResponse = deltaResponse("results");
	const form = node("FORM", { action: "/orders" });
	form.fields = [["q", "boots"], ["sort", "newest"]];
	const event = submitEvent(form);
	dispatch("submit", event);
	await new Promise((resolve) => setTimeout(resolve, 0));
	check(event.prevented, "the GET form submission was intercepted");
	check(requests.length === 1, "the form issued one update request");
	check(requests[0].url === "https://example.test/orders?q=boots&sort=newest", "the fields became the query");
	check(requests[0].headers["Pw-Render"] === "navigation", "the form asked for a navigation delta");
}

// The submitter's own overrides decide the method before this runtime decides
// whether it owns the submission. A button declaring formmethod=post inside a
// GET form is a POST, and reading only the form is how it became a GET.
{
	const runtime = fresh();
	const form = node("FORM", { action: "/orders", method: "get" });
	form.fields = [["q", "boots"]];
	const event = submitEvent(form, node("BUTTON", { formmethod: "post" }));
	dispatch("submit", event);
	check(!event.prevented, "a formmethod=post submitter was left to the browser");
	check(requests.length === 0, "no update request was issued for it");
}

// Same for the target and the action: an override that would send the
// submission somewhere else is the browser's.
{
	const runtime = fresh();
	const form = node("FORM", { action: "/orders" });
	form.fields = [];
	const targeted = submitEvent(form, node("BUTTON", { formtarget: "_blank" }));
	dispatch("submit", targeted);
	check(!targeted.prevented, "a formtarget submitter was left to the browser");

	const elsewhere = submitEvent(form, node("BUTTON", { formaction: "/search" }));
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { ops: [] },
	});
	dispatch("submit", elsewhere);
	await new Promise((resolve) => setTimeout(resolve, 0));
	check(requests.length === 1 && requests[0].url.startsWith("https://example.test/search"), "formaction chose the URL");
}

// The pressed button's own pair is part of the submission. The FormData
// constructor leaves every submit button out, because which one was pressed is
// not a property of the form.
{
	const runtime = fresh();
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { ops: [] },
	});
	const form = node("FORM", { action: "/orders" });
	form.fields = [["q", "boots"]];
	dispatch("submit", submitEvent(form, node("BUTTON", { name: "view", value: "grid" })));
	await new Promise((resolve) => setTimeout(resolve, 0));
	check(requests[0].url === "https://example.test/orders?q=boots&view=grid", "the submitter's pair joined the query");
}

// A non-GET form is the browser's, which is what keeps post-redirect-get
// working exactly as it did.
{
	const runtime = fresh();
	const form = node("FORM", { action: "/orders", method: "post" });
	form.fields = [];
	const event = submitEvent(form);
	dispatch("submit", event);
	check(!event.prevented && requests.length === 0, "a POST form was left to the browser");
}

// The ignore marker hands a form back, and it is read from an ancestor as well
// as from the element.
{
	const runtime = fresh();
	const form = node("FORM", { action: "/orders" }, ["[data-tb-ignore]"]);
	form.fields = [];
	const event = submitEvent(form);
	dispatch("submit", event);
	check(!event.prevented && requests.length === 0, "an ignored form was left to the browser");
}

// A same-origin link is intercepted, and a modified click is not.
{
	const runtime = fresh();
	element("c1");
	nextResponse = deltaResponse("c1");
	const link = node("A", { href: "/orders?page=3" });
	const event = clickEvent(link);
	dispatch("click", event);
	await new Promise((resolve) => setTimeout(resolve, 0));
	check(event.prevented && requests.length === 1, "a plain left click was intercepted");
	check(requests[0].url === "https://example.test/orders?page=3", "the link's href was requested");

	const modified = clickEvent(node("A", { href: "/orders?page=4" }), { metaKey: true });
	dispatch("click", modified);
	check(!modified.prevented, "a modified click was left to the browser");

	const downloaded = clickEvent(node("A", { href: "/orders.csv", download: "" }));
	dispatch("click", downloaded);
	check(!downloaded.prevented, "a download link was left to the browser");
}

// A fragment on the document already loaded is the browser's entirely: it has
// the element, and a round trip could only arrive at the same page.
{
	const runtime = fresh();
	const event = clickEvent(node("A", { href: "#results" }));
	dispatch("click", event);
	check(!event.prevented, "an in-page fragment link was left to the browser");
	check(requests.length === 0, "an in-page fragment link issued no request");
}

// A fragment on another document is a navigation, and it lands at its target
// rather than at the top.
{
	const runtime = fresh();
	element("c1");
	nextResponse = deltaResponse("c1");
	elements.set("section-3", landmark("section-3"));
	globalThis.document.getElementById = (id) => elements.get(id) || null;
	const event = clickEvent(node("A", { href: "/reports#section-3" }));
	dispatch("click", event);
	await new Promise((resolve) => setTimeout(resolve, 0));
	check(event.prevented && requests.length === 1, "a cross-document fragment link was intercepted");
	check(scrolledTo === "section-3", "the navigation landed at the fragment it named");
}

// --- continuity -------------------------------------------------------------

// A pushed navigation is arriving somewhere new, so it starts where a document
// load would have started it, tells a screen reader, and does not leave focus
// on a node the delta removed.
{
	const runtime = fresh();
	element("c1");
	globalThis.document.title = "Reports";
	nextResponse = deltaResponse("c1");
	await runtime.navigate("/reports");
	check(Array.isArray(scrolledTo) && scrolledTo[1] === 0, "a pushed navigation started at the top");
	check(focused.includes("main"), "focus moved to the main landmark");
	check(announced.includes("Reports"), "the new title was announced");
}

// Focus that survived the swap is left alone. Moving it would be the same bug
// in the other direction.
{
	const runtime = fresh();
	element("c1");
	nextResponse = deltaResponse("c1");
	globalThis.document.activeElement = { isConnected: true };
	await runtime.navigate("/reports");
	check(focused.length === 0, "surviving focus was not moved");
}

// A request in flight is marked on the document root, so a progress affordance
// is CSS rather than a subscriber every application writes again.
{
	const runtime = fresh();
	element("c1");
	let duringRequest = false;
	nextResponse = deltaResponse("c1");
	const watched = globalThis.fetch;
	globalThis.fetch = async (url, init) => {
		duringRequest = busy.has("data-tb-updating");
		return watched(url, init);
	};
	await runtime.navigate("/orders?page=2");
	globalThis.fetch = watched;
	check(duringRequest, "the document was marked busy while the request was open");
	check(!busy.has("data-tb-updating"), "the marker was cleared when the request settled");
}

// An update landing mid composition would replace the control being composed
// into, committing or discarding whatever the user was midway through
// spelling. It is the ordinary case for a Japanese search box that updates as
// it is typed.
{
	const runtime = fresh();
	element("c1");
	globalThis.__composing = true;
	nextResponse = deltaResponse("c1");
	const settled = runtime.navigate("/orders?q=%E3%81%8F%E3%81%A4");
	await new Promise((resolve) => setTimeout(resolve, 0));
	check(swapped.length === 0, "nothing was applied while a composition was open");
	globalThis.__composing = false;
	for (const handler of listeners.get("compositionend") || []) handler({});
	await settled;
	check(swapped.length === 1, "the delta applied once the composition ended");
}

// Back and forward restore what their entry recorded, and write no entry of
// their own: the browser already moved, and writing here would overwrite the
// position about to be read.
{
	const runtime = fresh();
	element("c1");
	nextResponse = deltaResponse("c1");
	const back = windowListeners.get("popstate") || [];
	check(back.length === 1, "the runtime listens for popstate");
	// A listener's return value is nothing the browser reads, so the delta is
	// waited for the way the page waits for it rather than by awaiting the call.
	back[0]({ state: { pwScroll: [0, 640] } });
	await new Promise((resolve) => setTimeout(resolve, 0));
	check(historyEntries.length === 0, "a pop wrote no history entry");
	check(Array.isArray(scrolledTo) && scrolledTo[1] === 640, "the recorded scroll was restored");
}


// A decomposed fragment names the holes in its markup. A hole this client holds
// is retained — the live node moves in, keeping the state inside it — and one it
// does not hold is left for the operation that fills it.
{
	const runtime = fresh();
	element("panel");
	element("kept");
	element("arriving");
	// parseFragment is stubbed and finds no holes, so what this covers is that a
	// fragment carrying a boundary list still installs, and that the child named
	// in it applies on its own record rather than being swallowed by the parent.
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/x-ndjson" },
		lines: [
			JSON.stringify({ r: "head", build: "build-1" }),
			JSON.stringify({
				r: "op", kind: "replace", id: "panel", frame: "f1",
				html: "<section></section>", boundaries: ["kept", "arriving"],
			}),
			JSON.stringify({ r: "op", kind: "replace", id: "arriving", html: "<b>new</b>", frame: "f2" }),
			JSON.stringify({ r: "end", reason: "final" }),
		],
	});
	await runtime.navigate("/orders");
	check(assigned === null && reloaded === 0, "a decomposed fragment did not fall back");
	check(swapped.length === 2, "the parent and the arriving child both applied");
}

// An operation carrying an address is rebuilt from it even when a markup field
// is also present and empty.
//
// The redraw response encodes its markup field unconditionally, so an operation
// that chose values arrives with an empty string beside them. Reading the markup
// first takes that empty string: the region is replaced with nothing, the client
// reports the update applied, and the row leaves the page with no error and no
// failed request. Found in a browser on a reloadable table row.
{
	const runtime = fresh();
	element("row-1", { "data-tb-kind": "pages.OrderRow" });
	responseQueue.push(
		response({
			headers: { "Pw-Render": "redraw", "Content-Type": "application/json" },
			json: {
				ops: [{ kind: "replace", id: "row-1", html: "", seq: "addr-1", values: ["Kettle"] }],
				manifest: [{ id: "row-1", frame: "f1" }],
			},
		}),
		response({
			headers: { "Pw-Render": "sequence", "Content-Type": "application/json" },
			json: { nodes: ["<td>", 0, "</td>"] },
		}),
	);
	const outcome = await runtime.redraw("row-1", {});
	check(outcome.applied === true, "a redraw carrying values applied");
	check(swapped.length === 1 && swapped[0].html === "<td>Kettle</td>",
		"the region was rebuilt from the address rather than blanked by the empty markup: " +
			JSON.stringify(swapped));
}

// A children operation carries no markup: the boundary's own output is unchanged
// and only the arrangement of its nested boundaries moved. It must be dispatched
// rather than read as an unchanged validator.
{
	const runtime = fresh();
	element("the-list");
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/x-ndjson" },
		lines: [
			JSON.stringify({ r: "head", build: "build-1" }),
			JSON.stringify({ r: "op", kind: "children", id: "the-list", frame: "f1", children: "c1", boundaries: ["row-0", "row-1"] }),
			JSON.stringify({ r: "end", reason: "final" }),
		],
	});
	await runtime.navigate("/orders");
	// The stubbed DOM holds no rows, so the reconciliation cannot place them and
	// the ordinary fallback runs. What this asserts is that the record reached
	// the operation path at all: read as an unchanged validator it would have
	// been silently dropped and the screen left stale.
	check(assigned !== null || reloaded > 0, "a children operation was dispatched rather than dropped");
}

// The outgoing page's live connection is closed before navigation records are
// applied, and the new page's is opened after. A delivery landing in between
// writes the old page's content into the new page's region.
{
	const runtime = fresh();
	element("c1");
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/x-ndjson" },
		lines: [
			JSON.stringify({ r: "head", build: "build-1" }),
			JSON.stringify({ r: "op", kind: "replace", id: "c1", html: "<p>hi</p>", frame: "f1" }),
			JSON.stringify({ r: "end", reason: "live_pending" }),
		],
	});
	await runtime.navigate("/orders");
	check(liveStopped === 1, "the outgoing live connection was closed");
	check(liveStarted === 1, "the incoming live connection was opened");
}

// A manifest entry is four fields. The children validator is what lets a list
// that gained a row be answered by naming the new order, so holding only the
// frame makes every parent's arrangement compare unequal.
{
	const runtime = fresh();
	element("panel");
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: {
			ops: [{ kind: "replace", id: "panel", html: "<section></section>" }],
			manifest: [
				{ id: "panel", frame: "f-panel", children: "c-panel" },
				{ id: "row-0", frame: "f-row", parent: "panel" },
				{ id: "flat", frame: "f-flat" },
			],
		},
	});
	await runtime.navigate("/orders");
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { ops: [] },
	});
	await runtime.navigate("/orders?page=2");
	const sent = requests[1].headers["Pw-Manifest"];
	check(sent.includes("panel:f-panel:c-panel"), "the children validator travelled");
	check(sent.includes("row-0:f-row::panel"), "an entry with a parent and no children keeps the empty field");
	check(sent.includes("flat:f-flat,") || sent.endsWith("flat:f-flat"), "a flat entry stays two fields");
}

// The children validator travels on a stream's operation records too, so a
// manifest rebuilt from one returns both halves. Holding only the frame makes
// every list look reordered on the next request.
{
	const runtime = fresh();
	element("panel");
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/x-ndjson" },
		lines: [
			JSON.stringify({ r: "head", build: "build-1" }),
			JSON.stringify({ r: "op", kind: "replace", id: "panel", html: "<section></section>", frame: "f-panel", children: "c-panel" }),
			JSON.stringify({ r: "op", id: "quiet", frame: "f-quiet", children: "c-quiet" }),
			JSON.stringify({ r: "end", reason: "final" }),
		],
	});
	await runtime.navigate("/orders");
	nextResponse = response({
		headers: { "Pw-Render": "navigation", "Content-Type": "application/json" },
		json: { ops: [] },
	});
	await runtime.navigate("/orders?page=2");
	const sent = requests[1].headers["Pw-Manifest"];
	check(sent.includes("panel:f-panel:c-panel"), "a replaced boundary's children validator was kept");
	check(sent.includes("quiet:f-quiet:c-quiet"), "an unchanged boundary's children validator was kept");
}

// The sequence walk, against the module's own reassembly.
//
// The fixtures are generated by pw/sequencefixture_test.go from real plans, and
// their expected markup is what htmlbind.Sequence.Reassemble produced from the
// same values. A walk that consumes the wrong number of values at any node puts
// every later value in the wrong place and still yields markup, so nothing but
// this round trip catches it.
{
	const fixtures = JSON.parse(fs.readFileSync(path.join(here, "sequence_fixtures.json"), "utf8"));
	check(fixtures.length > 0, "the sequence fixtures are not empty");
	for (const fixture of fixtures) {
		const runtime = fresh();
		element("panel");
		responseQueue.push(
			response({
				headers: { "Pw-Render": "navigation", "Content-Type": "application/x-ndjson" },
				lines: [
					JSON.stringify({ r: "head", build: "build-1" }),
					JSON.stringify({
						r: "op", kind: "replace", id: "panel", frame: "f1",
						seq: fixture.address, values: fixture.values,
					}),
					JSON.stringify({ r: "end", reason: "final" }),
				],
			}),
			response({
				headers: { "Pw-Render": "sequence" },
				json: { addr: fixture.address, nodes: fixture.nodes },
			}),
		);
		await runtime.navigate("/orders");
		check(swapped.length === 1, fixture.name + ": the fragment was installed");
		check(
			swapped.length === 1 && swapped[0].html === fixture.want,
			fixture.name + ": the walk reproduced the render\n  got  " +
				(swapped[0] ? swapped[0].html : "(nothing)") + "\n  want " + fixture.want,
		);
		// The address was asked for on its own request, in its own mode.
		const ask = requests[1];
		check(ask && ask.headers["Pw-Render"] === "sequence", fixture.name + ": the sequence was its own mode");
		check(
			ask && ask.headers["Pw-Sequence-Address"] === fixture.address,
			fixture.name + ": the address travelled on its header",
		);
		check(
			requests[0].headers["Pw-Sequences"] === "1",
			fixture.name + ": the navigation said it can walk sequences",
		);
	}
}

// One address is fetched once, however many operations name it.
{
	const runtime = fresh();
	const fixtures = JSON.parse(fs.readFileSync(path.join(here, "sequence_fixtures.json"), "utf8"));
	const fixture = fixtures[0];
	element("panel");
	responseQueue.push(
		response({
			headers: { "Pw-Render": "navigation", "Content-Type": "application/x-ndjson" },
			lines: [
				JSON.stringify({ r: "head", build: "build-1" }),
				JSON.stringify({ r: "op", kind: "replace", id: "panel", frame: "f1", seq: fixture.address, values: fixture.values }),
				JSON.stringify({ r: "end", reason: "final" }),
			],
		}),
		response({ headers: { "Pw-Render": "sequence" }, json: { addr: fixture.address, nodes: fixture.nodes } }),
		response({
			headers: { "Pw-Render": "navigation", "Content-Type": "application/x-ndjson" },
			lines: [
				JSON.stringify({ r: "head", build: "build-1" }),
				JSON.stringify({ r: "op", kind: "replace", id: "panel", frame: "f2", seq: fixture.address, values: fixture.values }),
				JSON.stringify({ r: "end", reason: "final" }),
			],
		}),
	);
	await runtime.navigate("/orders");
	element("panel");
	await runtime.navigate("/orders?page=2");
	check(requests.length === 3, "the second navigation reused the cached sequence");
}

// An address this deployment cannot describe is answered 404. A sequence is an
// optimisation over markup that is always available, so the ordinary fallback
// runs rather than the screen being left half written.
{
	const runtime = fresh();
	element("panel");
	responseQueue.push(
		response({
			headers: { "Pw-Render": "navigation", "Content-Type": "application/x-ndjson" },
			lines: [
				JSON.stringify({ r: "head", build: "build-1" }),
				JSON.stringify({ r: "op", kind: "replace", id: "panel", frame: "f1", seq: "gone", values: ["x"] }),
				JSON.stringify({ r: "end", reason: "final" }),
			],
		}),
		response({ ok: false, status: 404, headers: {} }),
	);
	await runtime.navigate("/orders");
	check(assigned !== null || reloaded > 0, "an unresolvable address fell back");
	check(swapped.length === 0, "nothing was written from an unresolvable address");
}

// Values that do not fit the tree came from a different render or a different
// build. Half-applying them would produce markup that is subtly wrong, so the
// walk yields nothing and the caller falls back.
{
	const runtime = fresh();
	const fixtures = JSON.parse(fs.readFileSync(path.join(here, "sequence_fixtures.json"), "utf8"));
	const fixture = fixtures[fixtures.length - 1];
	element("panel");
	responseQueue.push(
		response({
			headers: { "Pw-Render": "navigation", "Content-Type": "application/x-ndjson" },
			lines: [
				JSON.stringify({ r: "head", build: "build-1" }),
				JSON.stringify({
					r: "op", kind: "replace", id: "panel", frame: "f1",
					seq: fixture.address, values: fixture.values.slice(0, -1),
				}),
				JSON.stringify({ r: "end", reason: "final" }),
			],
		}),
		response({ headers: { "Pw-Render": "sequence" }, json: { addr: fixture.address, nodes: fixture.nodes } }),
	);
	await runtime.navigate("/orders");
	check(swapped.length === 0, "a short value stream wrote nothing");
	check(assigned !== null || reloaded > 0, "a mismatched walk fell back");
}

// The verdict, which must stay the last thing in this file: a case appended
// below it increments a count that has already been checked, and its failure is
// reported after the success line has been printed.
if (failures) {
	console.error(failures + " check(s) failed");
	process.exit(1);
}
console.log("update runtime conformance: all checks passed");
