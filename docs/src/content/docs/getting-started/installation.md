---
title: Installation
description: Install GOpinion and create its required policy file.
sidebar:
  order: 1
---

GOpinion requires Go 1.26.6 or newer.

## Add the module

From your application module:

```sh
go get github.com/manuelfedele/gopinion
```

Confirm the dependency:

```sh
go list -m github.com/manuelfedele/gopinion
```

## Create the policy file

Every application starts from an explicit `gopinion.yaml`:

```yaml title="gopinion.yaml"
version: 1
```

That minimal file enables fail-closed policy defaults. Omitted fields receive:

```yaml
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

:::caution
Because authentication defaults to `required`, constructing the application
without an authenticator fails. This is intentional fail-closed behavior.
:::

The policy does not generate credentials, validate JWTs, or configure TLS.
Applications must inject a production authenticator, and deployments must
terminate TLS before traffic reaches GOpinion's HTTP server.

## Suggested layout

```text
my-service/
├── cmd/api/main.go
├── internal/
│   ├── orders/
│   └── identity/
├── gopinion.yaml
├── go.mod
└── go.sum
```

GOpinion does not require this layout. The only structural requirement is that
your process can resolve the configuration path passed to `gopinion.New`.

## Next step

Build [your first authenticated application](../first-app/).
