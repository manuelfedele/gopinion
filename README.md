# GOpinion

[![CI](https://github.com/manuelfedele/gopinion/actions/workflows/ci.yml/badge.svg)](https://github.com/manuelfedele/gopinion/actions/workflows/ci.yml)
[![Documentation](https://github.com/manuelfedele/gopinion/actions/workflows/docs.yml/badge.svg)](https://github.com/manuelfedele/gopinion/actions/workflows/docs.yml)

GOpinion is a fail-closed Go framework for JSON HTTP applications.
Authentication, authorization, and bounded behavior are required by default.
Applications opt out globally in one strict configuration file, not endpoint by
endpoint.

> GOpinion is experimental. Its API is not yet stable.

Read the complete documentation at
[manuelfedele.github.io/gopinion](https://manuelfedele.github.io/gopinion/).

## Enforced Policies

- Authentication defaults to `required` and wraps every framework route.
- There is no route-level API for bypassing required authentication.
- Authorization defaults to `required`; startup requires an authorizer and every
  registered application route must declare a typed authorization phase.
- Collection responses must use typed paginated list routes by default.
- Request bodies use strict JSON decoding and a configurable size limit.
- Configuration rejects unknown fields and unsupported values at startup.
- The framework owns route registration, the HTTP server, and graceful shutdown.
- Errors and responses use fixed JSON envelopes.

## Configuration

`gopinion.yaml` is the single source of truth for application policy:

```yaml
version: 3

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

authorization:
  mode: required

pagination:
  mode: required
  default_limit: 25
  maximum_limit: 100
  maximum_offset: 10000
```

All fields except `version` can be omitted to use these defaults. Supported
policy modes are `required` and `disabled`. `gopinion.New` always loads
`gopinion.yaml` from the process working directory.

Authentication and authorization implementations are injected as code because
identity-provider and policy-engine clients are runtime dependencies, not
application policy. Startup fails when either required dependency is absent.

GOpinion does not generate credentials or implement JWT validation. Production
authenticators must validate credentials with an appropriate identity provider,
and deployments must terminate TLS before requests reach the HTTP server.

## Application

```go
app, err := gopinion.New(
    gopinion.WithAuthenticator(authenticator),
    gopinion.WithAuthorizer(authorizer),
)
if err != nil {
    return err
}

err = app.Register(gopinion.AuthorizedList(
    "/orders",
    func(ctx gopinion.Context, request gopinion.PageRequest) (gopinion.AuthorizationPlan[OrderScope], error) {
        return orderListAuthorization(ctx.Principal()), nil
    },
    func(ctx gopinion.Context, request gopinion.PageRequest, scope OrderScope) (gopinion.Page[Order], error) {
        orders, total, err := repository.List(ctx.Request().Context(), scope, request.Offset, request.Limit)
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

`Get`, `Post`, `Put`, and `Patch` define singular JSON routes. `Delete` returns
`204`. `List` requires a handler returning `Page[T]`. Their `Authorized...`
variants prepare typed domain data and invoke the configured authorizer before
the handler. List routes use bounded `limit`/`offset` requests, camelCase
metadata, and RFC 5988 `Link` relations.

## Example

Run the authenticated and authorized in-memory example:

The static token and plaintext localhost endpoint are for local use only.

```sh
cd examples/todos
GOPINION_EXAMPLE_TOKEN=change-me go run .
```

Then query it:

```sh
curl -H 'Authorization: Bearer change-me' \
  'http://localhost:8080/todos?limit=2&offset=0'
```

## Guarantee Boundary

GOpinion guarantees policy enforcement for endpoints registered with GOpinion.
Go cannot prevent application code from starting a separate `net/http` server.
Repositories requiring enforcement against deliberate bypass should prohibit
additional listeners in CI and enforce identity and access policy again at the
ingress or gateway.

## Development

```sh
gofmt -w .
go vet ./...
go test -race ./...
```

GOpinion requires Go 1.26.6 or newer and is licensed under Apache-2.0.
