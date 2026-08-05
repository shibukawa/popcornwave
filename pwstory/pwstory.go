// Package pwstory holds the registry the template storybook renders from.
//
// It exists because a generated template is Go code in the application's own
// module, so only a process compiled from that module can call it. pw generates
// one registration file per template package, under the pwdev build constraint,
// and a small main that imports them; this package is what those files register
// into and what that main serves.
//
// Nothing here is reachable from a release build. The registration files carry
// the build constraint, so an application compiled by pw build links no call to
// Register and therefore none of this package.
package pwstory

import (
	"sort"
	"sync"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// Template is one renderable template, registered from inside its own package.
//
// Registration happens there rather than through an exported symbol table
// because a generated fragment named for an unexported template is unreachable
// from anywhere else. What the storybook lists is therefore what the project
// generated, not the subset it chose to export.
type Template struct {
	// Package is the Go package the template was generated into, used to
	// group the list and to disambiguate a name two packages share.
	Package string
	// Name is the generated function name, which is the template's own name.
	Name string
	// Exported reports whether the template is reachable from outside its
	// package. The storybook shows both and says which is which, because an
	// unexported template is exactly the one nothing else can show.
	Exported bool
	// NewParams returns a pointer to a zero parameter value. The storybook
	// fills it by reflection or from a fixture and hands it back to Render.
	NewParams func() any
	// Render binds the template to a parameter pointer.
	Render func(params any) htmlbind.Fragment
}

var registry = struct {
	sync.RWMutex
	templates []Template
	// document is the application's own shell, registered by the package that
	// generated it. It is passed in rather than read from the framework so
	// that this package needs no import of pw and no development-only surface
	// on it.
	document *htmlbind.Wrapper
}{}

// Register adds a template. It is called from generated code at package
// initialisation, so it takes no error: a duplicate is a generation defect
// rather than a condition a running storybook can report.
func Register(template Template) {
	registry.Lock()
	defer registry.Unlock()
	registry.templates = append(registry.templates, template)
}

// RegisterDocument installs the shell a story may be rendered inside. The
// package holding document.pw.html registers it, because that package already
// binds the wrapper for the framework.
func RegisterDocument(wrapper htmlbind.Wrapper) {
	registry.Lock()
	defer registry.Unlock()
	registry.document = &wrapper
}

// Templates lists what was registered, ordered so the page is stable across
// runs. Package initialisation order is not, and a list that reshuffled itself
// between reloads would be read as the project having changed.
func Templates() []Template {
	registry.RLock()
	defer registry.RUnlock()
	templates := make([]Template, len(registry.templates))
	copy(templates, registry.templates)
	sort.Slice(templates, func(i, j int) bool {
		if templates[i].Package != templates[j].Package {
			return templates[i].Package < templates[j].Package
		}
		return templates[i].Name < templates[j].Name
	})
	return templates
}

// Lookup finds one template by package and name.
func Lookup(pkg, name string) (Template, bool) {
	for _, template := range Templates() {
		if template.Package == pkg && template.Name == name {
			return template, true
		}
	}
	return Template{}, false
}

func document() []htmlbind.Wrapper {
	registry.RLock()
	defer registry.RUnlock()
	if registry.document == nil {
		return nil
	}
	return []htmlbind.Wrapper{*registry.document}
}
