// A scoped component script as an author would write one: it names no
// declaration identity, takes its own element, and says what runs per instance.
export function setup(el, scope) {
	globalThis.__pageModuleEnters = (globalThis.__pageModuleEnters || 0) + 1;
	globalThis.__pageModuleElements = globalThis.__pageModuleElements || [];
	globalThis.__pageModuleElements.push(el);
	scope.on("app.frommodule", () => {
		globalThis.__pageModuleCalls = (globalThis.__pageModuleCalls || 0) + 1;
	});
	return () => {
		globalThis.__pageModuleLeaves = (globalThis.__pageModuleLeaves || 0) + 1;
	};
}
