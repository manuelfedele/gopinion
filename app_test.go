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
	_, err := New(writeTestConfig(t, "version: 1\n"))
	if err == nil || !strings.Contains(err.Error(), "no authenticator") {
		t.Fatalf("New() error = %v, want missing authenticator error", err)
	}
}

func TestAuthenticationWrapsEveryRoute(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 1\n")
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

	unknown := performRequest(app, http.MethodGet, "/unknown", "", "")
	if unknown.Code != http.StatusUnauthorized {
		t.Fatalf("unknown route status = %d, want %d", unknown.Code, http.StatusUnauthorized)
	}
}

func TestAuthenticatedPrincipalReachesHandler(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 1\n")
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
	app := newAuthenticatedTestApp(t, "version: 1\n")
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
	app := newAuthenticatedTestApp(t, "version: 1\n")
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
	if methodNotAllowed.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("Allow = %q, want %q", methodNotAllowed.Header().Get("Allow"), "GET, HEAD")
	}
}

func TestAuthenticationCanOnlyBeDisabledGlobally(t *testing.T) {
	app, err := New(writeTestConfig(t, "version: 1\nauthentication:\n  mode: disabled\n"))
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
	app := newAuthenticatedTestApp(t, "version: 1\npagination:\n  default_size: 2\n  maximum_size: 4\n")
	if err := app.Register(List("/widgets", func(_ Context, request PageRequest) (Page[string], error) {
		if request.Page != 1 || request.Size != 2 || request.Offset() != 0 {
			t.Fatalf("page request = %+v", request)
		}
		return NewPage([]string{"one", "two"}, 3, request)
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/widgets", "", "Bearer valid")
	want := "{\"data\":[\"one\",\"two\"],\"pagination\":{\"page\":1,\"page_size\":2,\"total_items\":3,\"total_pages\":2}}\n"
	if response.Code != http.StatusOK || response.Body.String() != want {
		t.Fatalf("response = %d %q, want %q", response.Code, response.Body.String(), want)
	}
}

func TestPaginationRejectsExcessivePageSize(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 1\npagination:\n  maximum_size: 30\n")
	called := false
	if err := app.Register(List("/widgets", func(_ Context, request PageRequest) (Page[string], error) {
		called = true
		return NewPage([]string{}, 0, request)
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/widgets?page_size=31", "", "Bearer valid")
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "page_size_too_large") {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if called {
		t.Fatal("handler was called with invalid pagination")
	}
}

func TestRequiredPaginationRejectsCollectionGet(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 1\n")
	err := app.Register(Get("/widgets", func(Context) ([]string, error) {
		return []string{"one"}, nil
	}))
	if !errors.Is(err, ErrPaginationRequired) {
		t.Fatalf("Register() error = %v, want ErrPaginationRequired", err)
	}
}

func TestRequiredPaginationRejectsDynamicCollection(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 1\n")
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
	content := "version: 1\nauthentication:\n  mode: disabled\npagination:\n  mode: disabled\n"
	app, err := New(writeTestConfig(t, content))
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
	app := newAuthenticatedTestApp(t, "version: 1\n")
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
	app := newAuthenticatedTestApp(t, "version: 1\n")
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
	app := newAuthenticatedTestApp(t, "version: 1\n")
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
	app := newAuthenticatedTestApp(t, "version: 1\n")
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
	app := newAuthenticatedTestApp(t, "version: 1\n")
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
	content := "version: 1\nserver:\n  max_body_bytes: 8\n"
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

func newAuthenticatedTestApp(t *testing.T, configuration string) *App {
	t.Helper()
	authenticator := AuthenticatorFunc(func(request *http.Request) (Principal, error) {
		if request.Header.Get("Authorization") != "Bearer valid" {
			return Principal{}, ErrUnauthenticated
		}
		return Principal{Subject: "user-123"}, nil
	})
	app, err := New(writeTestConfig(t, configuration), WithAuthenticator(authenticator))
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
