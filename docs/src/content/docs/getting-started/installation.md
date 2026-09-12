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
version: 3
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

authorization:
  mode: required

pagination:
  mode: required
  default_limit: 25
  maximum_limit: 100
  maximum_offset: 10000
```

:::caution
Because authentication and authorization default to `required`, constructing
the application without both dependencies fails. This is intentional
fail-closed behavior.
:::

The policy does not generate credentials, validate JWTs, provide an
authorization engine, or configure TLS. Applications must inject production
implementations, and deployments must terminate TLS before traffic reaches
GOpinion's HTTP server.

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

`gopinion.yaml` must be in the process working directory. `gopinion.New` does
not accept a path or environment override.

## Next step

Build [your first authenticated application](../first-app/).
