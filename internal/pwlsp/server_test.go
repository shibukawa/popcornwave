package pwlsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

// session runs one connection from the messages given and returns everything
// the server wrote, decoded. Every test drives the server the way an editor
// does rather than calling a handler directly, because the protocol is the
// contract requirement:pw-language-server states.
func session(t *testing.T, messages ...any) ([]map[string]any, int) {
	t.Helper()
	var input bytes.Buffer
	for _, message := range messages {
		if err := writeMessage(&input, message); err != nil {
			t.Fatalf("framing the input: %v", err)
		}
	}
	var output bytes.Buffer
	code := NewServer(&output, Options{Version: "test"}).Serve(&input)

	var decoded []map[string]any
	reader := bufio.NewReader(&output)
	for {
		body, err := readMessage(reader)
		if err == io.EOF {
			return decoded, code
		}
		if err != nil {
			t.Fatalf("reading the output: %v", err)
		}
		var message map[string]any
		if err := json.Unmarshal(body, &message); err != nil {
			t.Fatalf("decoding the output: %v", err)
		}
		decoded = append(decoded, message)
	}
}

func request(id int, method string, params any) any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
}

func notify(method string, params any) any {
	return map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
}

func didOpen(uri, text string) any {
	return notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{"uri": uri, "languageId": "pw-sql", "version": 1, "text": text},
	})
}

// diagnosticsFor returns the diagnostics of the last publish for one uri, and
// whether any publish named it at all.
func diagnosticsFor(messages []map[string]any, uri string) ([]any, bool) {
	var found []any
	seen := false
	for _, message := range messages {
		if message["method"] != "textDocument/publishDiagnostics" {
			continue
		}
		params := message["params"].(map[string]any)
		if params["uri"] != uri {
			continue
		}
		seen = true
		found, _ = params["diagnostics"].([]any)
	}
	return found, seen
}

func resultOf(t *testing.T, messages []map[string]any, id float64) map[string]any {
	t.Helper()
	for _, message := range messages {
		if message["id"] == id {
			return message
		}
	}
	t.Fatalf("no reply to request %v", id)
	return nil
}

const cleanSQL = "package q\ntype R { id: int }\nexport statement F(id: int): sql.one<R> {\n  SELECT id FROM t WHERE id = {id}\n}\n"

func TestInitializeDeclaresOnlyWhatTheServerAnswers(t *testing.T) {
	messages, _ := session(t,
		request(1, "initialize", map[string]any{"rootUri": "file:///work"}),
		request(2, "shutdown", nil),
		notify("exit", nil),
	)

	capabilities := resultOf(t, messages, 1)["result"].(map[string]any)["capabilities"].(map[string]any)
	if capabilities["documentSymbolProvider"] != true {
		t.Fatalf("capabilities = %+v, want documentSymbol", capabilities)
	}
	sync := capabilities["textDocumentSync"].(map[string]any)
	if sync["change"].(float64) != syncFull {
		t.Fatalf("change = %v, want full sync", sync["change"])
	}
	// A capability the server does not serve must not be declared: a client
	// that believes it will wait for an answer that never comes.
	if _, declared := capabilities["documentHighlightProvider"]; declared {
		t.Fatal("documentHighlight is declared and not served")
	}
}

func TestShutdownAnswersWithAPresentNullResult(t *testing.T) {
	// The specification requires the result field, and a client checking for
	// its presence must not see a response that omitted it.
	var output bytes.Buffer
	var input bytes.Buffer
	_ = writeMessage(&input, request(1, "shutdown", nil))
	_ = writeMessage(&input, notify("exit", nil))
	NewServer(&output, Options{}).Serve(&input)

	body, err := readMessage(bufio.NewReader(&output))
	if err != nil {
		t.Fatalf("reading the reply: %v", err)
	}
	if string(body) != `{"jsonrpc":"2.0","id":1,"result":null}` {
		t.Fatalf("reply = %s", body)
	}
}

func TestOpeningACleanDocumentPublishesAnEmptyList(t *testing.T) {
	// Silence would leave a client's previous findings on screen, so an
	// empty list is the message rather than the absence of one.
	messages, _ := session(t,
		request(1, "initialize", nil),
		didOpen("file:///work/q.pw.sql", cleanSQL),
		notify("exit", nil),
	)

	diagnostics, published := diagnosticsFor(messages, "file:///work/q.pw.sql")
	if !published {
		t.Fatal("opening a document published nothing")
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v, want none", diagnostics)
	}
}

func TestABrokenDocumentIsReportedBeforeAnySave(t *testing.T) {
	messages, _ := session(t,
		request(1, "initialize", nil),
		didOpen("file:///work/x.pw.html", "export component X(): html {\n  <p>unclosed\n}\n"),
		notify("exit", nil),
	)

	diagnostics, _ := diagnosticsFor(messages, "file:///work/x.pw.html")
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v, want one", diagnostics)
	}
}

