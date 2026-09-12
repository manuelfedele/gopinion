---
title: Routing
description: Define typed GET, POST, and list routes without exposing the raw router.
sidebar:
  order: 2
---

GOpinion uses Go's standard `http.ServeMux` pattern syntax internally but owns
the mux and method registration.

## Singular GET

```go
type project struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

func getProject(ctx gopinion.Context) (project, error) {
    id := ctx.PathValue("id")
    return projects.Find(ctx.Request().Context(), id)
}

err := app.Register(gopinion.Get("/projects/{id}", getProject))
```

The successful response is wrapped:

```json
{"data":{"id":"p-17","name":"Policy engine"}}
```

## Strict POST

```go
type createProjectInput struct {
    Name string `json:"name"`
}

func createProject(ctx gopinion.Context, input createProjectInput) (project, error) {
    if strings.TrimSpace(input.Name) == "" {
        return project{}, gopinion.NewHTTPError(400, "name_required", "A project name is required.")
    }
    return projects.Create(ctx.Request().Context(), input.Name)
}

err := app.Register(gopinion.Post("/projects", createProject))
```

`Post` returns status `201`. Unknown fields in struct inputs and multiple JSON
values are rejected before the handler runs.

## Paginated GET

```go
func listProjects(
    ctx gopinion.Context,
    request gopinion.PageRequest,
) (gopinion.Page[project], error) {
    rows, total, err := projects.List(
        ctx.Request().Context(),
        request.Offset(),
        request.Size,
    )
    if err != nil {
        return gopinion.Page[project]{}, err
    }
    return gopinion.NewPage(rows, total, request)
}

err := app.Register(gopinion.List("/projects", listProjects))
```

## Route patterns

Standard wildcard patterns are supported:

```go
gopinion.Get("/users/{userID}", getUser)
gopinion.Get("/users/{userID}/orders/{orderID}", getOrder)
gopinion.Get("/assets/{path...}", getAsset)
gopinion.Get("/health/{$}", health)
```

Do not include an HTTP method in the pattern. The route constructor owns it.

## Registration rules

- Patterns must start with `/`.
- Nil handlers are rejected.
- Duplicate and conflicting routes are rejected.
- Routes cannot be registered after `Run` starts.
- Application code cannot implement its own `Route` type.
- The underlying `http.ServeMux` is not exposed.

Unknown paths use the fixed JSON `404` envelope. Method mismatches use the
fixed JSON `405` envelope and include an `Allow` header.
Paths that `http.ServeMux` would redirect for cleaning or a missing trailing
slash are returned as JSON `404` responses instead.
