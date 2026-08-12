//go:build !fasthttp

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/tinygodriver/websocket"
)

// dial opens a socket against a server running this package's handler, and
// leaves the room clean for the next test.
func dial(t *testing.T, server *httptest.Server, header http.Header) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	return conn
}

func send(t *testing.T, conn *websocket.Conn, message ClientMsg) {
	t.Helper()
	payload, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// waitFor reads until the named kind arrives, skipping the presence traffic a
// room generates on its own. A test that counted messages instead would be
// asserting the order of other people's arrivals.
func waitFor(t *testing.T, conn *websocket.Conn, kind string) ServerMsg {
	t.Helper()
	for range 8 {
		if message := receive(t, conn); message.Type == kind {
			return message
		}
	}
	t.Fatalf("no %q arrived", kind)
	return ServerMsg{}
}

func receive(t *testing.T, conn *websocket.Conn) ServerMsg {
	t.Helper()
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var message ServerMsg
	if err := json.Unmarshal(data, &message); err != nil {
		t.Fatalf("the server sent something this protocol does not describe: %q", data)
	}
	return message
}

func chatServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(chatSocket))
	t.Cleanup(server.Close)
	t.Cleanup(func() {
		chat.mu.Lock()
		chat.members = map[member]string{}
		chat.mu.Unlock()
	})
	return server
}

// What a reader should take from this example: two structs went in and two
// structs came out, over a real handshake, with nothing in the handler touching
// bytes.
func TestJoiningTheRoomIsAnswered(t *testing.T) {
	conn := dial(t, chatServer(t), nil)

	send(t, conn, ClientMsg{Type: "join", Name: "ada"})
	welcome := waitFor(t, conn, "welcome")
	if welcome.Text != "ada" {
		t.Fatalf("welcome = %+v", welcome)
	}
	if welcome.Live != 1 {
		t.Fatalf("the room reports %d members", welcome.Live)
	}
}

// The room is the application's, and this is what it is for: one member's
// message reaches the other, written from the sender's goroutine.
func TestAMessageReachesTheOtherMember(t *testing.T) {
	server := chatServer(t)
	ada, grace := dial(t, server, nil), dial(t, server, nil)

	send(t, ada, ClientMsg{Type: "join", Name: "ada"})
	waitFor(t, ada, "welcome")
	send(t, grace, ClientMsg{Type: "join", Name: "grace"})
	waitFor(t, grace, "welcome")

	send(t, ada, ClientMsg{Type: "say", Text: "hello"})
	said := waitFor(t, grace, "message")
	if said.From != "ada" || said.Text != "hello" {
		t.Fatalf("grace received %+v", said)
	}
}

// A protocol error is the application's to answer, so it comes back as a
// message rather than as a closed connection.
func TestSpeakingBeforeJoiningIsRefusedWithoutClosing(t *testing.T) {
	conn := dial(t, chatServer(t), nil)

	send(t, conn, ClientMsg{Type: "say", Text: "hello"})
	if refusal := waitFor(t, conn, "error"); refusal.Code != "join_first" {
		t.Fatalf("refusal = %+v", refusal)
	}

	// Still open: the socket answered and kept the connection.
	send(t, conn, ClientMsg{Type: "join", Name: "ada"})
	if welcome := waitFor(t, conn, "welcome"); welcome.Text != "ada" {
		t.Fatalf("the socket did not survive its own protocol error: %+v", welcome)
	}
}

// The upgrade carries the session cookie and never meets the CSRF middleware,
// so this is the check standing between the room and another site.
func TestAnUpgradeFromAnotherOriginIsRefused(t *testing.T) {
	server := chatServer(t)
	header := http.Header{}
	header.Set("Origin", "https://evil.example")

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, response, err := websocket.DefaultDialer.Dial(url, header)
	if err == nil {
		_ = conn.Close()
		t.Fatal("a socket opened from another origin")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("refused with %v", response)
	}
	_ = response.Body.Close()
}
