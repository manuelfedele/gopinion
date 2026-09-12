---
title: Authentication
description: Establish a principal before any GOpinion route is selected.
sidebar:
  order: 1
---

Authentication is required by default and executes before routing. A request
with no valid identity receives `401` without invoking an application handler.

## Implement the interface

```go
type Authenticator interface {
    Authenticate(*http.Request) (Principal, error)
}
```

A principal must have a non-empty `Subject`:

```go
type apiKeyAuthenticator struct {
    keys map[string]string // key to subject
}

func (a apiKeyAuthenticator) Authenticate(r *http.Request) (gopinion.Principal, error) {
    key := r.Header.Get("X-API-Key")
    if key == "" {
        return gopinion.Principal{}, gopinion.ErrUnauthenticated
    }
    subject, found := a.keys[key]
    if !found {
        return gopinion.Principal{}, gopinion.ErrUnauthenticated
    }
    return gopinion.Principal{
        Subject: subject,
        Claims: map[string]any{"credential": "api-key"},
    }, nil
}

func (apiKeyAuthenticator) AuthenticationChallenge() string {
    return `ApiKey realm="api"`
}
```

Register it at application construction:

```go
app, err := gopinion.New(
    "gopinion.yaml",
    gopinion.WithAuthenticator(apiKeyAuthenticator{keys: keys}),
    gopinion.WithAuthorizer(authorizer),
)
```

## Use a function directly

`AuthenticatorFunc` avoids a dedicated type for small integrations:

```go
authenticator := gopinion.AuthenticatorFunc(func(r *http.Request) (gopinion.Principal, error) {
    session, err := sessions.Resolve(r.Context(), r.Header.Get("Authorization"))
    if err != nil {
        return gopinion.Principal{}, gopinion.ErrUnauthenticated
    }
    return gopinion.Principal{
        Subject: session.UserID,
        Claims:  map[string]any{"roles": session.Roles},
    }, nil
})
```

## OIDC integration

GOpinion does not choose an OIDC library. Wrap the verifier selected by your
organization:

```go
type oidcAuthenticator struct {
    verifier TokenVerifier
}

func (a oidcAuthenticator) Authenticate(r *http.Request) (gopinion.Principal, error) {
    raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
    if !ok {
        return gopinion.Principal{}, gopinion.ErrUnauthenticated
    }

    token, err := a.verifier.Verify(r.Context(), raw)
    if err != nil {
        if isInvalidToken(err) {
            return gopinion.Principal{}, gopinion.ErrUnauthenticated
        }
        return gopinion.Principal{}, fmt.Errorf("verify OIDC token: %w", err)
    }
    return gopinion.Principal{
        Subject: token.Subject,
        Claims:  token.Claims,
    }, nil
}
```

The verifier should validate signature, issuer, audience, expiry, and any
organization-specific claims.

GOpinion does not generate keys, parse JWTs, or select signing algorithms.
Those guarantees belong to the injected verifier. Authentication failures
should return `gopinion.ErrUnauthenticated`; operational verifier failures
should return their original error and produce a generic `500` response.

Unauthenticated responses use `WWW-Authenticate: Bearer` by default. An
authenticator can implement `AuthenticationChallenger` to provide another
challenge, as the API-key example does.

## Read the principal

Every authenticated handler receives the established identity:

```go
func currentUser(ctx gopinion.Context) (UserView, error) {
    subject := ctx.Principal().Subject
    user, err := users.Find(ctx.Request().Context(), subject)
    if errors.Is(err, sql.ErrNoRows) {
        return UserView{}, gopinion.NewHTTPError(404, "user_not_found", "The user does not exist.")
    }
    if err != nil {
        return UserView{}, err
    }
    return UserView{ID: user.ID, Name: user.Name}, nil
}
```

## Authorization follows authentication

Authentication establishes who is calling. Authorized route preparation then
loads trusted resource facts, and the configured authorizer decides whether the
handler may run. See the [authorization guide](../authorization/).

## Disable authentication

Only the global policy can disable authentication:

```yaml
authentication:
  mode: disabled

authorization:
  mode: disabled
```

There is no endpoint-level equivalent.
