// The one piece of application script in this example.
//
// A redraw is triggered rather than automatic: the framework's runtime installs
// the capability and intercepts navigation, but nothing decides on its own that
// a region is stale. This says when.
document.addEventListener("click", (event) => {
	const button = event.target.closest("[data-redraw]");
	if (!button) return;
	const id = button.getAttribute("data-redraw");
	// A redraw's parameters come from whoever asks for it. The browser is
	// holding the values already, so they travel from the DOM rather than from
	// a lookup the server would have to authorize.
	const params = {
		id: id,
		title: button.getAttribute("data-title"),
		owner: button.getAttribute("data-owner"),
		at: button.getAttribute("data-at"),
	};
	// popcornwave is the global the runtime installs. redraw takes the element
	// id and the parameters the component declares; they travel in the query of
	// a request to this page's own URL, which is what makes the redraw inherit
	// whatever guards the page.
	window.popcornwave.redraw(id, params).then((result) => {
		const row = document.getElementById(id);
		if (!row || !result.applied) return;
		// Restart the flash by taking the class off and putting it back.
		row.classList.remove("row--redrawn");
		void row.offsetWidth;
		row.classList.add("row--redrawn");
	});
});

// Every update the runtime performs is announced, which is how a progress
// indicator or an analytics call attaches without patching the runtime. Here it
// just says what happened, so the console is a log of the exchange.
window.popcornwave.subscribe((kind, detail) => {
	console.log("popcornwave:", kind, detail);
});
