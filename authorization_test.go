package gopinion

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestNewFailsClosedWithoutAuthorizer(t *testing.T) {
	_, err := New(
		writeTestConfig(t, "version: 2\n"),
		WithAuthenticator(validTestAuthenticator()),
	)
	if err == nil || !strings.Contains(err.Error(), "no authorizer") {
		t.Fatalf("New() error = %v, want missing authorizer error", err)
	}
}

func TestWithAuthorizerRejectsNil(t *testing.T) {
	var authorizer *nilAuthorizer
	_, err := New(
		writeTestConfig(t, "version: 2\n"),
		WithAuthenticator(validTestAuthenticator()),
		WithAuthorizer(authorizer),
	)
	if err == nil || !strings.Contains(err.Error(), "authorizer must not be nil") {
		t.Fatalf("New() error = %v, want nil authorizer error", err)
	}
}

func TestRequiredAuthorizationRejectsUnguardedRoute(t *testing.T) {
	app := newAuthorizedTestApp(t, allowAllAuthorizer())
	err := app.Register(Get("/resource", func(Context) (string, error) {
		return "unguarded", nil
	}))
	if !errors.Is(err, ErrAuthorizationRequired) {
		t.Fatalf("Register() error = %v, want ErrAuthorizationRequired", err)
	}
}

func TestAuthorizedGetPassesPreparedValueAfterAllow(t *testing.T) {
	var receivedPrincipal Principal
	var receivedRequest AuthorizationRequest
	authorizer := AuthorizerFunc(func(_ context.Context, principal Principal, request AuthorizationRequest) (AuthorizationDecision, error) {
		receivedPrincipal = principal
		receivedRequest = request
		return Allow, nil
	})
	app := newAuthorizedTestApp(t, authorizer)

	err := app.Register(AuthorizedGet(
		"/resources/{id}",
		func(ctx Context) (AuthorizationPlan[string], error) {
			return AuthorizationPlan[string]{
				Request: AuthorizationRequest{
					Action: "resource:read",
					Resource: AuthorizationResource{
						Type:       "resource",
						ID:         ctx.PathValue("id"),
						Attributes: map[string]any{"owner_id": "user-123"},
					},
				},
				Value: "prepared",
			}, nil
		},
		func(_ Context, value string) (string, error) {
			return value, nil
		},
	))
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/resources/42", "", "Bearer valid")
	if response.Code != http.StatusOK || response.Body.String() != "{\"data\":\"prepared\"}\n" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if receivedPrincipal.Subject != "user-123" {
		t.Fatalf("principal = %+v", receivedPrincipal)
	}
	if receivedRequest.Action != "resource:read" || receivedRequest.Resource.ID != "42" {
		t.Fatalf("authorization request = %+v", receivedRequest)
	}
}