func TestAChangeRepublishesAndAFixClearsTheFinding(t *testing.T) {
	uri := "file:///work/q.pw.sql"
	messages, _ := session(t,
		request(1, "initialize", nil),
		didOpen(uri, "package q\ntype R { id: int\n"),
		notify("textDocument/didChange", map[string]any{
			"textDocument":   map[string]any{"uri": uri, "version": 2},
			"contentChanges": []any{map[string]any{"text": cleanSQL}},
		}),
		notify("exit", nil),
	)

	diagnostics, _ := diagnosticsFor(messages, uri)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics after the fix = %+v, want none", diagnostics)
	}
}

func TestClosingADocumentClearsWhatWasReportedAboutIt(t *testing.T) {
	uri := "file:///work/x.pw.html"
	messages, _ := session(t,
		request(1, "initialize", nil),
		didOpen(uri, "export component X(): html {\n  <p>unclosed\n}\n"),
		notify("textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": uri}}),
		notify("exit", nil),
	)

	diagnostics, _ := diagnosticsFor(messages, uri)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics after close = %+v, want none", diagnostics)
	}
}

func TestADocumentThatIsNotATemplateIsNotTracked(t *testing.T) {
	messages, _ := session(t,
		request(1, "initialize", nil),
		didOpen("file:///work/main.go", "package main\n"),
		notify("exit", nil),
	)

	if _, published := diagnosticsFor(messages, "file:///work/main.go"); published {
		t.Fatal("a Go file was published against; gopls owns it")
	}
}

func TestDocumentSymbolListsTheDeclarations(t *testing.T) {
	uri := "file:///work/q.pw.sql"
	messages, _ := session(t,
		request(1, "initialize", nil),
		didOpen(uri, cleanSQL),
		request(2, "textDocument/documentSymbol", map[string]any{"textDocument": map[string]any{"uri": uri}}),
		notify("exit", nil),
	)

	symbols := resultOf(t, messages, 2)["result"].([]any)
	if len(symbols) != 2 {
		t.Fatalf("symbols = %+v, want the type and the statement", symbols)
	}
	names := []string{symbols[0].(map[string]any)["name"].(string), symbols[1].(map[string]any)["name"].(string)}
	if names[0] != "R" || names[1] != "F" {
		t.Fatalf("names = %v, want [R F] in source order", names)
	}
	children := symbols[0].(map[string]any)["children"].([]any)
	if len(children) != 1 || children[0].(map[string]any)["name"] != "id" {
		t.Fatalf("children = %+v, want the one field", children)
	}
}

func TestDocumentSymbolOnAnUnopenedDocumentIsAnError(t *testing.T) {
	// Reading the file from disk instead would answer about something other
	// than the buffer, and policy:editor-tool-execution keeps this server off
	// files it was not sent.
	messages, _ := session(t,
		request(1, "initialize", nil),
		request(2, "textDocument/documentSymbol", map[string]any{"textDocument": map[string]any{"uri": "file:///work/absent.pw.sql"}}),
		notify("exit", nil),
	)

	reply := resultOf(t, messages, 2)
	if reply["error"] == nil {
		t.Fatalf("reply = %+v, want an error", reply)
	}
}

func TestAnUnservedRequestIsAnsweredRatherThanIgnored(t *testing.T) {
	messages, _ := session(t,
		request(1, "initialize", nil),
		request(2, "textDocument/documentHighlight", map[string]any{}),
		notify("exit", nil),
	)

	failed := resultOf(t, messages, 2)["error"].(map[string]any)
	if failed["code"].(float64) != codeMethodNotFound {
		t.Fatalf("error = %+v, want method not found", failed)
	}
}

func TestAnUnservedNotificationIsIgnoredSilently(t *testing.T) {
	// A notification has no reply, so answering one with an error would be a
	// message the client cannot correlate with anything.
	messages, _ := session(t,
		request(1, "initialize", nil),
		notify("$/setTrace", map[string]any{"value": "verbose"}),
		notify("exit", nil),
	)

	if len(messages) != 1 {
		t.Fatalf("messages = %+v, want only the initialize reply", messages)
	}
}

func TestExitAfterShutdownIsClean(t *testing.T) {
	_, code := session(t, request(1, "shutdown", nil), notify("exit", nil))

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestAStreamThatEndsWithNoShutdownIsNotClean(t *testing.T) {
	// The nonzero exit is what tells a client its restart policy applies,
	// which requirement:pw-language-server relies on.
	_, code := session(t, request(1, "initialize", nil))

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestUnparseableJSONIsReportedAndTheSessionContinues(t *testing.T) {
	var input bytes.Buffer
	_ = writeMessage(&input, request(1, "initialize", nil))
	body := []byte("{not json")
	input.WriteString("Content-Length: 9\r\n\r\n")
	input.Write(body)
	_ = writeMessage(&input, notify("exit", nil))

	var output bytes.Buffer
	code := NewServer(&output, Options{}).Serve(&input)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 for a session with no shutdown", code)
	}

	reader := bufio.NewReader(&output)
	var replies []map[string]any
	for {
		message, err := readMessage(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading the output: %v", err)
		}
		var decoded map[string]any
		_ = json.Unmarshal(message, &decoded)
		replies = append(replies, decoded)
	}
	if len(replies) != 2 || replies[1]["error"] == nil {
		t.Fatalf("replies = %+v, want the initialize result and a parse error", replies)
	}
}
