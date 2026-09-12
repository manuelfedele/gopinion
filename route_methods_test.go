package gopinion

import (
	"net/http"
	"strings"
	"testing"
)

func TestBodyRouteMethodsStrictlyDecodeAndReturnExpectedStatus(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}
	tests := []struct {
		name        string
		method      string
		status      int
		constructor func(string, func(Context, input) (string, error)) Route
	}{
		{name: "post", method: http.MethodPost, status: http.StatusCreated, constructor: Post[input, string]},
		{name: "put", method: http.MethodPut, status: http.StatusOK, constructor: Put[input, string]},
		{name: "patch", method: http.MethodPatch, status: http.StatusOK, constructor: Patch[input, string]},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newAuthenticatedTestApp(t, "version: 3\n")
			if err := app.Register(test.constructor("/resource", func(_ Context, input input) (string, error) {
				return input.Name, nil
			})); err != nil {
				t.Fatalf("Register() error = %v", err)
			}

			response := performRequest(app, test.method, "/resource", `{"name":"one"}`, "Bearer valid")
			if response.Code != test.status || response.Body.String() != "{\"data\":\"one\"}\n" {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}

			invalid := performRequest(app, test.method, "/resource", `{"unknown":true}`, "Bearer valid")
			if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), "invalid_json") {
				t.Fatalf("invalid response = %d %q", invalid.Code, invalid.Body.String())
			}
		})
	}
}

