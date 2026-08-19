package pwlsp

// The LSP base protocol: JSON-RPC 2.0 messages framed by a Content-Length
// header on one stream pair.
//
// Framing and dispatch are written here rather than taken from a library
// because api:cli-lsp ships inside the pw binary: a protocol package would
// carry a transport set this server does not use and a generated type set far
// wider than requirement:pw-language-server answers, and every one of those
// types would still be reconciled against what the server actually sends.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// JSON-RPC error codes this server can produce. The parse and invalid-request
// codes come from JSON-RPC; the request-failed code is LSP's own.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
	codeRequestFailed  = -32803
)

// incoming is either a request, when ID is present, or a notification. The two
// are one struct because the wire format distinguishes them by that field
// alone, and mistaking one for the other is exactly the bug worth making hard.
type incoming struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (m incoming) isRequest() bool { return len(m.ID) > 0 }

// responseError is the error half of a response. Data is omitted rather than
// filled with a stack: policy:editor-tool-execution keeps diagnostics on the
// machine, and a client renders the message.
type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// result and failure are separate types because LSP requires exactly one of
// the two fields, and a single struct with omitempty would drop a null result
// that the specification requires to be present.
type result struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result"`
}

type failure struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   responseError   `json:"error"`
}

type notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

// errContentLength reports a header the reader cannot act on. It is fatal for
// the connection: with no length, the reader cannot find the next message.
type errContentLength struct{ reason string }

func (e *errContentLength) Error() string { return "lsp: " + e.reason }

// readMessage reads one framed message body. Headers other than Content-Length
// are read and ignored, which is what the base protocol asks of a reader that
// does not implement Content-Type negotiation.
func readMessage(reader *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && line == "" {
				return nil, io.EOF
			}
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, found := strings.Cut(line, ":")
		if !found {
			return nil, &errContentLength{reason: "malformed header " + strconv.Quote(line)}
		}
		if !strings.EqualFold(strings.TrimSpace(name), "content-length") {
			continue
		}
		length, err = strconv.Atoi(strings.TrimSpace(value))
		if err != nil || length < 0 {
			return nil, &errContentLength{reason: "unusable Content-Length " + strconv.Quote(value)}
		}
	}
	if length < 0 {
		return nil, &errContentLength{reason: "message with no Content-Length"}
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	return body, nil
}

// writeMessage frames one message. Every write to the output stream goes
// through here, so the header and the body can never disagree about a length.
func writeMessage(writer io.Writer, message any) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("lsp: encoding a message: %w", err)
	}
	if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = writer.Write(body)
	return err
}
