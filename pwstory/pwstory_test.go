package pwstory

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// A stand-in for what pw generates: a params struct, a plan, and a function
// binding one to the other.
type greetingParams struct {
	Title       string
	DisplayName string
	Count       int
	Tags        []string
}

var greetingOps = htmlbind.Builder[greetingParams]{}

var greetingPlan = &htmlbind.Plan[greetingParams]{
	Ops: []htmlbind.Op[greetingParams]{
		greetingOps.Static("<h1>"),
		greetingOps.Text(func(p greetingParams) string { return p.Title }),
		greetingOps.Static("</h1><p>"),
		greetingOps.Text(func(p greetingParams) string { return p.DisplayName }),
		greetingOps.Static("</p>"),
	},
}

func greeting(p greetingParams) htmlbind.Fragment { return htmlbind.Bind(greetingPlan, p) }

func register(t *testing.T) {
	t.Helper()
	registry.Lock()
	registry.templates = nil
	registry.document = nil
	registry.Unlock()
	Register(Template{
		Package: "templates", Name: "greeting", Exported: false,
		NewParams: func() any { return new(greetingParams) },
		Render:    func(p any) htmlbind.Fragment { return greeting(*p.(*greetingParams)) },
	})
}

func TestSynthesizedParametersAreRepresentativeAndNamed(t *testing.T) {
	params := &greetingParams{}
	Synthesize(params)
	if params.Title != "Title" {
		t.Errorf("Title = %q, want the field name read as words", params.Title)
	}
	if params.DisplayName != "Display Name" {
		t.Errorf("DisplayName = %q, want the field name split into words", params.DisplayName)
	}
	if params.Count == 0 {
		t.Error("a numeric field was left at zero, which shows nothing")
	}
	// One element would let a template that mishandles separators look right.
	if len(params.Tags) != sampleSlice {
		t.Errorf("Tags = %v, want %d elements", params.Tags, sampleSlice)
	}
}

// A story that changed every render would report nothing, so two renderings of
// the same template have to be identical.
func TestSynthesisIsDeterministic(t *testing.T) {
	first, second := &greetingParams{}, &greetingParams{}
	Synthesize(first)
	Synthesize(second)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("first = %+v, second = %+v", first, second)
	}
}

func TestStoryRendersTheTemplate(t *testing.T) {
	register(t)
	result := renderStory(Templates()[0], false)
	if result.Failed != "" {
		t.Fatalf("render failed: %s", result.Failed)
	}
	if !strings.Contains(result.Source, "<h1>Title</h1>") {
		t.Errorf("rendered = %q, want the synthesized title", result.Source)
	}
}

// The list is sorted rather than left in initialisation order, which is not
// stable and would reshuffle the page between reloads.
func TestTemplatesAreOrdered(t *testing.T) {
	register(t)
	Register(Template{Package: "templates", Name: "alpha", NewParams: func() any { return new(greetingParams) },
		Render: func(p any) htmlbind.Fragment { return greeting(*p.(*greetingParams)) }})
	Register(Template{Package: "aaa", Name: "zeta", NewParams: func() any { return new(greetingParams) },
		Render: func(p any) htmlbind.Fragment { return greeting(*p.(*greetingParams)) }})
	names := []string{}
	for _, template := range Templates() {
		names = append(names, template.Package+"."+template.Name)
	}
	want := []string{"aaa.zeta", "templates.alpha", "templates.greeting"}
	for index, name := range want {
		if names[index] != name {
			t.Fatalf("order = %v, want %v", names, want)
		}
	}
}

func TestIndexAndStoryPagesAreServed(t *testing.T) {
	register(t)
	recorder := httptest.NewRecorder()
	Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if body := recorder.Body.String(); !strings.Contains(body, "greeting") {
		t.Errorf("the index never listed the template:\n%s", body)
	}
	// An unexported template is exactly the one nothing else can show, so the
	// page says which it is rather than hiding the distinction.
	if body := recorder.Body.String(); !strings.Contains(body, "unexported") {
		t.Errorf("the index never marked the unexported template:\n%s", body)
	}

	recorder = httptest.NewRecorder()
	Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/story/templates/greeting", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "&lt;h1&gt;Title&lt;/h1&gt;") {
		t.Errorf("the story page never showed the emitted HTML:\n%s", body)
	}
}

// The preview is framed from a page carrying nothing of the storybook, so the
// harness stylesheet never reaches the markup under review.
func TestRawStoryCarriesOnlyTheTemplateOutput(t *testing.T) {
	register(t)
	recorder := httptest.NewRecorder()
	Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/raw/templates/greeting", nil))
	body := recorder.Body.String()
	if strings.Contains(body, "storybook") {
		t.Errorf("the raw story carried the harness chrome:\n%s", body)
	}
	if body != "<h1>Title</h1><p>Display Name</p>" {
		t.Errorf("raw = %q, want only the template output", body)
	}
}

func TestUnknownStoryIsNotFound(t *testing.T) {
	register(t)
	recorder := httptest.NewRecorder()
	Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/story/templates/nothing", nil))
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", recorder.Code)
	}
}
