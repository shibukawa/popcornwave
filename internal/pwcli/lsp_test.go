package pwcli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// A command missing from the help table is one nobody discovers, and an
// editor client is configured by hand from what pw help says exists.
func TestLSPIsRegisteredInTheCommandList(t *testing.T) {
	var usage bytes.Buffer
	printUsage(&usage)

	found := false
	for _, command := range commandSummaries {
		if command.name == "lsp" {
			found = command.summary != ""
		}
	}
	if !found {
		t.Fatal("lsp is missing from commandSummaries, so pw help omits it")
	}
	if !strings.Contains(usage.String(), lspUsage) {
		t.Fatal("pw help does not print the lsp usage line")
	}

	var out, errOut bytes.Buffer
	if status := Main([]string{"lsp", "--nonsense"}, &out, &errOut); status == 0 {
		t.Fatal("the dispatcher did not reach lsp: a bad argument exited 0")
	}
}

func TestLSPOptionsAcceptTheDocumentedFlags(t *testing.T) {
	options, err := parseLSPOptions([]string{"--stdio", "--log=/tmp/pw.log", "--root=/work"})
	if err != nil {
		t.Fatalf("parseLSPOptions: %v", err)
	}
	if options.Log != "/tmp/pw.log" || options.Root != "/work" {
		t.Fatalf("options = %+v", options)
	}
}

func TestLSPRejectsAnUnknownFlag(t *testing.T) {
	// A client passing a flag this build does not have is misconfigured, and
	// starting anyway would look like it worked.
	if _, err := parseLSPOptions([]string{"--socket=7000"}); err == nil {
		t.Fatal("an unknown flag was accepted")
	}
}

func TestLSPServesOneSessionOnTheStreams(t *testing.T) {
	var input bytes.Buffer
	frame(&input, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	frame(&input, `{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///w/x.pw.html","languageId":"pw-html","version":1,"text":"export component X(): html {\n  <p>unclosed\n}\n"}}}`)
	frame(&input, `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`)
	frame(&input, `{"jsonrpc":"2.0","method":"exit"}`)

	var output bytes.Buffer
	if err := runLSP([]string{"--stdio"}, &input, &output); err != nil {
		t.Fatalf("runLSP: %v", err)
	}

	published := false
	for _, message := range unframe(t, &output) {
		if message["method"] == "textDocument/publishDiagnostics" {
			published = true
		}
	}
	if !published {
		t.Fatal("the session published no diagnostics for a broken document")
	}
}

func TestLSPWritesTheTraceToTheLogAndNotToTheProtocolStream(t *testing.T) {
	// Anything but a framed message on stdout ends the session, so the log
	// flag exists precisely to keep tracing off it.
	log := filepath.Join(t.TempDir(), "pw.log")
	var input bytes.Buffer
	frame(&input, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	frame(&input, `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`)
	frame(&input, `{"jsonrpc":"2.0","method":"exit"}`)

	var output bytes.Buffer
	if err := runLSP([]string{"--log=" + log}, &input, &output); err != nil {
		t.Fatalf("runLSP: %v", err)
	}

	written, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("reading the log: %v", err)
	}
	if !strings.Contains(string(written), "initialize") {
		t.Fatalf("log = %q, want the traced methods", written)
	}
	for _, message := range unframe(t, &output) {
		if message["jsonrpc"] != "2.0" {
			t.Fatalf("a non-protocol message reached stdout: %+v", message)
		}
	}
}

func TestLSPReportsAClientThatLeftWithoutShuttingDown(t *testing.T) {
	var input bytes.Buffer
	frame(&input, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)

	err := runLSP(nil, &input, io.Discard)
	if err == nil {
		t.Fatal("a disconnect with no shutdown was reported as success")
	}
}

func frame(buffer *bytes.Buffer, body string) {
	buffer.WriteString("Content-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + body)
}

func unframe(t *testing.T, buffer *bytes.Buffer) []map[string]any {
	t.Helper()
	reader := bufio.NewReader(buffer)
	var messages []map[string]any
	for {
		header, err := reader.ReadString('\n')
		if err == io.EOF {
			return messages
		}
		if err != nil {
			t.Fatalf("reading a header: %v", err)
		}
		length, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "Content-Length:")))
		if err != nil {
			t.Fatalf("unusable header %q", header)
		}
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("reading the header separator: %v", err)
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(reader, body); err != nil {
			t.Fatalf("reading a body: %v", err)
		}
		var message map[string]any
		if err := json.Unmarshal(body, &message); err != nil {
			t.Fatalf("decoding a body: %v", err)
		}
		messages = append(messages, message)
	}
}
