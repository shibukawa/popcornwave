package pwstory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// AddressVar names the environment variable pw dev sets on the harness process
// to say where it should listen. The console proxies the pane to it, so the
// developer still sees one listener and one URL.
const AddressVar = "PW_STORYBOOK_ADDR"

// ListenAndServe runs the storybook. It is the whole body of the generated
// main, so that the generated file stays a list of imports and one call.
//
// The address is taken from the environment rather than a flag because the
// harness is started by pw dev and never by hand.
func ListenAndServe() error {
	address := strings.TrimSpace(os.Getenv(AddressVar))
	if address == "" {
		address = "127.0.0.1:0"
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	// The resolved address goes to stdout so pw can read it back when the
	// port was reserved rather than fixed.
	fmt.Printf("pw storybook listening on %s\n", listener.Addr())
	return (&http.Server{Handler: Handler()}).Serve(listener)
}

// Handler serves the storybook: an index of every registered template and one
// page per story.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", index)
	mux.HandleFunc("GET /story/{package}/{name}", story)
	mux.HandleFunc("GET /raw/{package}/{name}", raw)
	return mux
}

// rendering is one attempt to render a story, kept whole so a failure is shown
// in place of the story rather than in place of the page.
type rendering struct {
	HTML     template.HTML
	Source   string
	Params   string
	Failed   string
	InShell  bool
	HasShell bool
}

func renderStory(t Template, shell bool) rendering {
	result := rendering{InShell: shell, HasShell: len(document()) > 0}
	params := t.NewParams()
	Synthesize(params)
	if encoded, err := json.MarshalIndent(params, "", "  "); err == nil {
		result.Params = string(encoded)
	}
	var out bytes.Buffer
	err := func() (err error) {
		// A template that panics is a defect the developer is looking for, so
		// it is reported as the story's result rather than taking the harness
		// down and with it every other story.
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("panic: %v", recovered)
			}
		}()
		fragment := t.Render(params)
		if shell {
			return htmlbind.RenderChain(&out, document(), fragment)
		}
		return htmlbind.Render(&out, fragment)
	}()
	if err != nil {
		result.Failed = err.Error()
		return result
	}
	result.Source = out.String()
	result.HTML = template.HTML(result.Source)
	return result
}

func index(w http.ResponseWriter, r *http.Request) {
	writePage(w, r, indexPage, map[string]any{"Templates": Templates()})
}

func story(w http.ResponseWriter, r *http.Request) {
	t, ok := Lookup(r.PathValue("package"), r.PathValue("name"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	shell := r.URL.Query().Get("shell") == "1"
	writePage(w, r, storyPage, map[string]any{
		"Template":  t,
		"Templates": Templates(),
		"Rendering": renderStory(t, shell),
	})
}

// raw serves the story on its own, with nothing of the storybook around it, so
// the preview can be framed without the harness stylesheet reaching it.
func raw(w http.ResponseWriter, r *http.Request) {
	t, ok := Lookup(r.PathValue("package"), r.PathValue("name"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	result := renderStory(t, r.URL.Query().Get("shell") == "1")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if result.Failed != "" {
		http.Error(w, result.Failed, http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte(result.Source))
}

// panePrefixHeader is how the console tells a pane where it is mounted. It
// matches devconsole.PanePrefixHeader, which is in an internal package this one
// cannot import.
const panePrefixHeader = "X-Pw-Pane-Prefix"

// writePage renders one storybook page.
//
// The mount prefix is added here rather than by each handler, because every
// link on every page needs it and a page that forgot would be the bug this
// exists to fix.
func writePage(w http.ResponseWriter, r *http.Request, page *template.Template, data map[string]any) {
	data["Prefix"] = r.Header.Get(panePrefixHeader)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := page.Execute(w, data); err != nil {
		fmt.Fprintf(w, "<p>storybook template error: %s</p>", template.HTMLEscapeString(err.Error()))
	}
}
