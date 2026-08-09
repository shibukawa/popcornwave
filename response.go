package petitweb

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"sync"
)

// Renderer is implemented by generated, reflection-free HTML templates.
type Renderer interface {
	Render(io.Writer) error
}

// RenderFunc adapts a generated render function to Renderer.
type RenderFunc func(io.Writer) error

func (f RenderFunc) Render(w io.Writer) error { return f(w) }

// bodyPool recycles the buffers the buffered response writers assemble a body
// in. An outlier beyond the cap is dropped rather than kept alive for the life
// of the pool.
var bodyPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

const maxPooledBodyBytes = 1 << 20

func getBody() *bytes.Buffer { return bodyPool.Get().(*bytes.Buffer) }

func putBody(body *bytes.Buffer) {
	if body.Cap() > maxPooledBodyBytes {
		return
	}
	body.Reset()
	bodyPool.Put(body)
}

// writeBody commits the buffered body with its declared type and length.
func writeBody(w http.ResponseWriter, status int, contentType string, body *bytes.Buffer) error {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(body.Len()))
	w.WriteHeader(status)
	_, err := body.WriteTo(w)
	return err
}

// HTML writes a complete HTML response. The renderer is buffered so its error
// cannot leave a partially committed success response.
func HTML(w http.ResponseWriter, status int, renderer Renderer) error {
	if renderer == nil {
		return fmt.Errorf("petitweb: nil HTML renderer")
	}
	body := getBody()
	defer putBody(body)
	if err := renderer.Render(body); err != nil {
		return err
	}
	return writeBody(w, status, "text/html; charset=utf-8", body)
}

// JSON writes a JSON response.
func JSON(w http.ResponseWriter, status int, value any) error {
	body := getBody()
	defer putBody(body)
	if err := json.NewEncoder(body).Encode(value); err != nil {
		return err
	}
	return writeBody(w, status, "application/json", body)
}

// XML writes an XML response.
func XML(w http.ResponseWriter, status int, value any) error {
	body := getBody()
	defer putBody(body)
	if err := xml.NewEncoder(body).Encode(value); err != nil {
		return err
	}
	return writeBody(w, status, "application/xml", body)
}

// CSV writes records using the standard CSV encoder.
func CSV(w http.ResponseWriter, status int, records [][]string) error {
	body := getBody()
	defer putBody(body)
	writer := csv.NewWriter(body)
	err := writer.WriteAll(records)
	if err == nil {
		err = writer.Error()
	}
	if err != nil {
		return err
	}
	return writeBody(w, status, "text/csv; charset=utf-8", body)
}

// Redirect performs a normal browser redirect.
func Redirect(w http.ResponseWriter, r *http.Request, location string, status int) {
	http.Redirect(w, r, location, status)
}

// Download writes bytes as an attachment using a safely encoded filename.
func Download(w http.ResponseWriter, status int, filename, contentType string, content []byte) error {
	if contentType == "" {
		contentType = http.DetectContentType(content)
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	w.WriteHeader(status)
	_, err := w.Write(content)
	return err
}
