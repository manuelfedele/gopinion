---
title: Errors
description: Return client-safe failures without leaking internal details.
sidebar:
  order: 5
---

Handlers return ordinary Go errors. GOpinion exposes only explicitly declared
`HTTPError` values to clients.

## Client-safe errors

```go
func getOrder(ctx gopinion.Context) (Order, error) {
    order, err := orders.Find(ctx.Request().Context(), ctx.PathValue("id"))
    if errors.Is(err, sql.ErrNoRows) {
        return Order{}, gopinion.NewHTTPError(
            http.StatusNotFound,
            "order_not_found",
            "The order does not exist.",
        )
    }
    return order, err
}
```

The response is stable and machine-readable:

```json
{
  "error": {
    "code": "order_not_found",
    "message": "The order does not exist."
  }
}
```

Use short, stable codes. Messages may change for clarity; client logic should
branch on `code`.

## Internal errors stay internal

```go
func getReport(ctx gopinion.Context) (Report, error) {
    return reports.Generate(ctx.Request().Context())
}
```

If the repository returns an ordinary error, GOpinion logs it and responds:

```json
{
  "error": {
    "code": "internal_error",
    "message": "The server could not process the request."
  }
}
```

Database details, stack data, and internal messages are not serialized.

## Supply structured logging

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

app, err := gopinion.New(
    gopinion.WithAuthenticator(authenticator),
    gopinion.WithAuthorizer(authorizer),
    gopinion.WithLogger(logger),
)
```

## Framework errors

GOpinion creates the same envelope for framework failures:

| Status | Code | Cause |
| --- | --- | --- |
| 400 | `invalid_json` | Malformed JSON, unknown struct fields, or multiple values |
| 400 | `invalid_query` | Malformed query-string encoding |
| 400 | `invalid_limit` | Invalid or duplicate limit |
| 400 | `limit_too_large` | Limit exceeds policy |
| 400 | `invalid_offset` | Invalid or duplicate offset |
| 400 | `offset_too_large` | Offset exceeds policy |
| 400 | `invalid_pagination` | Legacy pagination parameter |
| 401 | `unauthenticated` | Authentication failed |
| 403 | `forbidden` | Authorization denied |
| 404 | `not_found` | No matching route |
| 405 | `method_not_allowed` | Path exists for another method |
| 413 | `body_too_large` | Request body exceeds policy |
| 500 | `internal_error` | Handler, contract, encoding, or panic failure |

## Panics are contained

Panics from authenticators, handlers, or response marshaling are recovered at
the application boundary. Clients receive the generic `500` envelope and the
panic value is sent to the configured logger.
