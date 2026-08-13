// A scoped component script as an author would write one: it names no
// declaration identity, destructures what it needs from the one argument it is
// handed, and says what runs per instance.
//
// The teardown is registered rather than returned, which is what leaves the
// return value meaning one thing — the handlers this component publishes.
export function setup({ el, onSignal, teardown, props }) {
	globalThis.__pageModuleEnters = (globalThis.__pageModuleEnters || 0) + 1;
	globalThis.__pageModuleElements = globalThis.__pageModuleElements || [];
	globalThis.__pageModuleElements.push(el);
	globalThis.__pageModuleProps = props;
	onSignal("app.frommodule", () => {
		globalThis.__pageModuleCalls = (globalThis.__pageModuleCalls || 0) + 1;
	});
	teardown(() => {
		globalThis.__pageModuleLeaves = (globalThis.__pageModuleLeaves || 0) + 1;
	});
	return {
		announce() {
			globalThis.__pageModuleHandled = (globalThis.__pageModuleHandled || 0) + 1;
		},
	};
}
