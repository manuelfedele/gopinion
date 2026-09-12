package gopinion

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewFailsClosedWithoutAuthenticator(t *testing.T) {
	_, err := newApp(writeTestConfig(t, "version: 3\n"))
	if err == nil || !strings.Contains(err.Error(), "no authenticator") {
		t.Fatalf("New() error = %v, want missing authenticator error", err)
	}
}

func TestAuthenticationWrapsEveryRoute(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	called := false
	if err := app.Register(Get("/resource", func(Context) (string, error) {
		called = true
		return "secret", nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/resource", "", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if called {
		t.Fatal("handler was called before authentication")
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security response header is missing")
	}
	if response.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q, want Bearer", response.Header().Get("WWW-Authenticate"))
	}

	unknown := performRequest(app, http.MethodGet, "/unknown", "", "")
	if unknown.Code != http.StatusUnauthorized {
		t.Fatalf("unknown route status = %d, want %d", unknown.Code, http.StatusUnauthorized)
	}
}

func TestAuthenticatorFailureIsInternalError(t *testing.T) {
	app, err := newApp(
		writeTestConfig(t, "version: 3\nauthorization:\n  mode: disabled\n"),
		WithAuthenticator(AuthenticatorFunc(func(*http.Request) (Principal, error) {
			return Principal{}, errors.New("identity provider unavailable")
		})),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/resource", "", "Bearer token")
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestAuthenticatorCanCustomizeChallenge(t *testing.T) {
	app, err := newApp(writeTestConfig(t, "version: 3\nauthorization:\n  mode: disabled\n"), WithAuthenticator(challengingAuthenticator{}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/resource", "", "")
	if response.Code != http.StatusUnauthorized || response.Header().Get("WWW-Authenticate") != `ApiKey realm="api"` {
		t.Fatalf("response = %d, WWW-Authenticate = %q", response.Code, response.Header().Get("WWW-Authenticate"))
	}
}

func TestAuthenticatedPrincipalReachesHandler(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	if err := app.Register(Get("/me", func(context Context) (string, error) {
		return context.Principal().Subject, nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/me", "", "Bearer valid")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if response.Body.String() != "{\"data\":\"user-123\"}\n" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestPathValueReachesHandler(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	if err := app.Register(Get("/resources/{id}", func(context Context) (string, error) {
		return context.PathValue("id"), nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/resources/42", "", "Bearer valid")
	if response.Code != http.StatusOK || response.Body.String() != "{\"data\":\"42\"}\n" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestAuthenticatedNotFoundAndMethodNotAllowedUseErrorEnvelope(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	if err := app.Register(Get("/resource", func(Context) (string, error) {
		return "ok", nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	notFound := performRequest(app, http.MethodGet, "/missing", "", "Bearer valid")
	if notFound.Code != http.StatusNotFound || !strings.Contains(notFound.Body.String(), `"code":"not_found"`) {
		t.Fatalf("not found response = %d %q", notFound.Code, notFound.Body.String())
	}

	methodNotAllowed := performRequest(app, http.MethodPost, "/resource", "", "Bearer valid")
	if methodNotAllowed.Code != http.StatusMethodNotAllowed || !strings.Contains(methodNotAllowed.Body.String(), `"code":"method_not_allowed"`) {
		t.Fatalf("method response = %d %q", methodNotAllowed.Code, methodNotAllowed.Body.String())
	}
	if methodNotAllowed.Header().Get("Allow") != "GET, HEAD, OPTIONS" {
		t.Fatalf("Allow = %q, want %q", methodNotAllowed.Header().Get("Allow"), "GET, HEAD, OPTIONS")
	}
}

func TestServeMuxRedirectsUseNotFoundEnvelope(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	if err := app.Register(Get("/tree/", func(Context) (string, error) {
		return "ok", nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	for _, target := range []string{"/tree", "/tree/../tree/"} {
		response := performRequest(app, http.MethodGet, target, "", "Bearer valid")
		if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
			t.Fatalf("response for %q = %d %q", target, response.Code, response.Body.String())
		}
		if response.Header().Get("Location") != "" {
			t.Fatalf("Location for %q = %q", target, response.Header().Get("Location"))
		}
	}
}

func TestAuthenticationCanOnlyBeDisabledGlobally(t *testing.T) {
	app, err := newApp(writeTestConfig(t, "version: 3\nauthentication:\n  mode: disabled\nauthorization:\n  mode: disabled\n"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := app.Register(Get("/public", func(context Context) (bool, error) {
		return context.Principal().Subject == "", nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/public", "", "")
	if response.Code != http.StatusOK || response.Body.String() != "{\"data\":true}\n" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestPaginationDefaultsAndEnvelope(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\npagination:\n  default_limit: 2\n  maximum_limit: 4\n")
	if err := app.Register(List("/widgets", func(_ Context, request PageRequest) (Page[string], error) {
		if request.Limit != 2 || request.Offset != 0 {
			t.Fatalf("page request = %+v", request)
		}
		return NewPage([]string{"one", "two"}, 3, request)
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/widgets", "", "Bearer valid")
	want := "{\"data\":[\"one\",\"two\"],\"pagination\":{\"limit\":2,\"offset\":0,\"totalItems\":3}}\n"
	if response.Code != http.StatusOK || response.Body.String() != want {
		t.Fatalf("response = %d %q, want %q", response.Code, response.Body.String(), want)
	}
	wantLink := `</widgets?limit=2&offset=0>; rel="first", </widgets?limit=2&offset=2>; rel="next", </widgets?limit=2&offset=2>; rel="last"`
	if response.Header().Get("Link") != wantLink {
		t.Fatalf("Link = %q, want %q", response.Header().Get("Link"), wantLink)
	}
	head := performRequest(app, http.MethodHead, "/widgets", "", "Bearer valid")
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Link") != wantLink {
		t.Fatalf("HEAD response = %d %q, Link = %q", head.Code, head.Body.String(), head.Header().Get("Link"))
	}
}

func TestPaginationRejectsExcessiveLimit(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\npagination:\n  maximum_limit: 30\n")
	called := false
	if err := app.Register(List("/widgets", func(_ Context, request PageRequest) (Page[string], error) {
		called = true
		return NewPage([]string{}, 0, request)
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/widgets?limit=31", "", "Bearer valid")
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "limit_too_large") {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if called {
		t.Fatal("handler was called with invalid pagination")
	}
}

func TestPaginationRejectsMalformedAndDuplicateParameters(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	if err := app.Register(List("/widgets", func(_ Context, request PageRequest) (Page[string], error) {
		return NewPage([]string{}, 0, request)
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	tests := []struct {
		target string
		code   string
	}{
		{target: "/widgets?limit=1;offset=2", code: "invalid_query"},
		{target: "/widgets?limit=1&limit=2", code: "invalid_limit"},
		{target: "/widgets?offset=1&offset=2", code: "invalid_offset"},
		{target: "/widgets?limit=0", code: "invalid_limit"},
		{target: "/widgets?limit=", code: "invalid_limit"},
		{target: "/widgets?offset=-1", code: "invalid_offset"},
		{target: "/widgets?offset=", code: "invalid_offset"},
		{target: "/widgets?page=1", code: "invalid_pagination"},
		{target: "/widgets?page_size=10", code: "invalid_pagination"},
	}
	for _, test := range tests {
		response := performRequest(app, http.MethodGet, test.target, "", "Bearer valid")
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) {
			t.Fatalf("response for %q = %d %q", test.target, response.Code, response.Body.String())
		}
	}

	empty := performRequest(app, http.MethodGet, "/widgets", "", "Bearer valid")
	if empty.Code != http.StatusOK || empty.Header().Get("Link") != "" {
		t.Fatalf("empty response = %d, Link = %q", empty.Code, empty.Header().Get("Link"))
	}
	emptyHead := performRequest(app, http.MethodHead, "/widgets?limit=", "", "Bearer valid")
	if emptyHead.Code != http.StatusBadRequest {
		t.Fatalf("HEAD empty limit response = %d %q", emptyHead.Code, emptyHead.Body.String())
	}
}

func TestPaginationRejectsExcessiveOffsetBeforeHandler(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\npagination:\n  maximum_offset: 10\n")
	called := false
	if err := app.Register(List("/widgets", func(_ Context, request PageRequest) (Page[string], error) {
		called = true
		return NewPage([]string{}, 0, request)
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/widgets?offset=11", "", "Bearer valid")
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"offset_too_large"`) {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if called {
		t.Fatal("handler was called with excessive offset")
	}

	boundary := performRequest(app, http.MethodGet, "/widgets?offset=10", "", "Bearer valid")
	if boundary.Code != http.StatusOK || !called {
		t.Fatalf("boundary response = %d, handler called = %t", boundary.Code, called)
	}
}

func TestPaginationRejectsPageForDifferentRequest(t *testing.T) {
	tests := []struct {
		name   string
		change func(PageRequest) PageRequest
	}{
		{name: "limit", change: func(request PageRequest) PageRequest { request.Limit++; return request }},
		{name: "offset", change: func(request PageRequest) PageRequest { request.Offset++; return request }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newAuthenticatedTestApp(t, "version: 3\n")
			if err := app.Register(List("/widgets", func(_ Context, request PageRequest) (Page[string], error) {
				return NewPage([]string{}, 0, test.change(request))
			})); err != nil {
				t.Fatalf("Register() error = %v", err)
			}

			response := performRequest(app, http.MethodGet, "/widgets?limit=2&offset=0", "", "Bearer valid")
			if response.Code != http.StatusInternalServerError || response.Header().Get("Link") != "" {
				t.Fatalf("response = %d, Link = %q", response.Code, response.Header().Get("Link"))
			}
		})
	}
}

func TestPaginationLinkHeaderPreservesQueryAndUsesRegisteredRelations(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	if err := app.Register(List("/widgets", func(_ Context, request PageRequest) (Page[string], error) {
		return NewPage([]string{"three", "four"}, 5, request)
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/widgets?status=open&offset=2&limit=2", "", "Bearer valid")
	want := `</widgets?limit=2&offset=0&status=open>; rel="first", </widgets?limit=2&offset=0&status=open>; rel="prev", </widgets?limit=2&offset=4&status=open>; rel="next", </widgets?limit=2&offset=4&status=open>; rel="last"`
	if response.Code != http.StatusOK || response.Header().Get("Link") != want {
		t.Fatalf("response = %d, Link = %q, want %q", response.Code, response.Header().Get("Link"), want)
	}
}

func TestPaginationLinkHeaderOmitsOffsetsAbovePolicyMaximum(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\npagination:\n  default_limit: 2\n  maximum_limit: 2\n  maximum_offset: 2\n")
	if err := app.Register(List("/widgets", func(_ Context, request PageRequest) (Page[string], error) {
		return NewPage([]string{"three", "four"}, 10, request)
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/widgets?offset=2", "", "Bearer valid")
	want := `</widgets?limit=2&offset=0>; rel="first", </widgets?limit=2&offset=0>; rel="prev"`
	if response.Code != http.StatusOK || response.Header().Get("Link") != want {
		t.Fatalf("response = %d, Link = %q, want %q", response.Code, response.Header().Get("Link"), want)
	}
}

func TestRequiredPaginationRejectsCollectionGet(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	err := app.Register(Get("/widgets", func(Context) ([]string, error) {
		return []string{"one"}, nil
	}))
	if !errors.Is(err, ErrPaginationRequired) {
		t.Fatalf("Register() error = %v, want ErrPaginationRequired", err)
	}
}

func TestRequiredPaginationRejectsDynamicCollection(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	if err := app.Register(Get("/widgets", func(Context) (any, error) {
		return []string{"one"}, nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/widgets", "", "Bearer valid")
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "internal_error") {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestDisabledPaginationAllowsCollectionGet(t *testing.T) {
	content := "version: 3\nauthentication:\n  mode: disabled\nauthorization:\n  mode: disabled\npagination:\n  mode: disabled\n"
	app, err := newApp(writeTestConfig(t, content))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := app.Register(Get("/widgets", func(Context) ([]string, error) {
		return []string{"one"}, nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/widgets", "", "")
	if response.Code != http.StatusOK || response.Body.String() != "{\"data\":[\"one\"]}\n" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestPostStrictlyDecodesJSON(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}
	app := newAuthenticatedTestApp(t, "version: 3\n")
	if err := app.Register(Post("/widgets", func(_ Context, input input) (string, error) {
		return input.Name, nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodPost, "/widgets", `{"name":"one","unknown":true}`, "Bearer valid")
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid_json") {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestHandlerHTTPErrorIsExposed(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	if err := app.Register(Get("/missing", func(Context) (string, error) {
		return "", NewHTTPError(http.StatusNotFound, "not_found", "The resource does not exist.")
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/missing", "", "Bearer valid")
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), "not_found") {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestResponseEncodingFailureDoesNotSendSuccess(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	if err := app.Register(Get("/broken", func(Context) (struct{ Value any }, error) {
		return struct{ Value any }{Value: make(chan int)}, nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/broken", "", "Bearer valid")
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "internal_error") {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestHandlerPanicIsContained(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	if err := app.Register(Get("/panic", func(Context) (string, error) {
		panic("boom")
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/panic", "", "Bearer valid")
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "internal_error") {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestRegisterRejectsNilHandlerAndDuplicateRoute(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	var handler func(Context) (string, error)
	if err := app.Register(Get("/nil", handler)); err == nil || !strings.Contains(err.Error(), "must not be nil") {
		t.Fatalf("Register(nil handler) error = %v", err)
	}

	route := Get("/duplicate", func(Context) (string, error) { return "ok", nil })
	if err := app.Register(route); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	if err := app.Register(route); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("second Register() error = %v", err)
	}
}

func TestPostRejectsOversizedBody(t *testing.T) {
	content := "version: 3\nserver:\n  max_body_bytes: 8\n"
	app := newAuthenticatedTestApp(t, content)
	if err := app.Register(Post("/widgets", func(_ Context, input map[string]string) (string, error) {
		return input["name"], nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodPost, "/widgets", `{"name":"too long"}`, "Bearer valid")
	if response.Code != http.StatusRequestEntityTooLarge || !strings.Contains(response.Body.String(), "body_too_large") {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestPostRejectsOversizedTrailingData(t *testing.T) {
	content := "version: 3\nserver:\n  max_body_bytes: 3\n"
	app := newAuthenticatedTestApp(t, content)
	if err := app.Register(Post("/widgets", func(_ Context, input map[string]string) (string, error) {
		return input["name"], nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodPost, "/widgets", "{}  ", "Bearer valid")
	if response.Code != http.StatusRequestEntityTooLarge || !strings.Contains(response.Body.String(), "body_too_large") {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestRunRejectsNilContextBeforeStarting(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	//lint:ignore SA1012 Verify the public API rejects an invalid context safely.
	if err := app.Run(nil); err == nil || !strings.Contains(err.Error(), "context") {
		t.Fatalf("Run(nil) error = %v", err)
	}
	if err := app.Register(Get("/still-configurable", func(Context) (string, error) {
		return "ok", nil
	})); err != nil {
		t.Fatalf("Register() after Run(nil) error = %v", err)
	}
}

func newAuthenticatedTestApp(t *testing.T, configuration string) *App {
	t.Helper()
	configuration += "authorization:\n  mode: disabled\n"
	authenticator := AuthenticatorFunc(func(request *http.Request) (Principal, error) {
		if request.Header.Get("Authorization") != "Bearer valid" {
			return Principal{}, ErrUnauthenticated
		}
		return Principal{Subject: "user-123"}, nil
	})
	app, err := newApp(writeTestConfig(t, configuration), WithAuthenticator(authenticator))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return app
}

func performRequest(app *App, method, target, body, authorization string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	response := httptest.NewRecorder()
	app.serveHTTP(response, request)
	return response
}

type challengingAuthenticator struct{}

func (challengingAuthenticator) Authenticate(*http.Request) (Principal, error) {
	return Principal{}, ErrUnauthenticated
}

func (challengingAuthenticator) AuthenticationChallenge() string {
	return `ApiKey realm="api"`
}
