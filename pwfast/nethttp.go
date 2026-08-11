package pwfast

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"

	"github.com/shibukawa/tinygodriver/fasthttp"
)

// NetHTTPHandler exposes a fasthttp handler through net/http. Source-function
// runtimes own a net/http listener and cannot accept RequestHandler directly;
// an in-memory HTTP/1 connection preserves the wire contract without teaching
// either runtime about the other's request structures.
func NetHTTPHandler(handler fasthttp.RequestHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handler == nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		serverConn, clientConn := net.Pipe()
		defer clientConn.Close()
		served := make(chan error, 1)
		go func() { served <- fasthttp.ServeConn(serverConn, handler) }()

		request := r.Clone(r.Context())
		request.Close = true
		request.RequestURI = ""
		if err := request.Write(clientConn); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		response, err := http.ReadResponse(bufio.NewReader(clientConn), request)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer response.Body.Close()
		for name, values := range response.Header {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}
		w.WriteHeader(response.StatusCode)
		_, copyErr := io.Copy(w, response.Body)
		serveErr := <-served
		if copyErr != nil && !errors.Is(copyErr, r.Context().Err()) {
			return
		}
		_ = serveErr
	})
}
