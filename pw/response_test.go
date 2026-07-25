package pw

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	tinybind "github.com/shibukawa/tinybind-go"
)

func TestProblemConstructorsCoverTinyBindStatuses(t *testing.T) {
	for _, tc := range []struct {
		problem Problem
		status  int
		code    string
	}{
		{BadRequest(), http.StatusBadRequest, "bad_request"},
		{Unauthorized(), http.StatusUnauthorized, "unauthorized"},
		{Forbidden(), http.StatusForbidden, "forbidden"},
		{NotFound(), http.StatusNotFound, "not_found"},
		{Conflict(), http.StatusConflict, "conflict"},
		{PayloadTooLarge(), http.StatusRequestEntityTooLarge, "payload_too_large"},
		{InternalServerError(), http.StatusInternalServerError, "internal"},
		{Validation(), http.StatusBadRequest, "validation_failed"},
	} {
		if tc.problem.Status != tc.status || tc.problem.Code != tc.code {
			t.Errorf("problem = %d %q, want %d %q", tc.problem.Status, tc.problem.Code, tc.status, tc.code)
		}
	}
}

func TestWriteProblemIncludesValidationFields(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	WriteProblem(recorder, request, Validation(
		Field("name", "body", "is required"),
		Field("page", "query", "must be positive"),
	))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
	var payload struct {
		Code   string              `json:"code"`
		Errors []map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != "validation_failed" {
		t.Errorf("code = %q", payload.Code)
	}
	want := []map[string]string{
		{"field": "name", "location": "body", "message": "is required"},
		{"field": "page", "location": "query", "message": "must be positive"},
	}
	if !reflect.DeepEqual(payload.Errors, want) {
		t.Errorf("errors = %v, want %v", payload.Errors, want)
	}
}

func TestWriteProblemCarriesTinyBindFieldsAndHides5xxDetail(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	WriteProblem(recorder, request, tinybind.BindError("id", "path", "must be an integer"))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
	var bound struct {
		Errors []map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &bound); err != nil {
		t.Fatal(err)
	}
	if len(bound.Errors) != 1 || bound.Errors[0]["field"] != "id" {
		t.Fatalf("errors = %v", bound.Errors)
	}

	recorder = httptest.NewRecorder()
	problem := InternalServerError(errors.New("dsn=secret"))
	problem.Fields = []FieldError{Field("id", "path", "leaked")}
	WriteProblem(recorder, request, problem)
	var internal struct {
		Detail string              `json:"detail"`
		Errors []map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &internal); err != nil {
		t.Fatal(err)
	}
	if internal.Detail != "internal error" || internal.Errors != nil {
		t.Fatalf("5xx response leaked details: %q", recorder.Body.String())
	}
}
