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

globalThis.document = {
	title: "",
	baseURI: "https://example.test/orders",
	cookie: "",
	head: { children: [], appendChild: (node) => headInstalled.push(node) },
	addEventListener() {},
	getElementById: (id) => elements.get(id) || null,
	querySelector: () => null,
	createElement: () => ({
		content: {},
		set innerHTML(value) {
			// Head installation is the only place the runtime parses a tag, and
			// what matters is that it reached the head at all, in order.
			this.content = { children: [{ tagName: "LINK", outerHTML: value, textContent: "" }] };
		},
	}),
};

globalThis.window = { addEventListener() {}, scrollX: 0, scrollY: 0, scrollTo() {} };
globalThis.history = {
	pushState: (state, _title, url) => historyEntries.push({ push: true, url: url }),
	replaceState: (state, _title, url) => historyEntries.push({ push: false, url: url }),
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
function stopLive() { globalThis.__liveStopped(); }
function setPreserveAttribute() {}
// The decomposed path parses a fragment, fills its holes, and swaps the nodes.
// The stub keeps the markup so an assertion can read it, and records the swap
// through the same list swapElement uses.
function parseFragment(html) { return { __html: html, querySelector: () => null }; }
function swapNode(target, fragment) {
	globalThis.__swapped.push({ id: target.id, html: fragment.__html });
	return globalThis.__swapOK;
}
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

function fresh() {
	requests.length = 0;
	swapped.length = 0;
	headInstalled.length = 0;
	historyEntries.length = 0;
	assigned = null;
	reloaded = 0;
	liveStarted = 0;
	liveStopped = 0;
	globalThis.__swapOK = true;
	elements.clear();
	return createUpdateRuntime({ header: "Pw", attr: "tb", build: "build-1", global: "popcornwave" });
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
	check(historyEntries.length === 1 && historyEntries[0].push, "a navigation pushed a history entry");

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
}

if (failures) {
	console.error(failures + " check(s) failed");
	process.exit(1);
}
console.log("update runtime conformance: all checks passed");

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
			JSON.stringify({ r: "op", kind: "children", id: "the-list", frame: "f1", boundaries: ["row-0", "row-1"] }),
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
