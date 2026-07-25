package wrappers

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
)

type request struct {
	ID int `path:"id"`
}

type response struct {
	ID int `json:"id"`
}

type AppConfig struct {
	Name string `default:"demo"`
}

type ImportCommand struct {
	Path string `arg:"required" help:"input path"`
}

var mux = pw.NewServeMux()

func init() {
	pw.RegisterConfig[AppConfig]("app")
	pw.SubCommand[ImportCommand]("import", "import data")
	mux.HandleFunc("GET /items/{id}", item)
}

func item(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[request](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	pw.WriteAPI(w, r, response{ID: input.ID})
}
