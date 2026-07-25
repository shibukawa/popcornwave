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
	"strings"
)

// Renderer is implemented by generated, reflection-free HTML templates.
type Renderer interface {
	Render(io.Writer) error
}

// RenderFunc adapts a generated render function to Renderer.
type RenderFunc func(io.Writer) error

func (f RenderFunc) Render(w io.Writer) error { return f(w) }

// HTML writes a complete HTML response. The renderer is buffered so its error
// cannot leave a partially committed success response.
func HTML(w http.ResponseWriter, status int, renderer Renderer) error {
	if renderer == nil {
		return fmt.Errorf("petitweb: nil HTML renderer")
	}
	var body strings.Builder
	if err := renderer.Render(&body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(body.Len()))
	w.WriteHeader(status)
	_, err := io.WriteString(w, body.String())
	return err
}

// JSON writes a JSON response.
func JSON(w http.ResponseWriter, status int, value any) error {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(value); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(body.Len()))
	w.WriteHeader(status)
	_, err := body.WriteTo(w)
	return err
}

// XML writes an XML response.
func XML(w http.ResponseWriter, status int, value any) error {
	var body bytes.Buffer
	if err := xml.NewEncoder(&body).Encode(value); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Length", strconv.Itoa(body.Len()))
	w.WriteHeader(status)
	_, err := body.WriteTo(w)
	return err
}

// CSV writes records using the standard CSV encoder.
func CSV(w http.ResponseWriter, status int, records [][]string) error {
	var body bytes.Buffer
	writer := csv.NewWriter(&body)
	err := writer.WriteAll(records)
	if err == nil {
		err = writer.Error()
	}
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(body.Len()))
	w.WriteHeader(status)
	_, err = body.WriteTo(w)
	return err
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
