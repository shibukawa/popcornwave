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
	};
	elements.set(id, node);
	return node;
}

const swapped = [];
const headInstalled = [];
let liveStarted = 0;
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

globalThis.document = {
	title: "",
	baseURI: "https://example.test/orders",
	cookie: "",
	activeElement: null,
	documentElement: root,
	head: { children: [], appendChild: (node) => headInstalled.push(node) },
	addEventListener: listen(listeners),
	getElementById: (id) => elements.get(id) || null,
	querySelector: (selector) => (selector.startsWith("main") ? mainLandmark : null),
	createElement: () => ({
		content: {},
		style: {},
		setAttribute() {},
		set textContent(value) {
			announced.push(value);
		},
		set innerHTML(value) {
			// Head installation is the only place the runtime parses a tag, and
			// what matters is that it reached the head at all, in order.
			this.content = { children: [{ tagName: "LINK", outerHTML: value, textContent: "" }] };
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
globalThis.fetch = async (url, init) => {
	requests.push({ url: url, headers: init.headers, signal: init.signal });
	if (!nextResponse) throw new Error("network");
	const answer = nextResponse;
	nextResponse = null;
	if (answer instanceof Error) throw answer;
	return answer;
};

// --- the boundary half, stubbed ---------------------------------------------

const prelude = `
function withCSRF(headers) { headers["X-CSRF-Token"] = "token"; return headers; }
function swapElement(target, html) { globalThis.__swapped.push({ id: target.id, html: html }); return globalThis.__swapOK; }
function applyHTML(id, html) { globalThis.__swapped.push({ placeholder: id, html: html }); return true; }
function startLive() { globalThis.__liveStarted(); }
function setPreserveAttribute() {}
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

const source = prelude + fs.readFileSync(path.join(here, "..", "update.js"), "utf8");
const module = path.join(here, ".update_harness_module.mjs");
fs.writeFileSync(module, source);
const { createUpdateRuntime } = await import("file://" + module);
fs.unlinkSync(module);

function fresh() {
	requests.length = 0;
	swapped.length = 0;
	headInstalled.length = 0;
	historyEntries.length = 0;
	announced.length = 0;
	focused.length = 0;
	assigned = null;
	reloaded = 0;
	liveStarted = 0;
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
{
	const runtime = fresh();
	element("card-1", { "data-tb-kind": "pages.Card" });
	nextResponse = response({
		headers: {
			"Pw-Render": "redraw",
			"Pw-Head": Buffer.from(JSON.stringify(['<link rel="stylesheet" href="/card.css">'])).toString("base64"),
		},
		text: '<article id="card-1">redrawn</article>',
	});
	const result = await runtime.redraw("card-1", { page: 7 });
	check(result.applied === true, "the redraw applied");
	check(requests[0].url.startsWith("https://example.test/orders?"), "the redraw went to the page URL");
	check(requests[0].url.includes("page=7"), "the declared parameter travelled in the query");
	check(requests[0].headers["Pw-Kind"] === "pages.Card", "the kind travelled in a header");
	check(requests[0].headers["Pw-Instance"] === "card-1", "the instance travelled in a header");
	check(headInstalled.length === 1, "the redraw's head contribution was installed");
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

if (failures) {
	console.error(failures + " check(s) failed");
	process.exit(1);
}
console.log("update runtime conformance: all checks passed");
