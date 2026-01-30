package api

import "fmt"

// APIError represents an error returned by the API.
type APIError struct {
	StatusCode int
	Message    string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("API error: status %d", e.StatusCode)
}

// IsNotFound returns true if the error is a 404 Not Found.
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == 404
}

// IsUnauthorized returns true if the error is a 401 Unauthorized.
func (e *APIError) IsUnauthorized() bool {
	return e.StatusCode == 401
}

// IsForbidden returns true if the error is a 403 Forbidden.
func (e *APIError) IsForbidden() bool {
	return e.StatusCode == 403
}

// IsConflict returns true if the error is a 409 Conflict.
func (e *APIError) IsConflict() bool {
	return e.StatusCode == 409
}
