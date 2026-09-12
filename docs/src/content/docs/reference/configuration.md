---
title: Configuration
description: Complete reference for the strict gopinion.yaml policy file.
sidebar:
  order: 1
---

GOpinion loads `gopinion.yaml` from the process working directory. It accepts
one YAML document and rejects unknown fields. `version` must be present and
non-null. Every other field has a default.

## Complete file

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

## Root fields

| Field | Required | Value |
| --- | --- | --- |
| `version` | Yes | Must be integer `3` |
| `server` | No | HTTP lifecycle and resource bounds |
| `authentication` | No | Global identity policy |
| `authorization` | No | Global access-control policy |
| `pagination` | No | Global collection policy |

Multiple YAML documents are rejected.

Version `3` replaces page-number pagination with `limit` and `offset`. Older
files are rejected so applications must migrate the pagination contract
explicitly.

## Server

| Field | Default | Validation |
| --- | --- | --- |
| `address` | `:8080` | Non-empty listen address |
| `read_header_timeout` | `5s` | Positive Go duration |
| `read_timeout` | `15s` | Positive Go duration |
| `write_timeout` | `30s` | Positive Go duration |
| `idle_timeout` | `60s` | Positive Go duration |
| `shutdown_timeout` | `10s` | Positive Go duration |
| `max_body_bytes` | `1048576` | Positive integer |

## Authentication

| Field | Default | Values |
| --- | --- | --- |
| `mode` | `required` | `required`, `disabled` |

`required` means an authenticator must be supplied to `gopinion.New` and every
request must establish a principal before routing.

## Authorization

| Field | Default | Values |
| --- | --- | --- |
| `mode` | `required` | `required`, `disabled` |

`required` means an authorizer must be supplied to `gopinion.New` and every
application route must use an authorized route constructor. Authorization
cannot be required when authentication is disabled.

Disabling authorization permits ordinary route constructors. An explicitly
authorized route still runs its authorization phase and requires an authorizer.

## Pagination

| Field | Default | Validation |
| --- | --- | --- |
| `mode` | `required` | `required`, `disabled` |
| `default_limit` | `25` | Greater than zero |
| `maximum_limit` | `100` | At least `default_limit` |
| `maximum_offset` | `10000` | Non-negative; sum with `maximum_limit` must fit `int` |

List routes may be used in either mode. `required` additionally prohibits
top-level collections from singular routes.

## Strict failure examples

Misspelled field:

```yaml
version: 3
authentcation:
  mode: disabled
```

```text
decode configuration: yaml: unmarshal errors:
  line 2: field authentcation not found in type gopinion.config
```

Unsupported policy:

```yaml
version: 3
authentication:
  mode: optional
```

```text
validate configuration: authentication.mode must be "required" or "disabled", got "optional"
```
