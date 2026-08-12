package pwfast

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinybind-go/jsonbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
	"github.com/shibukawa/tinygodriver/websocket"
)

// The same two halves the net/http test declares, and deliberately not shared
// with it: what is being checked is that one callback body behaves the same on
// two transports, which a shared fixture would assume rather than show.
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

// echoSocket is the net/http handler's body with the qualifier moved, which is
// the whole claim the second build makes.
func echoSocket(r *fasthttp.RequestCtx) {
	_ = WebSocket(r, func(socket *Socket[socketIn, socketOut]) error {
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

// socketServer serves handler on a real port, because a WebSocket handshake is
// a connection this test has to dial rather than a request it can synthesize.
func socketServer(t *testing.T, handler fasthttp.RequestHandler) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &fasthttp.Server{Handler: handler}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Shutdown() })
	return "ws://" + listener.Addr().String() + "/ws"
}

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

// The callback runs after the handler returned on this transport, which is the
// difference the shape exists to hide. A round trip is what shows it hidden.
func TestASocketCarriesTypedMessagesBothWays(t *testing.T) {
	url := socketServer(t, echoSocket)

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
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

// One deployment answers one way whichever runtime serves it, which is why the
// origin check is not each transport's own.
func TestAnUpgradeFromAnotherOriginIsRefused(t *testing.T) {
	publishedSettings(t, pwruntime.ChainSettings{})
	url := socketServer(t, echoSocket)

	header := http.Header{}
	header.Set("Origin", "https://evil.example")
	conn, response, err := websocket.DefaultDialer.Dial(url, header)
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

func TestAnUpgradeFromThisOriginIsAdmitted(t *testing.T) {
	publishedSettings(t, pwruntime.ChainSettings{})
	url := socketServer(t, echoSocket)

	header := http.Header{}
	header.Set("Origin", "http"+strings.TrimPrefix(strings.TrimSuffix(url, "/ws"), "ws"))
	conn, _, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		t.Fatalf("the deployment refused its own origin: %v", err)
	}
	_ = conn.Close()
}
