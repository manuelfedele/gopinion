package gopinion

import "net/http"

// Context carries the request and the identity established by the framework.
type Context struct {
	request    *http.Request
	principal  Principal
	pagination paginationConfig
}

// Request returns the underlying request. The framework retains ownership of
// routing and response writing.
func (c Context) Request() *http.Request {
	return c.request
}

// Principal returns the authenticated identity. It is empty only when global
// authentication is disabled.
func (c Context) Principal() Principal {
	return c.principal
}

// PathValue returns a named wildcard captured by the route pattern.
func (c Context) PathValue(name string) string {
	return c.request.PathValue(name)
}
