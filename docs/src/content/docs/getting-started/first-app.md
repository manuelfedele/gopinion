---
title: Your First App
description: Run an authenticated, paginated GOpinion application.
sidebar:
  order: 2
---

This example exposes a single authenticated list endpoint.

## 1. Configure policy

```yaml title="gopinion.yaml"
version: 1

server:
  address: ":8080"

authentication:
  mode: required

pagination:
  mode: required
  default_size: 2
  maximum_size: 10
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

## 3. Define a paginated handler

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

func listTasks(_ gopinion.Context, request gopinion.PageRequest) (gopinion.Page[task], error) {
    start := request.Offset()
    if start > len(tasks) {
        start = len(tasks)
    }
    end := min(start+request.Size, len(tasks))
    return gopinion.NewPage(tasks[start:end], int64(len(tasks)), request)
}
```

`PageRequest` is validated before your handler runs. `NewPage` then verifies
that the returned item count fits the requested size and declared total.

## 4. Assemble and run

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
        "gopinion.yaml",
        gopinion.WithAuthenticator(tokenAuthenticator{tokenHash: sha256.Sum256([]byte(token))}),
    )
    if err != nil {
        log.Fatal(err)
    }

    if err := app.Register(gopinion.List("/tasks", listTasks)); err != nil {
        log.Fatal(err)
    }

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    defer stop()
    if err := app.Run(ctx); err != nil {
        log.Fatal(err)
    }
}
```

## 5. Call the endpoint

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
  'http://localhost:8080/tasks?page=1&page_size=2'
```

```json
{
  "data": [
    {"id": 1, "title": "Load policy"},
    {"id": 2, "title": "Authenticate requests"}
  ],
  "pagination": {
    "page": 1,
    "page_size": 2,
    "total_items": 3,
    "total_pages": 2
  }
}
```
