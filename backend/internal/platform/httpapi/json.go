package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var (
	ErrInvalidJSON  = errors.New("request body must contain one valid JSON object")
	ErrBodyTooLarge = errors.New("request body is too large")
)

type ErrorResponse struct {
	Error APIError `json:"error"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func DecodeJSON(reader io.Reader, maxBytes int64, destination any) error {
	payload, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return fmt.Errorf("read request body: %w", err)
	}
	if int64(len(payload)) > maxBytes {
		return ErrBodyTooLarge
	}

	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ErrInvalidJSON
	}
	return nil
}

func WriteJSON(response http.ResponseWriter, status int, payload any) error {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	return json.NewEncoder(response).Encode(payload)
}

func WriteError(response http.ResponseWriter, status int, code, message string) {
	_ = WriteJSON(response, status, ErrorResponse{
		Error: APIError{
			Code:    code,
			Message: message,
		},
	})
}
