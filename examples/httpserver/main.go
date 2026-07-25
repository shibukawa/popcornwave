package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/shibukawa/popcornwave/pw"
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

type GenerateConfigCommand struct {
	Format string `arg:"required" help:"output format: toml or env"`
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[EchoRequest](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	pw.WriteAPI(w, r, EchoResponse{
		Message: input.Message,
		Count:   input.Count,
		Source:  "tinygo-httpbind",
	})
}

func writeConfigScaffold(format string, output io.Writer) error {
	switch format {
	case "toml", ".toml":
		return pw.WriteScaffoldTOML(output)
	case "env", ".env":
		return pw.WriteScaffoldEnv(output)
	default:
		return fmt.Errorf("unknown config format %q; use toml or env", format)
	}
}

func configureOpenAPI() error {
	return pw.SetOpenAPIInfo(pw.OpenAPIInfo{
		Title:   "Popcorn Wave Example API",
		Version: "1.0.0",
	})
}

func main() {
	pw.SubCommand[GenerateConfigCommand]("generate-config", "write merged configuration scaffolds")
	if err := pw.ParseConfig(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if command, ok := pw.Command[GenerateConfigCommand](); ok {
		err := writeConfigScaffold(command.Format, os.Stdout)
		if err != nil {
			fmt.Fprintln(os.Stderr, "generate-config:", err)
			os.Exit(2)
		}
		return
	}

	if err := configureOpenAPI(); err != nil {
		fmt.Println("openapi error:", err)
		return
	}

	// Popcorn Wave's mux is net/http.ServeMux on standard Go and the compatible
	// tinygodriver implementation on TinyGo.
	mux := pw.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello from Popcorn Wave netdev method=%s path=%q\n", r.Method, r.URL.Path)
	})
	mux.HandleFunc("POST /echo", echoHandler)
	mux.HandleFunc("GET /openapi.yaml", pw.OpenAPIYAML)
	if err := pw.Run(context.Background(), mux); err != nil {
		fmt.Println("server error:", err)
	}
}
