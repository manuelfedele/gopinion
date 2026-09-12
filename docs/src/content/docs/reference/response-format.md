---
title: Response Format
description: Exact JSON envelopes returned by GOpinion applications.
sidebar:
  order: 3
---

GOpinion fixes response envelopes so clients do not infer structure per route.

## Singular success

```json
{
  "data": {
    "id": "u-17",
    "name": "Ada"
  }
}
```

The `data` value may be a scalar or object. With pagination required, it may
not be a top-level array, map, or page.

## Collection success

```json
{
  "data": [
    {"id": "u-17", "name": "Ada"},
    {"id": "u-18", "name": "Grace"}
  ],
  "pagination": {
    "limit": 25,
    "offset": 0,
    "totalItems": 42
  }
}
```

For zero items, `data` is `[]` and `totalItems` is `0`.

## Error

```json
{
  "error": {
    "code": "not_found",
    "message": "The requested resource does not exist."
  }
}
```

## Headers

All framework responses set `X-Content-Type-Options`. Responses with a JSON body
also set `Content-Type`:

```text
Content-Type: application/json
X-Content-Type-Options: nosniff
```

A method mismatch and generated `OPTIONS` response also set an `Allow` header:

```text
Allow: GET, HEAD, OPTIONS
```

Paginated responses use the RFC 5988 `Link` header. Available relations are
`first`, `prev`, `next`, and `last`:

```text
Link: </users?limit=25&offset=0>; rel="first", </users?limit=25&offset=25>; rel="next", </users?limit=25&offset=25>; rel="last"
```

An authentication failure sets `WWW-Authenticate`. The default challenge is
`Bearer`; authenticators can provide another challenge.

## Statuses

| Route or condition | Status |
| --- | --- |
| `Get` success | 200 |
| `List` success | 200 |
| `Post` success | 201 |
| `Put` success | 200 |
| `Patch` success | 200 |
| `Delete` success | 204 |
| Generated `OPTIONS` | 204 |
| Invalid input | 400 |
| Unauthenticated | 401 |
| Authorization denied | 403 |
| Unknown route | 404 |
| Method mismatch | 405 |
| Oversized body | 413 |
| Internal or contract failure | 500 |