func TestDeleteReturnsNoContent(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	called := false
	if err := app.Register(Delete("/resource", func(Context) error {
		called = true
		return nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodDelete, "/resource", "", "Bearer valid")
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if !called {
		t.Fatal("delete handler was not called")
	}
}

func TestHeadRunsGetWithoutResponseBody(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	called := false
	if err := app.Register(Get("/resource", func(Context) (string, error) {
		called = true
		return "value", nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodHead, "/resource", "", "Bearer valid")
	if response.Code != http.StatusOK || response.Body.Len() != 0 {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if !called {
		t.Fatal("GET handler was not called for HEAD")
	}
}

func TestOptionsIsGeneratedFromRegisteredMethods(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	called := false
	if err := app.Register(Get("/resource", func(Context) (string, error) {
		called = true
		return "value", nil
	})); err != nil {
		t.Fatalf("Register(GET) error = %v", err)
	}
	if err := app.Register(Put("/resource", func(Context, struct{}) (string, error) {
		called = true
		return "value", nil
	})); err != nil {
		t.Fatalf("Register(PUT) error = %v", err)
	}
	if err := app.Register(Patch("/resource", func(Context, struct{}) (string, error) {
		called = true
		return "value", nil
	})); err != nil {
		t.Fatalf("Register(PATCH) error = %v", err)
	}
	if err := app.Register(Delete("/resource", func(Context) error {
		called = true
		return nil
	})); err != nil {
		t.Fatalf("Register(DELETE) error = %v", err)
	}

	response := performRequest(app, http.MethodOptions, "/resource", "", "Bearer valid")
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if response.Header().Get("Allow") != "DELETE, GET, HEAD, OPTIONS, PATCH, PUT" {
		t.Fatalf("Allow = %q", response.Header().Get("Allow"))
	}
	if called {
		t.Fatal("application handler was called for OPTIONS")
	}
}

func TestGeneralOptionsRemainsInsideAuthenticatedHandler(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	if !app.newServer().DisableGeneralOptionsHandler {
		t.Fatal("http.Server general OPTIONS handler is enabled")
	}

	response := performRequest(app, http.MethodOptions, "*", "", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("OPTIONS * status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("OPTIONS * security response header is missing")
	}
}

func TestOptionsRejectsServeMuxRedirectPaths(t *testing.T) {
	app := newAuthenticatedTestApp(t, "version: 3\n")
	if err := app.Register(Get("/tree/", func(Context) (string, error) {
		return "value", nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	for _, target := range []string{"/tree", "/tree/../tree/"} {
		response := performRequest(app, http.MethodOptions, target, "", "Bearer valid")
		if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
			t.Fatalf("OPTIONS response for %q = %d %q", target, response.Code, response.Body.String())
		}
		if response.Header().Get("Allow") != "" {
			t.Fatalf("Allow for %q = %q", target, response.Header().Get("Allow"))
		}
	}
}

func TestAuthorizedBodyAndListRoutesPrepareValidatedInput(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}
	app := newAuthorizedTestApp(t, allowAllAuthorizer())
	bodyPrepared := false
	if err := app.Register(AuthorizedPatch(
		"/resource",
		func(_ Context, input input) (AuthorizationPlan[string], error) {
			bodyPrepared = input.Name == "one"
			return testStringPlan(Context{})
		},
		func(_ Context, input input, prepared string) (string, error) {
			return input.Name + ":" + prepared, nil
		},
	)); err != nil {
		t.Fatalf("Register(PATCH) error = %v", err)
	}

	listPrepared := false
	if err := app.Register(AuthorizedList(
		"/resources",
		func(_ Context, request PageRequest) (AuthorizationPlan[string], error) {
			listPrepared = request.Offset == 1 && request.Limit == 1
			return testStringPlan(Context{})
		},
		func(_ Context, request PageRequest, _ string) (Page[string], error) {
			return NewPage([]string{"two"}, 2, request)
		},
	)); err != nil {
		t.Fatalf("Register(List) error = %v", err)
	}

	patchResponse := performRequest(app, http.MethodPatch, "/resource", `{"name":"one"}`, "Bearer valid")
	if patchResponse.Code != http.StatusOK || !bodyPrepared {
		t.Fatalf("PATCH response = %d %q, prepared = %t", patchResponse.Code, patchResponse.Body.String(), bodyPrepared)
	}
	listResponse := performRequest(app, http.MethodGet, "/resources?offset=1&limit=1", "", "Bearer valid")
	if listResponse.Code != http.StatusOK || !listPrepared {
		t.Fatalf("List response = %d %q, prepared = %t", listResponse.Code, listResponse.Body.String(), listPrepared)
	}
}

func TestAuthorizedConstructorsCoverMutationMethods(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}
	type bodyConstructor func(
		string,
		func(Context, input) (AuthorizationPlan[string], error),
		func(Context, input, string) (string, error),
	) Route
	tests := []struct {
		name        string
		method      string
		status      int
		constructor bodyConstructor
	}{
		{name: "post", method: http.MethodPost, status: http.StatusCreated, constructor: AuthorizedPost[input, string, string]},
		{name: "put", method: http.MethodPut, status: http.StatusOK, constructor: AuthorizedPut[input, string, string]},
		{name: "patch", method: http.MethodPatch, status: http.StatusOK, constructor: AuthorizedPatch[input, string, string]},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newAuthorizedTestApp(t, allowAllAuthorizer())
			if err := app.Register(test.constructor(
				"/resource",
				func(_ Context, input input) (AuthorizationPlan[string], error) {
					plan, err := testStringPlan(Context{})
					plan.Value = input.Name
					return plan, err
				},
				func(_ Context, _ input, prepared string) (string, error) {
					return prepared, nil
				},
			)); err != nil {
				t.Fatalf("Register() error = %v", err)
			}

			response := performRequest(app, test.method, "/resource", `{"name":"one"}`, "Bearer valid")
			if response.Code != test.status || response.Body.String() != "{\"data\":\"one\"}\n" {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}

	app := newAuthorizedTestApp(t, allowAllAuthorizer())
	if err := app.Register(AuthorizedDelete("/resource", testStringPlan, func(_ Context, prepared string) error {
		if prepared != "prepared" {
			t.Fatalf("prepared = %q", prepared)
		}
		return nil
	})); err != nil {
		t.Fatalf("Register(DELETE) error = %v", err)
	}
	response := performRequest(app, http.MethodDelete, "/resource", "", "Bearer valid")
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("DELETE response = %d %q", response.Code, response.Body.String())
	}
}
