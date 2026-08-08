// Popcorn Wave update runtime.
//
// This is the framework's own implementation of the update wire contract that
// system:tinybind publishes. It replaced the module's bundled client, which used
// to be concatenated above this file: the protocol's names, its endpoints, and
// its server are this framework's, and having the browser half belong to a
// dependency meant a change this framework could make alone — where a redraw is
// addressed, most concretely — needed a coordinated release instead.
//
// It is written against the specification rather than against that client, and
// deliberately shares the apply core in boundary.js rather than carrying a
// second one. Two implementations of "put this markup there" on one page is what
// the merged asset used to be, and nothing made them agree.
//
// Every failure path here performs the ordinary browser navigation. That is the
// invariant the whole design rests on: it is what lets each capability be
// incomplete without ever being incorrect.

export function createUpdateRuntime(config) {
	const renderHeader = config.header + "-Render";
	const buildHeader = config.header + "-Build";
	const manifestHeader = config.header + "-Manifest";
	const kindHeader = config.header + "-Kind";
	const instanceHeader = config.header + "-Instance";
	const liveHeader = config.header + "-Live";
	const headHeader = config.header + "-Head";

	const idAttr = "data-" + config.attr + "-id";
	const kindAttr = "data-" + config.attr + "-kind";
	const ignoreAttr = "data-" + config.attr + "-ignore";
	const busyAttr = "data-" + config.attr + "-updating";
	setPreserveAttribute("data-" + config.attr + "-preserve");

	// The browser's own scroll restoration runs at popstate, against the document
	// still on screen, before the delta that makes the page that tall has arrived.
	// Taking it over is what lets the position be restored after the content is.
	if (typeof history === "object" && "scrollRestoration" in history) {
		history.scrollRestoration = "manual";
	}

	// The validators this client holds, keyed by instance id. They are a hint the
	// server uses to omit unchanged regions, and nothing else: an oversized
	// manifest is dropped rather than rejected, so a delta is never assumed
	// minimal.
	const manifest = new Map();

	// One ticket per target. A superseded response must not overwrite newer
	// state, and aborting it also stops spending bandwidth and parse work on a
	// response that can no longer land.
	const tickets = new Map();
	function claim(target) {
		const previous = tickets.get(target);
		if (previous) previous.controller.abort();
		const ticket = { controller: new AbortController() };
		tickets.set(target, ticket);
		return {
			signal: ticket.controller.signal,
			current: () => tickets.get(target) === ticket,
			release: () => {
				if (tickets.get(target) === ticket) tickets.delete(target);
			},
		};
	}

	const listeners = new Set();
	function emit(kind, detail) {
		for (const listener of listeners) {
			try {
				listener(kind, detail);
			} catch (error) {
				// A subscriber is a progress indicator or an analytics call. It
				// must not be able to break the update it is watching.
				console.error("Popcorn Wave: update subscriber failed", error);
			}
		}
	}

	// A request in flight is marked on the document root rather than reported only
	// through the events above, so a progress affordance is CSS an author writes
	// once instead of a subscriber every application writes again. It is additive:
	// nothing depends on the marker ever appearing, which is what keeps a page
	// that never runs this script from needing a rule about it.
	let inFlight = 0;
	function markBusy(busy) {
		const root = document.documentElement;
		if (!root || !root.setAttribute) return;
		inFlight += busy ? 1 : -1;
		if (inFlight === 1 && busy) root.setAttribute(busyAttr, "");
		else if (inFlight <= 0) {
			inFlight = 0;
			root.removeAttribute(busyAttr);
		}
	}

	// A document load announces itself; a delta has to say so. The region is
	// created at construction rather than at the moment there is something to
	// announce, because a live region inserted and filled in the same task is not
	// reliably read at all.
	let announcer = null;
	function installAnnouncer() {
		if (!document.body || !document.createElement) return;
		const region = document.createElement("div");
		if (!region.setAttribute || !region.style) return;
		region.setAttribute("aria-live", "polite");
		region.setAttribute("aria-atomic", "true");
		// Set through the CSSOM rather than as a style attribute, so a policy
		// forbidding inline style still gets a region that is out of the way.
		region.style.position = "absolute";
		region.style.width = "1px";
		region.style.height = "1px";
		region.style.overflow = "hidden";
		region.style.clip = "rect(0 0 0 0)";
		region.style.whiteSpace = "nowrap";
		document.body.appendChild(region);
		announcer = region;
	}

	function announce(text) {
		// A full document replacement takes the body's children with it, and the
		// region is one of them. Re-installing costs a check and is the difference
		// between announcing once and announcing until the first such replacement.
		if (announcer && announcer.isConnected === false) {
			announcer = null;
			installAnnouncer();
		}
		if (announcer && text) announcer.textContent = text;
	}

	// An update landing mid composition replaces the control being composed into,
	// which commits or discards whatever the user was midway through spelling. It
	// is the ordinary case for a Japanese or Chinese search box that updates as it
	// is typed, and it is the reason the boundary half exposes the probe at all.
	//
	// Deferring never resurrects stale markup: every caller re-checks its ticket
	// after waiting, so a response superseded while a composition was open is
	// discarded exactly as one superseded at any other moment is.
	function afterComposition() {
		if (!compositionActive()) return null;
		return new Promise((resolve) => {
			document.addEventListener("compositionend", () => resolve(), { once: true });
		});
	}

	// fall performs the navigation the browser would have performed. Every
	// failure ends here, so a user action is never lost.
	function fall(url, reason) {
		emit("fellBack", { url: url, reason: reason });
		if (url && url !== location.href) location.assign(url);
		else location.reload();
		return { applied: false, fellBack: true, reason: reason };
	}

	function headers(mode, extra) {
		const set = { [renderHeader]: mode, [buildHeader]: config.build };
		return Object.assign(set, extra || {});
	}

	// The served mode is checked on every response. A shared cache or a proxy
	// may have answered a delta request with the document body, and applying that
	// as a delta fills the page with markup that means nothing.
	function served(response) {
		const value = response.headers.get(renderHeader) || "";
		return value.split(";")[0].trim();
	}

	// Head is installed before the markup that needs it, which is the ordering
	// the contract makes normative: a delta reuses the live document shell, so a
	// component reachable for the first time has no stylesheet on the page and
	// markup landing first paints unstyled.
	function installHead(tags) {
		if (!tags || !tags.length) return;
		for (const tag of tags) {
			const holder = document.createElement("template");
			holder.innerHTML = tag;
			for (const node of Array.from(holder.content.children)) {
				if (node.tagName === "TITLE") {
					// A title accumulates into nonsense otherwise, and history
					// entries and assistive technology both read the old one.
					document.title = node.textContent || "";
					continue;
				}
				if (alreadyInHead(node)) continue;
				document.head.appendChild(node);
			}
		}
	}

	function alreadyInHead(node) {
		const markup = node.outerHTML;
		for (const existing of document.head.children) {
			if (existing.outerHTML === markup) return true;
		}
		return false;
	}

	// A boundary is addressed by the framework attribute first and by the
	// author's own id second, because a redraw and an action response rewrite
	// regions the author named.
	function locate(id) {
		const quoted = id.replace(/["\\]/g, "\\$&");
		return document.querySelector("[" + idAttr + '="' + quoted + '"]') || document.getElementById(id);
	}

	function applyOperation(operation) {
		// An unrecognized kind comes from a newer server. Ignoring it keeps this
		// client working rather than abandoning a stream it could still use.
		if (operation.kind !== "replace") return true;
		if (typeof operation.html !== "string") return true;
		const target = locate(operation.id);
		if (!target) return false;
		return swapElement(target, operation.html);
	}

	function recordValidator(entry) {
		if (entry && typeof entry.id === "string" && typeof entry.frame === "string") {
			manifest.set(entry.id, entry.frame);
		}
	}

	function replaceManifest(entries) {
		manifest.clear();
		for (const entry of entries || []) recordValidator(entry);
	}

	function manifestValue() {
		if (!manifest.size) return "";
		const pairs = [];
		for (const [id, frame] of manifest) pairs.push(id + ":" + frame);
		return pairs.join(",");
	}

	// A navigation, a refinement of the current route, and a back or forward are
	// one request. What differs is only what happens to the history entry and to
	// the viewport afterwards, and naming the three keeps that from being decided
	// by a boolean that means something different at each call site.
	//
	//	push    a link or a GET form: a new entry, and a new page to be at the top of
	//	replace the same page with different arguments: the entry and the viewport stay
	//	pop     back or forward: the browser already moved, so nothing is written
	async function go(url, mode) {
		const target = new URL(url, document.baseURI);
		if (target.origin !== location.origin) return fall(target.href, "cross-origin");
		// Recorded before anything is swapped. After the delta lands the page is a
		// different height and the position the user was at may already have
		// clamped, so reading it afterwards describes somewhere they never were.
		if (mode === "push") rememberScroll();
		const request = claim("navigation");
		markBusy(true);
		try {
			return await navigateClaimed(target, mode, request);
		} finally {
			markBusy(false);
			request.release();
		}
	}

	async function navigateClaimed(target, mode, request) {
		const current = request.current;
		emit("start", { url: target.href });
		const validators = manifestValue();
		let response;
		try {
			response = await fetch(target.href, {
				headers: withCSRF(headers("navigation", validators ? { [manifestHeader]: validators } : null)),
				credentials: "same-origin",
				redirect: "error",
				signal: request.signal,
			});
		} catch (error) {
			if (!current()) return { applied: false, superseded: true };
			return fall(target.href, "network");
		}
		if (!response.ok) return fall(target.href, "status");
		// A build mismatch is answered as a document, so the mode check catches it
		// with no separate branch: the page reloads and arrives holding the new
		// client, which is what makes deploying the two halves independent.
		if (served(response) !== "navigation") return fall(target.href, "not-a-delta");
		if (!current()) return { applied: false, superseded: true };

		const composition = afterComposition();
		if (composition) {
			await composition;
			if (!current()) return { applied: false, superseded: true };
		}

		const type = response.headers.get("Content-Type") || "";
		const outcome = type.includes("ndjson")
			? await consumeStream(response, current)
			: await consumeBuffered(response, current);
		if (outcome.fellBack) return fall(target.href, outcome.reason);
		if (outcome.superseded) return { applied: false, superseded: true };
		if (outcome.navigate) {
			const destination = resolveNavigable(outcome.navigate);
			// A target that would run script instead of navigating is a failure
			// like any other here, so it takes the ordinary path: reload, and
			// name the reason for whoever is watching the events.
			if (!destination) return fall(location.href, "unsafe-navigate");
			location.assign(destination);
			return { applied: false, navigated: true };
		}

		// History moves only after the response committed, so a delta that failed
		// leaves the address bar describing what is actually on screen.
		commitHistory(target, mode);
		settlePlace(target, mode);
		if (outcome.live || response.headers.get(liveHeader) === "1") startLive();
		emit("applied", { url: target.href });
		return { applied: true };
	}

	async function consumeBuffered(response, current) {
		let body;
		try {
			body = await response.json();
		} catch (error) {
			return { fellBack: true, reason: "unreadable" };
		}
		if (!current()) return { superseded: true };
		installHead(body.head);
		for (const operation of body.ops || []) {
			if (!applyOperation(operation)) return { fellBack: true, reason: "missing-target" };
		}
		replaceManifest(body.manifest);
		return { navigate: body.navigate, live: body.live === true };
	}

	// A streamed delta applies each region as it is written rather than when the
	// response ends, which is what makes a slow region cost only itself.
	async function consumeStream(response, current) {
		if (!response.body || !response.body.getReader) return { fellBack: true, reason: "not-a-stream" };
		const reader = response.body.getReader();
		const decoder = new TextDecoder();
		// Validators are held aside until the terminator arrives. A truncated
		// stream must not leave this client claiming regions it never received,
		// because the server would then omit them forever.
		const pending = [];
		let buffer = "";
		let ended = null;
		let navigate = null;
		for (;;) {
			let chunk;
			try {
				chunk = await reader.read();
			} catch (error) {
				return { fellBack: true, reason: "network" };
			}
			if (chunk.done) break;
			buffer += decoder.decode(chunk.value, { stream: true });
			let newline = buffer.indexOf("\n");
			while (newline >= 0) {
				const line = buffer.slice(0, newline);
				buffer = buffer.slice(newline + 1);
				newline = buffer.indexOf("\n");
				if (!line.trim()) continue;
				let record;
				try {
					record = JSON.parse(line);
				} catch (error) {
					return { fellBack: true, reason: "unreadable" };
				}
				if (!current()) return { superseded: true };
				if (record.r === "head") {
					// The opening record repeats the build, so a server redeployed
					// under an open connection is detectable without reading the
					// response headers again.
					if (record.build && record.build !== config.build) {
						return { fellBack: true, reason: "stale-build" };
					}
					installHead(record.head);
					continue;
				}
				if (record.r === "op") {
					pending.push({ id: record.id, frame: record.frame });
					// An op with no html says the region is unchanged: record the
					// validator and apply nothing.
					if (typeof record.html !== "string") continue;
					if (!applyOperation(record)) return { fellBack: true, reason: "missing-target" };
					continue;
				}
				if (record.r === "await") {
					// A placeholder id names a hole inside a region already
					// installed, which is a different namespace from an instance
					// id and lands through the boundary half of this asset.
					applyHTML(record.id, record.html);
					continue;
				}
				if (record.r === "end") {
					ended = record;
					continue;
				}
				if (record.r === "navigate" && record.url) navigate = record.url;
				// Anything else is from a newer server and is ignored.
			}
			if (ended) break;
		}
		// A stream that ended with no terminator was cut off rather than finished.
		if (!ended) return { fellBack: true, reason: "truncated" };
		if (ended.reason === "failed") {
			// Content already applied stays; what was not described is not
			// claimed, so the manifest is not updated.
			emit("failed", { error: ended.error });
			return { navigate: navigate };
		}
		replaceManifest(pending);
		return { navigate: navigate, live: ended.reason === "live_pending" };
	}

	function scrollState() {
		return { pwScroll: [window.scrollX, window.scrollY] };
	}

	// The entry being left records where the user actually was. Writing that
	// position into the entry being pushed — which is what this used to do —
	// describes the page they are arriving at, so back restored the scroll of the
	// page it came from and the entry a session opened on held nothing at all.
	function rememberScroll() {
		history.replaceState(scrollState(), "", location.href);
	}

	function commitHistory(target, mode) {
		// A pop already moved the browser. Writing an entry here would overwrite
		// the position that is about to be restored from it.
		if (mode === "pop") return;
		if (mode === "push") history.pushState(scrollState(), "", target.href);
		else history.replaceState(scrollState(), "", target.href);
	}

	// Where the user is left, once the regions have landed.
	//
	// A pushed navigation is arriving somewhere new, so it starts where a document
	// load would have started it: at the top, or at the fragment it named. A
	// replaced one is the same page with different arguments — a filter, a sort, a
	// page number — so moving the viewport would throw away the place the user was
	// reading. A pop restores what its entry recorded, and does it in the popstate
	// handler because only that has the entry.
	function settlePlace(target, mode) {
		if (mode !== "push") return;
		const anchor = target.hash ? locateAnchor(target.hash.slice(1)) : null;
		if (anchor && anchor.scrollIntoView) anchor.scrollIntoView();
		else if (window.scrollTo) window.scrollTo(0, 0);
		settleFocus(anchor);
		// The title is installed by the head records before this runs, so what is
		// read out is the page the user arrived at rather than the one they left.
		announce(document.title);
	}

	function locateAnchor(name) {
		if (!name) return null;
		return document.getElementById(name);
	}

	// Focus that was on a node the delta removed is now on the body, where a
	// keyboard is at the top of nothing. Sending it to the landmark the page
	// already has is the documented destination; a page with no main gets the
	// body, which is what a document load would have given it anyway.
	function settleFocus(anchor) {
		const active = document.activeElement;
		if (active && active !== document.body && active.isConnected) return;
		const landing = anchor || (document.querySelector ? document.querySelector("main, [role=main]") : null);
		if (!landing || !landing.focus) return;
		// A landmark is not focusable by default, and making it programmatically
		// focusable must not put it in the tab order.
		if (landing.hasAttribute && !landing.hasAttribute("tabindex")) {
			landing.setAttribute("tabindex", "-1");
		}
		landing.focus({ preventScroll: true });
	}

	// A redraw re-renders one registered component with the values the browser
	// holds. It is addressed at the page's own URL, so it inherits whatever
	// guards that page rather than needing a rule of its own.
	async function redraw(elementId, params, options) {
		const target = document.getElementById(elementId);
		if (!target) return { applied: false, reason: "no such element" };
		const kind = target.getAttribute(kindAttr);
		// Refusing locally costs no request. A component that is not reloadable
		// carries no kind, and a page that changed under this client is a
		// different question answered by the fallback below.
		if (!kind) return { applied: false, reason: "not a reloadable component" };
		const url = new URL((options && options.url) || location.href, document.baseURI);
		if (url.origin !== location.origin) return fall(url.href, "cross-origin");
		url.search = "";
		for (const name of Object.keys(params || {})) url.searchParams.set(name, String(params[name]));
		const request = claim("redraw:" + elementId);
		markBusy(true);
		try {
			return await redrawClaimed(elementId, kind, url, request);
		} finally {
			markBusy(false);
			request.release();
		}
	}

	async function redrawClaimed(elementId, kind, url, request) {
		const current = request.current;
		emit("start", { id: elementId });
		let response;
		try {
			response = await fetch(url.href, {
				headers: withCSRF(headers("redraw", { [kindHeader]: kind, [instanceHeader]: elementId })),
				credentials: "same-origin",
				redirect: "error",
				signal: request.signal,
			});
		} catch (error) {
			if (!current()) return { applied: false, superseded: true };
			return fall(location.href, "network");
		}
		// 404 means this deployment publishes no such kind, which makes the page
		// stale rather than the region. Every refusal ends the same way.
		if (!response.ok) return fall(location.href, "status:" + response.status);
		if (served(response) !== "redraw") return fall(location.href, "not-a-redraw");
		let html;
		try {
			html = await response.text();
		} catch (error) {
			return fall(location.href, "unreadable");
		}
		if (!current()) return { applied: false, superseded: true };
		const composition = afterComposition();
		if (composition) {
			await composition;
			if (!current()) return { applied: false, superseded: true };
		}
		installHead(decodeHead(response.headers.get(headHeader)));
		// The element is looked up again: the await above yields, and the region
		// may have been replaced by something else in the meantime.
		const landing = document.getElementById(elementId);
		if (!landing || !swapElement(landing, html)) return fall(location.href, "missing-target");
		emit("redrawn", { id: elementId });
		return { applied: true };
	}

	// A redraw's head travels base64-encoded because a head tag may hold any
	// character an attribute value may, and a header is not a place to discover
	// which of those a proxy passes through.
	function decodeHead(value) {
		if (!value) return null;
		try {
			const decoded = JSON.parse(atob(value));
			return Array.isArray(decoded) ? decoded : null;
		} catch (error) {
			return null;
		}
	}

	// apply installs what a mutating fetch returned. The application issues that
	// request itself, because it is the application's endpoint and its body.
	async function apply(response) {
		if (!response) return { applied: false, reason: "no response" };
		if (served(response) !== "action") return { applied: false, reason: "not an action response" };
		let body;
		try {
			body = await response.json();
		} catch (error) {
			return fall(location.href, "unreadable");
		}
		const composition = afterComposition();
		if (composition) await composition;
		installHead(body.head);
		for (const operation of body.ops || []) {
			// A rewritten region drops its stored validator, or a later navigation
			// could find that boundary unchanged and leave this markup in place.
			manifest.delete(operation.id);
			if (!applyOperation(operation)) return fall(location.href, "missing-target");
		}
		if (body.navigate) {
			const destination = resolveNavigable(body.navigate);
			if (!destination) return fall(location.href, "unsafe-navigate");
			location.assign(destination);
			return { applied: false, navigated: true };
		}
		emit("applied", {});
		// The status is not a signal to skip applying: a rejected submission
		// returns 4xx and the regions it carries are the validation errors.
		return { applied: true };
	}

	// The headers an application fetch must carry to ask for an action response.
	function updateHeaders() {
		return withCSRF(headers("action"));
	}

	// Same-origin navigation is intercepted so a link refines the page it is on.
	// Everything the browser should keep is left to it: a modified click, a
	// target, a download, a non-GET submission — which is what keeps
	// post-redirect-get working exactly as it did.
	function intercept() {
		document.addEventListener("click", (event) => {
			if (event.defaultPrevented || event.button !== 0) return;
			if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
			const link = event.target && event.target.closest ? event.target.closest("a[href]") : null;
			if (!link || link.target || link.hasAttribute("download")) return;
			if (link.closest("[" + ignoreAttr + "]")) return;
			const url = new URL(link.getAttribute("href"), document.baseURI);
			if (url.origin !== location.origin) return;
			// A fragment on the document already loaded is the browser's. It has
			// the element, it knows where to put it, and a round trip could only
			// arrive at the same page — while leaving it alone is also what keeps
			// the scroll, the :target styling, and back out of a deep link working
			// the way they do on every other page.
			if (sameDocument(url) && url.hash !== new URL(location.href).hash) return;
			event.preventDefault();
			go(url.href, "push");
		});
		document.addEventListener("submit", (event) => {
			if (event.defaultPrevented) return;
			const form = event.target;
			if (!form || !form.getAttribute) return;
			if (form.closest && form.closest("[" + ignoreAttr + "]")) return;
			// The submitter's own overrides decide the method, the action and the
			// target before this runtime decides whether it owns the submission at
			// all. Reading only the form is how a button declaring formmethod=post
			// inside a GET form used to be intercepted and re-sent as a GET.
			const submitter = event.submitter;
			if ((attributeOf(submitter, "formmethod") || form.method || "get").toLowerCase() !== "get") return;
			if (attributeOf(submitter, "formtarget") || form.target) return;
			const action = attributeOf(submitter, "formaction") || form.getAttribute("action") || location.href;
			const url = new URL(action, document.baseURI);
			if (url.origin !== location.origin) return;
			event.preventDefault();
			// A search form's fields become the query, which is how a form refines
			// the page it is on rather than leaving it. The submitter's own pair
			// belongs in that query and is added by hand: the FormData constructor
			// leaves every submit button out, because which one was pressed is not
			// a property of the form.
			const fields = new FormData(form);
			if (submitter && submitter.name) fields.append(submitter.name, submitter.value);
			url.search = new URLSearchParams(fields).toString();
			go(url.href, "push");
		});
		window.addEventListener("popstate", (event) => {
			const scroll = event.state && event.state.pwScroll;
			go(location.href, "pop").then(() => {
				// Restored after the delta, because the position only exists once
				// the content that makes the page that tall is back on it.
				if (scroll && window.scrollTo) window.scrollTo(scroll[0], scroll[1]);
			});
		});
	}

	function attributeOf(node, name) {
		return node && node.getAttribute ? node.getAttribute(name) : null;
	}

	function sameDocument(url) {
		const here = new URL(location.href);
		return url.pathname === here.pathname && url.search === here.search;
	}

	intercept();
	installAnnouncer();

	return {
		// Re-render the current route with different search parameters, replacing
		// the history entry rather than stacking one, and leaving the viewport
		// where it is because this is the page already on screen.
		//
		// The parameters given are the whole query: what is not named is dropped,
		// which is what a GET form submission does by specification and is why
		// this matches it rather than merging.
		update(params) {
			const url = new URL(location.href);
			url.search = "";
			for (const name of Object.keys(params || {})) url.searchParams.set(name, String(params[name]));
			return go(url.href, "replace");
		},
		navigate(url) {
			return go(url, "push");
		},
		redraw: redraw,
		apply: apply,
		updateHeaders: updateHeaders,
		subscribe(listener) {
			listeners.add(listener);
			return () => listeners.delete(listener);
		},
		// The attribute names are exposed because an application writing a
		// preserve marker from script needs the same name its templates use, and
		// guessing it from a prefix it did not choose is how the two drift.
		idAttribute: idAttr,
		kindAttribute: kindAttr,
		preserveAttribute: "data-" + config.attr + "-preserve",
		ignoreAttribute: ignoreAttr,
	};
}
