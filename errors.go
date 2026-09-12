package gopinion

import "fmt"

// HTTPError is a client-safe error returned by a handler.
type HTTPError struct {
	Status  int
	Code    string
	Message string
}

// NewHTTPError creates an error that is safe to expose to a client.
func NewHTTPError(status int, code, message string) *HTTPError {
	return &HTTPError{Status: status, Code: code, Message: message}
}

// Error implements error.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
