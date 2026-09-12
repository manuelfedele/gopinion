# GOpinion

[![CI](https://github.com/manuelfedele/gopinion/actions/workflows/ci.yml/badge.svg)](https://github.com/manuelfedele/gopinion/actions/workflows/ci.yml)
[![Documentation](https://github.com/manuelfedele/gopinion/actions/workflows/docs.yml/badge.svg)](https://github.com/manuelfedele/gopinion/actions/workflows/docs.yml)

GOpinion is the fail-closed Go framework for JSON HTTP applications. Secure and
bounded behavior is the default. Applications opt out globally in one strict
configuration file, not endpoint by endpoint.

> GOpinion is experimental. Its API is not yet stable.

Read the complete documentation at
[manuelfedele.github.io/gopinion](https://manuelfedele.github.io/gopinion/).

## Enforced Policies

- Authentication defaults to `required` and wraps every framework route.
- There is no route-level API for bypassing required authentication.
- Collection responses must use typed paginated list routes by default.
- Request bodies use strict JSON decoding and a configurable size limit.
- Configuration rejects unknown fields and unsupported values at startup.
- The framework owns route registration, the HTTP server, and graceful shutdown.
- Errors and responses use fixed JSON envelopes.

## Configuration

`gopinion.yaml` is the single source of truth for application policy:

```yaml
version: 1

server:
  address: ":8080"
  read_header_timeout: 5s
  read_timeout: 15s
  write_timeout: 30s
  idle_timeout: 60s
  shutdown_timeout: 10s
  max_body_bytes: 1048576

authentication:
  mode: required

pagination:
  mode: required
  default_size: 25
  maximum_size: 100
```

All fields except `version` can be omitted to use these defaults. Supported
policy modes are `required` and `disabled`.

Authentication implementations are injected as code because credentials and
identity-provider clients are runtime dependencies, not application policy.
When authentication is required, startup fails unless an authenticator is
supplied.

## Application

```go
app, err := gopinion.New(
    "gopinion.yaml",
    gopinion.WithAuthenticator(authenticator),
)
if err != nil {
    return err
}

err = app.Register(gopinion.List(
    "/orders",
    func(ctx gopinion.Context, request gopinion.PageRequest) (gopinion.Page[Order], error) {
        orders, total, err := repository.List(ctx.Request().Context(), request.Offset(), request.Size)
        if err != nil {
            return gopinion.Page[Order]{}, err
        }
        return gopinion.NewPage(orders, total, request)
    },
))
if err != nil {
    return err
}

return app.Run(ctx)
```

`Get` and `Post` define singular routes. `List` requires a handler returning
`Page[T]`, and `Page[T]` can only be validly constructed through `NewPage`.

## Example

Run the authenticated in-memory example:

```sh
cd examples/todos
GOPINION_EXAMPLE_TOKEN=change-me go run .
```

Then query it:

```sh
curl -H 'Authorization: Bearer change-me' \
  'http://localhost:8080/todos?page=1&page_size=2'
```

## Guarantee Boundary

GOpinion guarantees policy enforcement for endpoints registered with GOpinion.
Go cannot prevent application code from starting a separate `net/http` server.
Repositories requiring enforcement against deliberate bypass should prohibit
additional listeners in CI and enforce authentication again at the ingress or
gateway.

## Development

```sh
gofmt -w .
go vet ./...
go test -race ./...
```

GOpinion requires Go 1.24 or newer and is licensed under Apache-2.0.
