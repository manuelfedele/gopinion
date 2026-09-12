---
title: Go API
description: Public types and constructors exposed by the GOpinion package.
sidebar:
  order: 2
---

Import the package:

```go
import "github.com/manuelfedele/gopinion"
```

## Application

```go
func New(configPath string, options ...Option) (*App, error)
func (app *App) Register(route Route) error
func (app *App) Run(ctx context.Context) error
```

`App` owns routing and serving. It does not expose `http.Handler` or
`http.ServeMux`.

## Options

```go
func WithAuthenticator(authenticator Authenticator) Option
func WithLogger(logger *slog.Logger) Option
```

Options inject dependencies. They do not change global policy.

## Route constructors

```go
func Get[O any](pattern string, handler func(Context) (O, error)) Route

func Post[I, O any](
    pattern string,
    handler func(Context, I) (O, error),
) Route

func List[O any](
    pattern string,
    handler func(Context, PageRequest) (Page[O], error),
) Route
```

`Get` returns `200`, `Post` returns `201`, and `List` returns `200`.

## Request context

```go
func (ctx Context) Request() *http.Request
func (ctx Context) Principal() Principal
func (ctx Context) PathValue(name string) string
```

The response writer is intentionally absent.

## Authentication

```go
type Authenticator interface {
    Authenticate(*http.Request) (Principal, error)
}

type AuthenticatorFunc func(*http.Request) (Principal, error)

type Principal struct {
    Subject string
    Claims  map[string]any
}

var ErrUnauthenticated error
```

A successful principal requires a non-empty `Subject`.

## Pagination

```go
type PageRequest struct {
    Page int
    Size int
}

func (request PageRequest) Offset() int

func NewPage[T any](
    items []T,
    total int64,
    request PageRequest,
) (Page[T], error)

func (page Page[T]) Items() []T
func (page Page[T]) TotalItems() int64
```

`Items` returns a copy. A `Page` zero value is invalid and cannot be serialized.

## Client-safe errors

```go
func NewHTTPError(status int, code, message string) *HTTPError

type HTTPError struct {
    Status  int
    Code    string
    Message string
}
```

Only statuses from `400` through `599` with non-empty code and message are
exposed. Invalid `HTTPError` values become generic internal errors.

## Sentinel errors

```go
var ErrPaginationRequired error
var ErrUnauthenticated error
```

Use `errors.Is` when checking `ErrPaginationRequired` returned from route
registration.
