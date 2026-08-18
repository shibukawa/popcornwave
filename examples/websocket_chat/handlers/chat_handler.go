//go:build !fasthttp

package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
)

func init() {
	mux.HandleFunc("GET /{$}", home)
	mux.HandleFunc("GET /ws", chatSocket)
}

// home sends the browser to the page that opens the socket. The client is a
// static file rather than a template: this example is about the connection, and
// a page with no server-rendered data has nothing to render.
func home(w http.ResponseWriter, r *http.Request) {
	pw.Redirect(w, r, "/public/index.html", http.StatusFound)
}

// chatSocket upgrades the request and runs the room protocol.
//
// Neither type argument is spelled here. Generation reads ClientMsg and
// ServerMsg out of the closure parameter and writes the two codecs, so Read
// returns a struct and Write takes one.
func chatSocket(w http.ResponseWriter, r *http.Request) {
	if err := pw.WebSocket(w, r, func(socket *pw.Socket[ClientMsg, ServerMsg]) error {
		// Whatever the callback returns, this runs before the connection goes
		// away — so a tab closed mid-sentence still leaves the room.
		defer func() {
			if name, joined := chat.leave(socket); joined {
				chat.broadcast(ServerMsg{
					Type: "presence",
					Text: name + " left",
					Live: chat.size(),
				})
			}
		}()

		for {
			in, err := socket.Read()
			if err != nil {
				// A peer that went away, or one that stayed silent past the
				// idle timeout. Returning nil calls that an ordinary goodbye;
				// returning err would send it to the error handler installed in
				// main, which is for failures worth looking at.
				return nil
			}

			switch in.Type {
			case "join":
				if in.Name == "" {
					if err := socket.Write(ServerMsg{Type: "error", Code: "name_required"}); err != nil {
						return err
					}
					continue
				}
				chat.join(socket, in.Name)
				if err := socket.Write(ServerMsg{
					Type: "welcome",
					Text: in.Name,
					Live: chat.size(),
				}); err != nil {
					return err
				}
				chat.broadcast(ServerMsg{
					Type: "presence",
					Text: in.Name + " joined",
					Live: chat.size(),
				})

			case "say":
				name, joined := chat.name(socket)
				if !joined {
					if err := socket.Write(ServerMsg{Type: "error", Code: "join_first"}); err != nil {
						return err
					}
					continue
				}
				chat.broadcast(ServerMsg{Type: "message", From: name, Text: in.Text})

			default:
				if err := socket.Write(ServerMsg{Type: "error", Code: "unknown_type"}); err != nil {
					return err
				}
			}
		}
	}); err != nil {
		// The refusal — a cross-site origin, a request that is not an upgrade —
		// has already been written as a problem document. This is for the log.
		pw.Logger(r).Warn("chat upgrade refused", pw.Err(err))
	}
}
