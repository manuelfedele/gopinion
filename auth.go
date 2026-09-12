package gopinion

import (
	"errors"
	"net/http"
)

// ErrUnauthenticated indicates that a request has no valid identity.
var ErrUnauthenticated = errors.New("request is not authenticated")

// Principal is the identity established for an authenticated request.
type Principal struct {
	Subject string
	Claims  map[string]any
}

// Authenticator establishes the principal for an HTTP request.
type Authenticator interface {
	Authenticate(*http.Request) (Principal, error)
}

// AuthenticationChallenger optionally provides the WWW-Authenticate value
// returned when authentication fails. Bearer is used by default.
type AuthenticationChallenger interface {
	AuthenticationChallenge() string
}

// AuthenticatorFunc adapts a function to Authenticator.
type AuthenticatorFunc func(*http.Request) (Principal, error)

// Authenticate calls f with the incoming request.
func (f AuthenticatorFunc) Authenticate(request *http.Request) (Principal, error) {
	return f(request)
}
