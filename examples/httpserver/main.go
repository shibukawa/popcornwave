package main

import (
	"fmt"
	"net/http"
	"os"

	httpbinder "github.com/shibukawa/httpbind-go"
	"github.com/shibukawa/tinygodriver/httpmux"
	_ "github.com/shibukawa/tinygodriver/netdev" // Registers the host Netdever for TinyGo's net package.
)

type EchoRequest struct {
	Message string `payload:"message" check:"required,minlen=1,maxlen=128"`
	Count   int    `payload:"count" check:"required,min=1,max=10"`
}

type EchoResponse struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
	Source  string `json:"source"`
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	input, err := httpbinder.Bind[EchoRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	if err := httpbinder.Write[EchoResponse](w, r, EchoResponse{
		Message: input.Message,
		Count:   input.Count,
		Source:  "tinygo-httpbind",
	}); err != nil {
		httpbinder.WriteError(w, r, err)
	}
}

func openAPIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	httpbinder.OpenAPIJSON(w, r)
}

// describeRoutes gives httpbinder-gen method-aware route metadata.
func describeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /echo", echoHandler)
	mux.HandleFunc("GET /openapi.json", httpbinder.OpenAPIJSON)
}

func main() {
	mux := httpmux.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello from petitweb netdev method=%s path=%q\n", r.Method, r.URL.Path)
	})
	mux.HandleFunc("POST /echo", echoHandler)
	mux.HandleFunc("GET /openapi.json", openAPIHandler)
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	fmt.Println("listening on", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Println("server error:", err)
	}
}
