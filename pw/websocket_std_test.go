//go:build !tinygo

package pw

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornweb/pwruntime"
	"github.com/shibukawa/tinybind-go/jsonbind"
	"github.com/shibukawa/tinygodriver/websocket"
)

// The two halves of a test protocol. Generation emits a decoder for the first
// and an encoder for the second; registering them by hand here is the same
// registry that output writes into.
type socketIn struct {
	Text string `json:"text"`
}

type socketOut struct {
	Text string `json:"text"`
}

func init() {
	jsonbind.RegisterDecode(func(data []byte) (socketIn, error) {
		var in socketIn
		text := string(data)
		start := strings.Index(text, `"text":"`)
		if start < 0 {
			return in, nil
		}
		rest := text[start+len(`"text":"`):]
		in.Text = rest[:strings.IndexByte(rest, '"')]
		return in, nil
	})
	jsonbind.RegisterEncode(func(w io.Writer, out socketOut) error {
		_, err := w.Write([]byte(`{"text":"` + out.Text + `"}`))
		return err
	})
}

// publishedSettings installs chain settings for one test and restores what was
// there. Nothing in this package reads them; the socket origin check does.
func publishedSettings(t *testing.T, settings pwruntime.ChainSettings) {
	t.Helper()
	previous, had := pwruntime.ResolvedChainSettings()
	pwruntime.PublishChainSettings(settings)
	t.Cleanup(func() {
		if had {
			pwruntime.PublishChainSettings(previous)
			return
		}
		pwruntime.PublishChainSettings(pwruntime.ChainSettings{})
	})
}

func echoSocket(w http.ResponseWriter, r *http.Request) {
	_ = WebSocket(w, r, func(socket *Socket[socketIn, socketOut]) error {
		for {
			in, err := socket.Read()
			if err != nil {
				return nil
			}
			if err := socket.Write(socketOut{Text: "echo:" + in.Text}); err != nil {
				return err
			}
		}
	})
}

// The whole point of the typed socket: a struct goes in and a struct comes
// back, over a real handshake and a real frame.
func TestASocketCarriesTypedMessagesBothWays(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(echoSocket))
	t.Cleanup(server.Close)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"text":"hello"}`)); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"echo:hello"`)) {
		t.Fatalf("the socket answered %q", data)
	}
}

// A browser on another site is what the origin check exists to refuse, and the
// refusal is this framework's problem document rather than the library's plain
// text.
func TestAnUpgradeFromAnotherOriginIsRefused(t *testing.T) {
	publishedSettings(t, pwruntime.ChainSettings{})
	server := httptest.NewServer(http.HandlerFunc(echoSocket))
	t.Cleanup(server.Close)

	header := http.Header{}
	header.Set("Origin", "https://evil.example")
	conn, response, err := websocket.DefaultDialer.Dial(wsURL(server.URL), header)
	if err == nil {
		_ = conn.Close()
		t.Fatal("an upgrade from another origin completed")
	}
	if response == nil {
		t.Fatalf("the handshake failed without a response: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("refused with %d, not 403", response.StatusCode)
	}
	body, _ := io.ReadAll(response.Body)
	if !bytes.Contains(body, []byte(`"websocket_origin"`)) {
		t.Fatalf("the refusal is not this framework's problem document: %q", body)
	}
}

// A browser on this deployment's own origin has to get through, or the check
// above is a socket nobody can open.
func TestAnUpgradeFromThisOriginIsAdmitted(t *testing.T) {
	publishedSettings(t, pwruntime.ChainSettings{})
	server := httptest.NewServer(http.HandlerFunc(echoSocket))
	t.Cleanup(server.Close)

	header := http.Header{}
	header.Set("Origin", server.URL)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL), header)
	if err != nil {
		t.Fatalf("the deployment refused its own origin: %v", err)
	}
	_ = conn.Close()
}

// The entry answers the handshake failure and nothing else, and it answers it
// after the response has already gone out.
func TestAnOrdinaryRequestToASocketRouteIsRefused(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(echoSocket))
	t.Cleanup(server.Close)

	response, err := http.Get(server.URL + "/ws")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("a request carrying no upgrade tokens answered %d", response.StatusCode)
	}
	body, _ := io.ReadAll(response.Body)
	if !bytes.Contains(body, []byte(`"websocket_upgrade"`)) {
		t.Fatalf("the refusal is not a problem document: %q", body)
	}
}

// A writer that cannot hand over the connection is what a TinyGo net/http
// server gives a handler, and the whole reason the bootstrap serves through the
// upgrade-capable entry. Left unchecked it is a handshake that hangs with no
// error and no log line; here it is an answer.
func TestAWriterThatCannotHijackIsRefusedRatherThanHanging(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ws", nil)
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Sec-WebSocket-Version", "13")
	request.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	err := WebSocket(recorder, request, func(*Socket[socketIn, socketOut]) error {
		t.Error("the callback ran on a connection that was never handed over")
		return nil
	})
	if err == nil {
		t.Fatal("a writer that cannot hijack was accepted")
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("refused with %d, not 500", recorder.Code)
	}
	// The body says nothing specific: a server that cannot hijack is a
	// misconfiguration, and the name for it belongs in the log rather than in a
	// response to whoever asked.
	if !strings.Contains(err.Error(), "hand over the connection") {
		t.Fatalf("the returned error does not name the cause: %v", err)
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte("websocket_hijack")) {
		t.Fatalf("the response told the client what is misconfigured: %q", recorder.Body.String())
	}
}

func wsURL(base string) string { return "ws" + strings.TrimPrefix(base, "http") + "/ws" }
