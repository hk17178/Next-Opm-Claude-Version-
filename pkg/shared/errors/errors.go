// Package errors provides standard error types and utilities for OpsNexus services.
package errors

import (
	"fmt"
	"net/http"
)

// Code represents a machine-readable error code.
type Code string

const (
	CodeValidation    Code = "VALIDATION_ERROR"
	CodeNotFound      Code = "NOT_FOUND"
	CodeConflict      Code = "CONFLICT"
	CodeUnauthorized  Code = "UNAUTHORIZED"
	CodeForbidden     Code = "FORBIDDEN"
	CodeRateLimited   Code = "RATE_LIMITED"
	CodeInternal      Code = "INTERNAL_ERROR"
	CodeBadGateway    Code = "BAD_GATEWAY"
	CodeUnavailable   Code = "SERVICE_UNAVAILABLE"
)

// AppError represents a structured application error.
type AppError struct {
	Code    Code        `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
	Err     error       `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// HTTPStatus returns the appropriate HTTP status code for this error.
func (e *AppError) HTTPStatus() int {
	switch e.Code {
	case CodeValidation:
		return http.StatusBadRequest
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeRateLimited:
		return http.StatusTooManyRequests
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	case CodeBadGateway:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

// New creates a new AppError.
func New(code Code, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

// Wrap creates a new AppError wrapping an existing error.
func Wrap(code Code, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

// WithDetails returns a copy of the error with additional details.
func (e *AppError) WithDetails(details interface{}) *AppError {
	return &AppError{
		Code:    e.Code,
		Message: e.Message,
		Details: details,
		Err:     e.Err,
	}
}

// Convenience constructors

func NotFound(resource string, id string) *AppError {
	return &AppError{
		Code:    CodeNotFound,
		Message: fmt.Sprintf("%s with id '%s' not found", resource, id),
	}
}

func ValidationFailed(message string) *AppError {
	return &AppError{
		Code:    CodeValidation,
		Message: message,
	}
}

func Unauthorized(message string) *AppError {
	return &AppError{
		Code:    CodeUnauthorized,
		Message: message,
	}
}

func Forbidden(message string) *AppError {
	return &AppError{
		Code:    CodeForbidden,
		Message: message,
	}
}

func Internal(message string, err error) *AppError {
	return &AppError{
		Code:    CodeInternal,
		Message: message,
		Err:     err,
	}
}
