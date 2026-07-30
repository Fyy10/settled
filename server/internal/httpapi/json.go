package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"unicode/utf8"
)

const maxJSONBodyBytes int64 = 1 << 20

func decodeJSON(w http.ResponseWriter, request *http.Request, destination any) error {
	contentTypes := request.Header.Values("Content-Type")
	if len(contentTypes) != 1 {
		return fmt.Errorf("%w: Content-Type must be application/json", ErrBadRequest)
	}
	contentType := contentTypes[0]
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		return fmt.Errorf("%w: Content-Type must be application/json", ErrBadRequest)
	}

	request.Body = http.MaxBytesReader(w, request.Body, maxJSONBodyBytes)
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return fmt.Errorf("%w: read JSON body", ErrBadRequest)
	}
	if !utf8.Valid(body) {
		return fmt.Errorf("%w: JSON body must be valid UTF-8", ErrBadRequest)
	}
	trimmed := trimJSONWhitespace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return fmt.Errorf("%w: body must be one JSON object", ErrBadRequest)
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("%w: decode JSON body", ErrBadRequest)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: body must contain one JSON value", ErrBadRequest)
	}
	return nil
}

func trimJSONWhitespace(value []byte) []byte {
	start := 0
	for start < len(value) && isJSONWhitespace(value[start]) {
		start++
	}
	end := len(value)
	for end > start && isJSONWhitespace(value[end-1]) {
		end--
	}
	return value[start:end]
}

func isJSONWhitespace(value byte) bool {
	switch value {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) error {
	if status == http.StatusNoContent {
		w.Header().Del("Content-Type")
		w.WriteHeader(status)
		return nil
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(value)
}

func writeNoContent(w http.ResponseWriter) {
	w.Header().Del("Content-Type")
	w.WriteHeader(http.StatusNoContent)
}
