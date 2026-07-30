package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type decodeFixture struct {
	Name string `json:"name"`
}

func TestDecodeJSONAcceptsOneStrictObject(t *testing.T) {
	t.Parallel()

	contentTypes := []string{
		"application/json",
		"application/json; charset=utf-8",
		"Application/JSON; Charset=UTF-8",
	}
	for _, contentType := range contentTypes {
		t.Run(contentType, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPost,
				"/api/test",
				strings.NewReader(" \n{\"name\":\"Settled\"}\t"),
			)
			request.Header.Set("Content-Type", contentType)
			response := httptest.NewRecorder()
			var destination decodeFixture

			if err := decodeJSON(response, request, &destination); err != nil {
				t.Fatalf("decodeJSON: %v", err)
			}
			if destination.Name != "Settled" {
				t.Errorf("Name = %q, want Settled", destination.Name)
			}
		})
	}
}

func TestDecodeJSONAllowsExactlyOneMiB(t *testing.T) {
	t.Parallel()

	prefix := `{"name":"`
	suffix := `"}`
	body := prefix +
		strings.Repeat("a", int(maxJSONBodyBytes)-len(prefix)-len(suffix)) +
		suffix
	if len(body) != int(maxJSONBodyBytes) {
		t.Fatalf("test body length = %d", len(body))
	}
	request := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	var destination decodeFixture

	if err := decodeJSON(response, request, &destination); err != nil {
		t.Fatalf("decodeJSON: %v", err)
	}
}

func TestDecodeJSONRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	oversizedPrefix := `{"name":"`
	oversizedSuffix := `"}`
	oversized := oversizedPrefix +
		strings.Repeat(
			"a",
			int(maxJSONBodyBytes)+1-len(oversizedPrefix)-len(oversizedSuffix),
		) +
		oversizedSuffix

	tests := []struct {
		name         string
		body         string
		contentTypes []string
	}{
		{name: "missing content type", body: `{}`},
		{
			name:         "wrong content type",
			body:         `{}`,
			contentTypes: []string{"text/plain"},
		},
		{
			name:         "malformed content type",
			body:         `{}`,
			contentTypes: []string{"application/json; charset"},
		},
		{
			name:         "multiple content types",
			body:         `{}`,
			contentTypes: []string{"application/json", "application/json"},
		},
		{
			name:         "empty",
			contentTypes: []string{"application/json"},
		},
		{
			name:         "whitespace",
			body:         " \n\t",
			contentTypes: []string{"application/json"},
		},
		{
			name:         "leading vertical tab",
			body:         "\v{}",
			contentTypes: []string{"application/json"},
		},
		{
			name:         "trailing vertical tab",
			body:         "{}\v",
			contentTypes: []string{"application/json"},
		},
		{
			name:         "leading form feed",
			body:         "\f{}",
			contentTypes: []string{"application/json"},
		},
		{
			name:         "trailing form feed",
			body:         "{}\f",
			contentTypes: []string{"application/json"},
		},
		{
			name:         "leading non-breaking space",
			body:         "\u00a0{}",
			contentTypes: []string{"application/json"},
		},
		{
			name:         "trailing non-breaking space",
			body:         "{}\u00a0",
			contentTypes: []string{"application/json"},
		},
		{
			name:         "invalid UTF-8 string value",
			body:         "{\"name\":\"" + string([]byte{0xff}) + "\"}",
			contentTypes: []string{"application/json"},
		},
		{
			name:         "invalid UTF-8 object key",
			body:         "{\"" + string([]byte{0xff}) + "\":\"value\"}",
			contentTypes: []string{"application/json"},
		},
		{
			name:         "null",
			body:         "null",
			contentTypes: []string{"application/json"},
		},
		{
			name:         "array",
			body:         `[]`,
			contentTypes: []string{"application/json"},
		},
		{
			name:         "string",
			body:         `"value"`,
			contentTypes: []string{"application/json"},
		},
		{
			name:         "unknown field",
			body:         `{"unknown":true}`,
			contentTypes: []string{"application/json"},
		},
		{
			name:         "malformed",
			body:         `{"name":`,
			contentTypes: []string{"application/json"},
		},
		{
			name:         "trailing text",
			body:         `{"name":"value"} trailing`,
			contentTypes: []string{"application/json"},
		},
		{
			name:         "second object",
			body:         `{"name":"one"} {"name":"two"}`,
			contentTypes: []string{"application/json"},
		},
		{
			name:         "oversized",
			body:         oversized,
			contentTypes: []string{"application/json"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPost,
				"/api/test",
				strings.NewReader(test.body),
			)
			for _, contentType := range test.contentTypes {
				request.Header.Add("Content-Type", contentType)
			}
			response := httptest.NewRecorder()

			err := decodeJSON(response, request, &decodeFixture{})

			if !errors.Is(err, ErrBadRequest) {
				t.Errorf("error = %v, want wrapped ErrBadRequest", err)
			}
		})
	}
}

func TestWriteJSONAndNoContent(t *testing.T) {
	t.Parallel()

	t.Run("JSON", func(t *testing.T) {
		response := httptest.NewRecorder()
		if err := writeJSON(
			response,
			http.StatusCreated,
			map[string]string{"status": "ok"},
		); err != nil {
			t.Fatalf("writeJSON: %v", err)
		}
		if response.Code != http.StatusCreated {
			t.Errorf("status = %d, want 201", response.Code)
		}
		if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Errorf("Content-Type = %q", got)
		}
		if got := response.Body.String(); got != "{\"status\":\"ok\"}\n" {
			t.Errorf("body = %q", got)
		}
	})

	t.Run("no content", func(t *testing.T) {
		response := httptest.NewRecorder()
		response.Header().Set("Content-Type", "application/json")
		if err := writeJSON(response, http.StatusNoContent, map[string]string{"ignored": "value"}); err != nil {
			t.Fatalf("writeJSON: %v", err)
		}
		if response.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", response.Code)
		}
		if response.Body.Len() != 0 {
			t.Errorf("body = %q, want empty", response.Body.String())
		}
		if got := response.Header().Get("Content-Type"); got != "" {
			t.Errorf("Content-Type = %q, want empty", got)
		}
	})
}
