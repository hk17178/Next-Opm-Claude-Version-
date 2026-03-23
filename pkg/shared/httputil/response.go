// Package httputil provides HTTP response helpers for OpsNexus services.
package httputil

import (
	"encoding/json"
	"errors"
	"net/http"

	apperrors "github.com/opsnexus/opsnexus/pkg/errors"
)

// JSON writes a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// Error writes an error response. If the error is an AppError, uses its HTTP status and structure.
// Otherwise, returns a generic 500.
func Error(w http.ResponseWriter, err error) {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		JSON(w, appErr.HTTPStatus, appErr)
		return
	}
	JSON(w, http.StatusInternalServerError, map[string]string{
		"code":    "internal",
		"message": "internal server error",
	})
}

// NoContent writes a 204 No Content response.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Created writes a 201 Created response with the given data.
func Created(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusCreated, data)
}

// Accepted writes a 202 Accepted response with the given data.
func Accepted(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusAccepted, data)
}

// OK writes a 200 OK response with the given data.
func OK(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusOK, data)
}

// PageResponse represents a paginated response.
type PageResponse struct {
	Data          interface{} `json:"data"`
	NextPageToken string      `json:"next_page_token,omitempty"`
}
