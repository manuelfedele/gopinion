---
title: Configuration
description: Complete reference for the strict gopinion.yaml policy file.
sidebar:
  order: 1
---

GOpinion loads one YAML document and rejects unknown fields. `version` must be
present and non-null. Every other field has a default.

## Complete file

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

## Root fields

| Field | Required | Value |
| --- | --- | --- |
| `version` | Yes | Must be integer `1` |
| `server` | No | HTTP lifecycle and resource bounds |
| `authentication` | No | Global identity policy |
| `pagination` | No | Global collection policy |

Multiple YAML documents are rejected.

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

## Pagination

| Field | Default | Validation |
| --- | --- | --- |
| `mode` | `required` | `required`, `disabled` |
| `default_size` | `25` | Greater than zero |
| `maximum_size` | `100` | At least `default_size` |

List routes may be used in either mode. `required` additionally prohibits
top-level collections from singular routes.

## Strict failure examples

Misspelled field:

```yaml
version: 1
authentcation:
  mode: disabled
```

```text
decode configuration: yaml: unmarshal errors:
  line 2: field authentcation not found in type gopinion.config
```

Unsupported policy:

```yaml
version: 1
authentication:
  mode: optional
```

```text
validate configuration: authentication.mode must be "required" or "disabled", got "optional"
```
