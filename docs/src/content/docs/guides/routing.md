---
title: Routing
description: Define typed, authorized JSON routes without exposing the raw router.
sidebar:
  order: 3
---

GOpinion uses Go's standard `http.ServeMux` pattern syntax internally but owns
the mux, method registration, authorization ordering, and responses.

## Authorized GET

```go
func prepareProject(ctx gopinion.Context) (gopinion.AuthorizationPlan[project], error) {
    item, err := projects.Find(ctx.Request().Context(), ctx.PathValue("id"))
    if err != nil {
        return gopinion.AuthorizationPlan[project]{}, err
    }
    return projectReadPlan(item), nil
}

func getProject(_ gopinion.Context, item project) (project, error) {
    return item, nil
}

err := app.Register(gopinion.AuthorizedGet(
    "/projects/{id}",
    prepareProject,
    getProject,
))
```

Preparation loads trusted data. The handler receives it only after the
configured authorizer returns `Allow`.

## Body routes

`AuthorizedPost`, `AuthorizedPut`, and `AuthorizedPatch` decode strict JSON
before calling their preparation phase:

```go
err := app.Register(gopinion.AuthorizedPost(
    "/projects",
    prepareCreateProject,
    createProject,
))
```

The preparation function and handler both receive the typed input. POST returns
`201`; PUT and PATCH return `200`.

## Delete

```go
err := app.Register(gopinion.AuthorizedDelete(
    "/projects/{id}",
    prepareDeleteProject,
    deleteProject,
))
```

A successful delete returns `204` with no response body.

## Paginated GET

`AuthorizedList` validates pagination before preparation. Its prepared value
should carry the authorization scope used by the repository query:

```go
err := app.Register(gopinion.AuthorizedList(
    "/projects",
    prepareProjectList,
    listProjects,
))
```

## HEAD and OPTIONS

`HEAD` uses the matching GET route, including its preparation and authorization
phases, but returns no body. GOpinion generates `OPTIONS` for known paths. It is
authenticated, invokes no application handler, returns `204`, and sets `Allow`.

## Route patterns

Standard wildcard patterns are supported:

```go
gopinion.AuthorizedGet("/users/{userID}", prepareUser, getUser)
gopinion.AuthorizedGet("/users/{userID}/orders/{orderID}", prepareOrder, getOrder)
gopinion.AuthorizedGet("/assets/{path...}", prepareAsset, getAsset)
gopinion.AuthorizedGet("/health/{$}", prepareHealth, health)
```

Do not include an HTTP method in the pattern. The constructor owns it.

## Registration rules

- Patterns must start with `/`.
- Nil preparation functions and handlers are rejected.
- Required authorization rejects ordinary route constructors.
- Duplicate and conflicting routes are rejected.
- Routes cannot be registered after `Run` starts.
- Application code cannot implement its own `Route` type.
- The underlying `http.ServeMux` is not exposed.

Unknown paths use the fixed JSON `404` envelope. Method mismatches use the fixed
JSON `405` envelope and include an `Allow` header. Paths that `http.ServeMux`
would redirect are returned as JSON `404` responses instead.
