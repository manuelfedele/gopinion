---
title: Complete Example
description: Run an authenticated, authorized, paginated todo API.
sidebar:
  order: 8
---

The complete runnable application is in
[`examples/todos`](https://github.com/manuelfedele/gopinion/tree/main/examples/todos).

## Policy

```yaml title="examples/todos/gopinion.yaml"
version: 2

server:
  address: ":8080"

authentication:
  mode: required

authorization:
  mode: required

pagination:
  mode: required
  default_size: 2
  maximum_size: 10
```

## Dependencies

The example injects a constant-time local token authenticator and a plain Go
authorizer:

```go
app, err := gopinion.New(
    "gopinion.yaml",
    gopinion.WithAuthenticator(exampleAuthenticator{
        tokenHash: sha256.Sum256([]byte(token)),
    }),
    gopinion.WithAuthorizer(exampleAuthorizer{}),
)
```

The static token and plaintext localhost endpoint are only for local use.

## Authorized routes

```go
app.Register(gopinion.AuthorizedList(
    "/todos",
    prepareListTodos,
    listTodos,
))

app.Register(gopinion.AuthorizedGet(
    "/todos/{id}",
    prepareGetTodo,
    getTodo,
))
```

`prepareGetTodo` loads the todo and places its trusted owner in the
authorization request. The handler receives the todo only after `Allow`.

The list preparation returns an owner scope. `listTodos` filters by that scope
before applying the page offset and computes totals from visible todos. A
production repository should apply that scope in its database query.

## Run it

```sh
cd examples/todos
GOPINION_EXAMPLE_TOKEN=change-me go run .
```

List the first page:

```sh
curl -H 'Authorization: Bearer change-me' \
  'http://localhost:8080/todos?page=1&page_size=2'
```

Read an owned item:

```sh
curl -H 'Authorization: Bearer change-me' \
  http://localhost:8080/todos/1
```

Try the enforced boundaries:

```sh
# Missing identity: 401
curl -i http://localhost:8080/todos

# Another user's item: 403
curl -i -H 'Authorization: Bearer change-me' \
  http://localhost:8080/todos/3

# Page above policy: 400
curl -i -H 'Authorization: Bearer change-me' \
  'http://localhost:8080/todos?page_size=50'
```