func TestAuthorizationDenialDoesNotInvokeHandler(t *testing.T) {
	app := newAuthorizedTestApp(t, AuthorizerFunc(func(context.Context, Principal, AuthorizationRequest) (AuthorizationDecision, error) {
		return Deny, nil
	}))
	called := false
	if err := app.Register(AuthorizedGet("/resource", testStringPlan, func(Context, string) (string, error) {
		called = true
		return "secret", nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/resource", "", "Bearer valid")
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), `"code":"forbidden"`) {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if called {
		t.Fatal("handler was called after authorization denial")
	}
}

func TestAuthenticationAndInputValidationPrecedeAuthorization(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}
	app := newAuthorizedTestApp(t, allowAllAuthorizer())
	prepared := false
	if err := app.Register(AuthorizedPost(
		"/resource",
		func(_ Context, _ input) (AuthorizationPlan[string], error) {
			prepared = true
			return testStringPlan(Context{})
		},
		func(Context, input, string) (string, error) {
			return "created", nil
		},
	)); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	unauthenticated := performRequest(app, http.MethodPost, "/resource", `{"name":"one"}`, "")
	if unauthenticated.Code != http.StatusUnauthorized || prepared {
		t.Fatalf("unauthenticated response = %d, prepared = %t", unauthenticated.Code, prepared)
	}

	invalid := performRequest(app, http.MethodPost, "/resource", `{"unknown":true}`, "Bearer valid")
	if invalid.Code != http.StatusBadRequest || prepared {
		t.Fatalf("invalid response = %d, prepared = %t", invalid.Code, prepared)
	}
}

func TestAuthorizerFailureIsInternalError(t *testing.T) {
	app := newAuthorizedTestApp(t, AuthorizerFunc(func(context.Context, Principal, AuthorizationRequest) (AuthorizationDecision, error) {
		return Deny, errors.New("policy store unavailable")
	}))
	if err := app.Register(AuthorizedGet("/resource", testStringPlan, func(Context, string) (string, error) {
		return "secret", nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/resource", "", "Bearer valid")
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "policy store") {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestAuthorizerErrForbiddenIsOperationalFailure(t *testing.T) {
	app := newAuthorizedTestApp(t, AuthorizerFunc(func(context.Context, Principal, AuthorizationRequest) (AuthorizationDecision, error) {
		return Deny, ErrForbidden
	}))
	if err := app.Register(AuthorizedGet("/resource", testStringPlan, func(Context, string) (string, error) {
		return "secret", nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/resource", "", "Bearer valid")
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestInvalidAuthorizationDecisionIsInternalError(t *testing.T) {
	app := newAuthorizedTestApp(t, AuthorizerFunc(func(context.Context, Principal, AuthorizationRequest) (AuthorizationDecision, error) {
		return AuthorizationDecision(99), nil
	}))
	if err := app.Register(AuthorizedGet("/resource", testStringPlan, func(Context, string) (string, error) {
		return "secret", nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/resource", "", "Bearer valid")
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestInvalidAuthorizationPlanFailsClosed(t *testing.T) {
	authorizerCalled := false
	app := newAuthorizedTestApp(t, AuthorizerFunc(func(context.Context, Principal, AuthorizationRequest) (AuthorizationDecision, error) {
		authorizerCalled = true
		return Allow, nil
	}))
	if err := app.Register(AuthorizedGet("/resource", func(Context) (AuthorizationPlan[string], error) {
		return AuthorizationPlan[string]{Value: "secret"}, nil
	}, func(Context, string) (string, error) {
		return "secret", nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	response := performRequest(app, http.MethodGet, "/resource", "", "Bearer valid")
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if authorizerCalled {
		t.Fatal("authorizer was called with an invalid plan")
	}
}

func TestAuthorizedRouteRejectsNilPreparation(t *testing.T) {
	app := newAuthorizedTestApp(t, allowAllAuthorizer())
	var prepare func(Context) (AuthorizationPlan[string], error)
	err := app.Register(AuthorizedGet("/resource", prepare, func(Context, string) (string, error) {
		return "ok", nil
	}))
	if err == nil || !strings.Contains(err.Error(), "authorization phase") {
		t.Fatalf("Register() error = %v, want nil authorization phase error", err)
	}
}

func TestAuthorizedRouteRequiresAuthorizerWhenPolicyDisabled(t *testing.T) {
	app, err := New(
		writeTestConfig(t, "version: 2\nauthorization:\n  mode: disabled\n"),
		WithAuthenticator(validTestAuthenticator()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	err = app.Register(AuthorizedGet("/resource", testStringPlan, func(Context, string) (string, error) {
		return "ok", nil
	}))
	if err == nil || !strings.Contains(err.Error(), "requires an authorizer") {
		t.Fatalf("Register() error = %v, want missing authorizer error", err)
	}
}

func testStringPlan(Context) (AuthorizationPlan[string], error) {
	return AuthorizationPlan[string]{
		Request: AuthorizationRequest{
			Action:   "resource:read",
			Resource: AuthorizationResource{Type: "resource", ID: "one"},
		},
		Value: "prepared",
	}, nil
}

func newAuthorizedTestApp(t *testing.T, authorizer Authorizer) *App {
	t.Helper()
	app, err := New(
		writeTestConfig(t, "version: 2\n"),
		WithAuthenticator(validTestAuthenticator()),
		WithAuthorizer(authorizer),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return app
}

func validTestAuthenticator() Authenticator {
	return AuthenticatorFunc(func(request *http.Request) (Principal, error) {
		if request.Header.Get("Authorization") != "Bearer valid" {
			return Principal{}, ErrUnauthenticated
		}
		return Principal{Subject: "user-123"}, nil
	})
}

func allowAllAuthorizer() Authorizer {
	return AuthorizerFunc(func(context.Context, Principal, AuthorizationRequest) (AuthorizationDecision, error) {
		return Allow, nil
	})
}

type nilAuthorizer struct{}

func (*nilAuthorizer) Authorize(context.Context, Principal, AuthorizationRequest) (AuthorizationDecision, error) {
	return Allow, nil
}
