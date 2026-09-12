---
title: Your First App
description: Run an authenticated, paginated GOpinion application.
sidebar:
  order: 2
---

This example exposes a single authenticated list endpoint.

## 1. Configure policy

```yaml title="gopinion.yaml"
version: 3

server:
  address: ":8080"

authentication:
  mode: required

authorization:
  mode: required

pagination:
  mode: required
  default_limit: 2
  maximum_limit: 10
  maximum_offset: 100
```

## 2. Supply an authenticator

An authenticator receives the original HTTP request and either establishes a
principal or returns an error.

```go
type tokenAuthenticator struct {
    tokenHash [sha256.Size]byte
}

func (a tokenAuthenticator) Authenticate(r *http.Request) (gopinion.Principal, error) {
    presented, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
    presentedHash := sha256.Sum256([]byte(presented))
    if !ok || subtle.ConstantTimeCompare(presentedHash[:], a.tokenHash[:]) != 1 {
        return gopinion.Principal{}, gopinion.ErrUnauthenticated
    }
    return gopinion.Principal{Subject: "first-user"}, nil
}
```

The static token is suitable only for this local example. Production services
should validate an OIDC token, session, or another trusted credential.

## 3. Supply an authorizer

```go
type taskAuthorizer struct{}

func (taskAuthorizer) Authorize(
    _ context.Context,
    principal gopinion.Principal,
    request gopinion.AuthorizationRequest,
) (gopinion.AuthorizationDecision, error) {
    if request.Action == "task:list" && principal.Subject != "" {
        return gopinion.Allow, nil
    }
    return gopinion.Deny, nil
}
```

Unknown actions deny by default.

## 4. Define preparation and a paginated handler

```go
type task struct {
    ID    int    `json:"id"`
    Title string `json:"title"`
}

var tasks = []task{
    {ID: 1, Title: "Load policy"},
    {ID: 2, Title: "Authenticate requests"},
    {ID: 3, Title: "Bound collections"},
}

func prepareListTasks(
    ctx gopinion.Context,
    _ gopinion.PageRequest,
) (gopinion.AuthorizationPlan[string], error) {
    return gopinion.AuthorizationPlan[string]{
        Request: gopinion.AuthorizationRequest{
            Action: "task:list",
            Resource: gopinion.AuthorizationResource{
                Type: "task_collection",
                ID: "tasks",
            },
        },
        Value: ctx.Principal().Subject,
    }, nil
}

func listTasks(_ gopinion.Context, request gopinion.PageRequest, _ string) (gopinion.Page[task], error) {
    start := request.Offset
    if start > len(tasks) {
        start = len(tasks)
    }
    end := min(start+request.Limit, len(tasks))
    return gopinion.NewPage(tasks[start:end], int64(len(tasks)), request)
}
```

`PageRequest` is validated before your handler runs. `NewPage` then verifies
that the returned item count fits the requested limit and declared total.

## 5. Assemble and run

```go title="main.go"
package main

import (
    "context"
    "crypto/sha256"
    "crypto/subtle"
    "log"
    "net/http"
    "os"
    "os/signal"
    "strings"

    "github.com/manuelfedele/gopinion"
)

func main() {
    token := os.Getenv("APP_TOKEN")
    if token == "" {
        log.Fatal("APP_TOKEN must be set")
    }

    app, err := gopinion.New(
        gopinion.WithAuthenticator(tokenAuthenticator{tokenHash: sha256.Sum256([]byte(token))}),
        gopinion.WithAuthorizer(taskAuthorizer{}),
    )
    if err != nil {
        log.Fatal(err)
    }

    if err := app.Register(gopinion.AuthorizedList("/tasks", prepareListTasks, listTasks)); err != nil {
        log.Fatal(err)
    }

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    defer stop()
    if err := app.Run(ctx); err != nil {
        log.Fatal(err)
    }
}
```

## 6. Call the endpoint

```sh
APP_TOKEN=local-secret go run .
```

Without a credential:

```sh
curl -i http://localhost:8080/tasks
```

```json
{
  "error": {
    "code": "unauthenticated",
    "message": "Authentication is required."
  }
}
```

With a credential:

```sh
curl -H 'Authorization: Bearer local-secret' \
  'http://localhost:8080/tasks?limit=2&offset=0'
```

```json
{
  "data": [
    {"id": 1, "title": "Load policy"},
    {"id": 2, "title": "Authenticate requests"}
  ],
  "pagination": {
    "limit": 2,
    "offset": 0,
    "totalItems": 3
  }
}
```
