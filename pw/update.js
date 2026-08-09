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
	// The capability header says this client can walk a sequence tree; the
	// address header asks for one. They are two headers rather than one because
	// a request that walks sequences and a request for a sequence are different
	// requests — the first is a navigation, the second is its own mode.
	const sequenceHeader = config.header + "-Sequences";
	const sequenceAddressHeader = config.header + "-Sequence-Address";

	const idAttr = "data-" + config.attr + "-id";
	const kindAttr = "data-" + config.attr + "-kind";
	const ignoreAttr = "data-" + config.attr + "-ignore";
	// The placeholder a decomposed fragment leaves where a nested boundary sits.
	//
	// It is a template because a template is the one element the HTML parser
	// leaves where it was written. An unknown element inside a table is
	// foster-parented — lifted out of the tbody and inserted before the table —
	// so a list's holes would sit outside the list and the rows filling them
	// would land loose on the page. It renders nothing, which is what a hole must
	// do until it is filled, and it carries attributes, so the id is on it and
	// every lookup here finds it the same way it finds a boundary.
	//
	// A progressive render's await boundary is a comment fence rather than this,
	// which is the one place the two shapes differ: a template's contents do not
	// render, and a fallback that does not render is not a fallback.
	const placeholderElement = "template";
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

	// abortRedraws cancels every redraw still in flight.
	//
	// It runs before navigation records are applied, because a redraw addresses
	// an element of the page being left. Applied afterwards it would write a
	// component of the old page into the new one, at an id the new page may well
	// also use — and the ticket that would have caught the supersession is keyed
	// per instance, so it never sees a navigation at all.
	function abortRedraws() {
		for (const [target, ticket] of tickets) {
			if (target.startsWith("redraw:")) ticket.controller.abort();
		}
	}

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

	// Every request this client issues says it can walk a sequence, because it
	// can. Whether a fragment then travels as an address and values or as markup
	// is the server's choice per fragment: values are smaller on a large region
	// and larger on a small one, so neither answer is wrong and no bookkeeping
	// about which addresses this client holds needs to travel.
	//
	// A sequence request is the exception. Asking for a tree while claiming to
	// walk one says nothing, and the mode already says what it is.
	function headers(mode, extra) {
		const set = { [renderHeader]: mode, [buildHeader]: config.build };
		if (mode !== "sequence") set[sequenceHeader] = "1";
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

	// A fragment arrives decomposed: its own markup, with an empty placeholder
	// where each nested boundary sits, and the ids of those holes listed beside
	// it. A hole is filled one of two ways, and nothing in the markup says which.
	//
	// This client decides by what it holds. A hole whose boundary is on screen
	// takes that live node, moved rather than re-rendered, which is what keeps
	// the state inside it — a playing video, a third-party widget, an open
	// details element. A hole whose boundary this client does not hold is left
	// empty for the operation that fills it, which arrives later in the same
	// response because operations are ordered outermost first.
	//
	// Deciding by what we hold rather than by which operations this response
	// carries is what makes the streamed path work at all: when a parent lands,
	// its children's records have not arrived yet, so there is nothing to consult
	// but the DOM. Retaining a node an operation then replaces costs one swap and
	// converges on the same screen.
	function fillHoles(fragment, boundaries) {
		if (!boundaries || !boundaries.length) return;
		for (const id of boundaries) {
			const hole = fragment.querySelector("[" + idAttr + '="' + quote(id) + '"]');
			if (!hole) continue;
			const held = locate(id);
			// A node that is inside the region being replaced is not a retain: it
			// is about to be discarded along with its parent, and moving it would
			// be moving a node into its own replacement.
			if (held && held !== hole) hole.replaceWith(held);
		}
	}

	function quote(id) {
		return id.replace(/["\\]/g, "\\$&");
	}

	// A fragment's static half — its literal text — is identical in every render,
	// so it stops travelling. An operation then carries the address of that half
	// and the values that fill it, and this client rebuilds the markup.
	//
	// The address is a digest of the template's compiled shape, so a sequence is
	// immutable and the response carrying it is public: it is the one thing on
	// this wire that is not per user. That is also why it is fetched rather than
	// pushed — riding inside a private response would forfeit exactly that, and
	// re-send every sequence on every page load.
	const sequences = new Map();
	const sequenceRequests = new Map();

	// sequenceNodes returns the tree an address names, fetching it once. Two
	// operations naming one address share a single request rather than racing.
	function sequenceNodes(address) {
		if (sequences.has(address)) return Promise.resolve(sequences.get(address));
		let pending = sequenceRequests.get(address);
		if (!pending) {
			pending = fetchSequence(address).then((nodes) => {
				sequenceRequests.delete(address);
				// A null is cached too. An address this deployment cannot describe
				// will not start describing itself, and asking again on every
				// operation would turn one miss into one request per record.
				sequences.set(address, nodes);
				return nodes;
			});
			sequenceRequests.set(address, pending);
		}
		return pending;
	}

	async function fetchSequence(address) {
		let response;
		try {
			response = await fetch(location.href, {
				headers: withCSRF(headers("sequence", { [sequenceAddressHeader]: address })),
				credentials: "same-origin",
				redirect: "error",
			});
		} catch (error) {
			return null;
		}
		// 404 means this process has never rendered the plan behind the address,
		// so it cannot describe it. That is not a failure to recover from here: a
		// sequence is an optimization over markup that is always available, and
		// the caller falls back to asking for the markup.
		if (!response.ok || served(response) !== "sequence") return null;
		try {
			const body = await response.json();
			return body && Array.isArray(body.nodes) ? body.nodes : null;
		} catch (error) {
			return null;
		}
	}

	// materialize returns an operation's markup, whichever half it arrived as.
	// Null means this client cannot rebuild it and the caller falls back.
	//
	// The address wins where both are present, and that ordering is load-bearing
	// rather than arbitrary. An operation is supposed to carry one form or the
	// other, but the redraw response encodes its markup field unconditionally, so
	// choosing values leaves an empty string beside them. Reading the markup first
	// takes that empty string and blanks the region: the runtime reports the
	// redraw applied, the row leaves the page, and nothing anywhere says why.
	//
	// Preferring the address is also right on its own terms. An empty string is
	// a legitimate rendering — a region that now shows nothing — so it cannot be
	// told from an absent one, and the address is the unambiguous half.
	async function materialize(operation) {
		if (typeof operation.seq !== "string") {
			return typeof operation.html === "string" ? operation.html : null;
		}
		// Markup beside an address is the recovery when the address cannot be
		// resolved, which costs a swap this client already has the bytes for
		// instead of the whole page.
		const carried = operation.html ? operation.html : null;
		const nodes = await sequenceNodes(operation.seq);
		if (!nodes) {
			// Named separately from a missing target, because the two failures
			// look identical from the outside and have nothing in common: one is
			// a page that moved under this client, the other is an address this
			// deployment cannot describe.
			console.warn("Popcorn Wave: no sequence for", operation.seq);
			return carried;
		}
		const html = reassemble(nodes, operation.values || []);
		if (html === null) {
			console.warn("Popcorn Wave: values do not fit sequence", operation.seq,
				(operation.values || []).length);
			return carried;
		}
		return html;
	}

	// reassemble walks the tree and consumes the values, which is the whole of
	// what a client does with a sequence.
	//
	// One value per hole, per conditional (which branch ran), per loop (how many
	// times), and per component call (whether it opened a boundary). Consuming
	// the wrong number at any node puts every later value in the wrong place, so
	// a walk that does not end exactly at the end of the values is a mismatch and
	// yields nothing rather than markup that is subtly wrong.
	//
	// Nothing is escaped here. The values were escaped by the render that
	// produced them, which is the whole reason this client needs no escaping
	// rules of its own — and must not apply any.
	function reassemble(nodes, values) {
		const out = [];
		const consumed = walkSequence(nodes, values, 0, out);
		if (consumed !== values.length) return null;
		return out.join("");
	}

	// The node kinds, as the tree encodes them: a static run is a bare string, a
	// hole is the number zero, and everything else is an object naming its kind.
	const seqIf = 2;
	const seqRepeat = 3;
	const seqComponent = 4;

	function walkSequence(nodes, values, index, out) {
		for (const node of nodes) {
			if (typeof node === "string") {
				out.push(node);
				continue;
			}
			if (node === 0) {
				if (index >= values.length) return -1;
				out.push(values[index++]);
				continue;
			}
			if (index >= values.length) return -1;
			const marker = values[index++];
			let taken;
			if (node.k === seqIf) {
				if (marker !== "t" && marker !== "f") return -1;
				taken = marker === "f" ? node.e || [] : node.t;
			} else if (node.k === seqComponent) {
				if (marker !== "b" && marker !== "i") return -1;
				taken = marker === "i" ? node.e || [] : node.t;
			} else if (node.k === seqRepeat) {
				// The count is decimal digits, never a JavaScript number cast: an
				// empty string casts to zero and a trailing character casts to NaN,
				// and both would silently produce the wrong markup.
				if (!/^\d+$/.test(marker)) return -1;
				const count = parseInt(marker, 10);
				for (let repeat = 0; repeat < count; repeat++) {
					index = walkSequence(node.t, values, index, out);
					if (index < 0) return -1;
				}
				continue;
			} else {
				// A node kind from a newer server. There is no safe guess about
				// how many values it takes, so the walk stops.
				return -1;
			}
			index = walkSequence(taken, values, index, out);
			if (index < 0) return -1;
		}
		return index;
	}

	// applyOperation is async because a replacement may arrive as an address and
	// its values rather than as markup, and resolving the address can cost one
	// fetch. Every other kind decides without waiting.
	async function applyOperation(operation) {
		if (operation.kind === "children") return reconcileChildren(operation);
		// An unrecognized kind comes from a newer server. Ignoring it keeps this
		// client working rather than abandoning a stream it could still use.
		if (operation.kind !== "replace") return true;
		const html = await materialize(operation);
		// An operation carrying neither markup nor a resolvable address describes
		// a region this client cannot rebuild, so the caller falls back.
		if (typeof html !== "string") return false;
		const target = locate(operation.id);
		if (!target) return false;
		const fragment = parseFragment(html);
		fillHoles(fragment, operation.boundaries);
		return swapNode(target, fragment);
	}

	// A children operation says a boundary's own markup is unchanged and its
	// nested boundaries are now these, in this order.
	//
	// It exists because appending one row to a list is the ordinary event on a
	// live screen, and saying it by replacing the parent costs the whole list of
	// holes. So the parent is left alone and only the arrangement is stated.
	//
	// An id already on screen keeps its node, moving if the order moved. An id
	// the list drops is removed. An id this client does not hold gets an empty
	// placeholder, which the operation later in this response fills.
	function reconcileChildren(operation) {
		const parent = locate(operation.id);
		if (!parent) return false;
		const wanted = operation.boundaries || [];
		const held = directBoundaries(parent);
		if (!wanted.length) {
			for (const node of held) node.remove();
			return true;
		}
		// The container is where the children already are. Reading it from the
		// DOM rather than assuming the boundary's root is what keeps a list
		// wrapped in a ul working, and a parent holding none is a rearrangement
		// this client cannot place — it falls back rather than guessing.
		const anchor = held.length ? held[0] : null;
		if (!anchor) return false;
		const container = anchor.parentNode;
		if (!container) return false;
		const byID = new Map();
		for (const node of held) byID.set(node.getAttribute(idAttr), node);
		let before = anchor;
		for (const id of wanted) {
			let node = byID.get(id);
			if (node) {
				byID.delete(id);
			} else {
				// The operation that fills this arrives later in the same
				// response. Until then the placeholder holds the position, which
				// is what keeps the order the server stated.
				node = document.createElement(placeholderElement);
				node.setAttribute(idAttr, id);
			}
			// insertBefore moves a node that is already in the document, so a
			// reorder costs one move per element and no re-render.
			container.insertBefore(node, before);
			before = node.nextSibling;
		}
		// Whatever the list did not name is gone from the render.
		for (const node of byID.values()) node.remove();
		return true;
	}

	// directBoundaries returns the update boundaries immediately inside one,
	// skipping the ones nested deeper: a grandchild belongs to its own parent's
	// arrangement rather than to this one.
	function directBoundaries(parent) {
		const found = [];
		for (const node of parent.querySelectorAll("[" + idAttr + "]")) {
			const enclosing = node.parentElement && node.parentElement.closest("[" + idAttr + "]");
			if (enclosing === parent) found.push(node);
		}
		return found;
	}

	// A manifest entry is four fields, not two. The frame validator says whether
	// a boundary's own markup changed; the children validator says whether the
	// arrangement of the boundaries inside it did, which is what lets a list that
	// gained a row be answered by naming the new order rather than by re-sending
	// the list. Holding only the frame makes every parent's arrangement compare
	// unequal, so the server restates one on every navigation.
	function recordValidator(entry) {
		if (entry && typeof entry.id === "string" && typeof entry.frame === "string") {
			manifest.set(entry.id, {
				frame: entry.frame,
				children: typeof entry.children === "string" ? entry.children : "",
				parent: typeof entry.parent === "string" ? entry.parent : "",
			});
		}
	}

	function replaceManifest(entries) {
		manifest.clear();
		for (const entry of entries || []) recordValidator(entry);
	}

	// recordManifest merges entries into what this client holds, for a response
	// that describes part of the page rather than all of it. A redraw is that
	// case: it renders one component and the boundaries under it, and says
	// nothing about the rest of the screen, so clearing would throw away every
	// validator the last navigation established.
	function recordManifest(entries) {
		for (const entry of entries || []) recordValidator(entry);
	}

	// The trailing fields are written only when they carry something, which is
	// what keeps a flat page's manifest the size it always was. The order and the
	// omission rule are the server's encoder read backwards; a parent with no
	// children validator still writes an empty field, because the two positions
	// cannot be told apart otherwise.
	function manifestValue() {
		if (!manifest.size) return "";
		const entries = [];
		for (const [id, held] of manifest) {
			let entry = id + ":" + held.frame;
			if (held.children || held.parent) entry += ":" + held.children;
			if (held.parent) entry += ":" + held.parent;
			entries.push(entry);
		}
		return entries.join(",");
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

		// The order below is the contract, and every step of it is about the page
		// being left rather than the one arriving.
		//
		// An open composition goes first, because waiting for it yields: every
		// decision after this point has to be made against the page as it is
		// once the user has finished spelling, not as it was when the response
		// arrived.
		const composition = afterComposition();
		if (composition) {
			await composition;
			if (!current()) return { applied: false, superseded: true };
		}
		// The outgoing page's live connection goes next: it is executing that
		// page on the server and delivering to boundary ids this response is
		// about to reuse, so a delivery landing between here and the last record
		// writes the old page's content into the new page's region.
		//
		// Pending redraws go with it, for the same reason at a smaller scale.
		stopLive();
		abortRedraws();

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
		// And the new page's connection opens last, after every record landed and
		// after the viewport settled. Opening it earlier would have it deliver
		// into regions this response had not written yet, which is the same
		// collision as the outgoing one in the other direction.
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
			if (!(await applyOperation(operation))) return { fellBack: true, reason: "missing-target" };
		}
		replaceManifest(body.manifest);
		return { navigate: body.navigate, live: body.live === true };
	}

	// A streamed delta applies each region as it is written rather than when the
	// response ends, which is what makes a slow region cost only itself.
	async function consumeStream(response, current) {
		if (!response.body || !response.body.getReader) return { fellBack: true, reason: "not-a-stream" };
		// Validators are held aside until the terminator arrives. A truncated
		// stream must not leave this client claiming regions it never received,
		// because the server would then omit them forever.
		const pending = [];
		let ended = null;
		let navigate = null;
		try {
			for await (const record of readRecords(response.body)) {
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
					// All three fields of a manifest entry travel on every
					// operation record, including an unchanged one, because a
					// client rebuilding its manifest from a stream has no other
					// source for them and the next request is compared against
					// all three: the frame decides whether a region is resent,
					// the children digest whether a list looks reordered, and the
					// parent whether a disappearing boundary can be narrowed to
					// something smaller than replacing the outermost one.
					pending.push({
						id: record.id, frame: record.frame,
						children: record.children, parent: record.parent,
					});
					// A record with no kind restates a validator and nothing else:
					// the region is unchanged, so it is recorded and not applied.
					// Every kind is dispatched, because a kind carrying no markup
					// is not the same statement — a children operation says the
					// arrangement moved while the markup stayed.
					if (!record.kind) continue;
					if (!(await applyOperation(record))) return { fellBack: true, reason: "missing-target" };
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
					// The terminator is the last record by contract, so nothing
					// after it is read rather than read and ignored.
					ended = record;
					break;
				}
				if (record.r === "navigate" && record.url) navigate = record.url;
				// Anything else is from a newer server and is ignored.
			}
		} catch (error) {
			return { fellBack: true, reason: "network" };
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
		// Since system:tinybind v0.4.4 a redraw answers with the same body an
		// action does — head, operations, and the manifest entries it rendered.
		// The head has left its header, so no base64 decode is needed and a
		// component's tags travel where every other path's do; and the validators
		// now come back, where a redrawn region used to leave a stale one behind
		// and cost the next navigation a re-send of markup already on screen.
		let body;
		try {
			body = await response.json();
		} catch (error) {
			return fall(location.href, "unreadable");
		}
		if (!current()) return { applied: false, superseded: true };
		// An open composition is waited out before anything lands, and the wait
		// yields: applyOperation locates its target again on the other side, so
		// a region replaced in the meantime is found or reported missing rather
		// than written over a stale node.
		const composition = afterComposition();
		if (composition) {
			await composition;
			if (!current()) return { applied: false, superseded: true };
		}
		installHead(body.head);
		for (const operation of body.ops || []) {
			if (!(await applyOperation(operation))) return fall(location.href, "missing-target");
		}
		recordManifest(body.manifest);
		emit("redrawn", { id: elementId });
		return { applied: true };
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
			if (!(await applyOperation(operation))) return fall(location.href, "missing-target");
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
