---
title: Pagination
description: Make every collection request explicit, bounded, and predictable.
sidebar:
  order: 4
---

Pagination is required by default. GOpinion parses the query, enforces the
configured maximum, and gives handlers validated coordinates.

## Configure bounds

```yaml
pagination:
  mode: required
  default_limit: 25
  maximum_limit: 100
  maximum_offset: 10000
```

Clients use zero-based `offset` and bounded `limit` parameters:

```text
GET /orders?limit=20&offset=40
```

The handler receives:

```go
gopinion.PageRequest{Limit: 20, Offset: 40}
```

## Query a repository

```go
func listOrders(
    ctx gopinion.Context,
    request gopinion.PageRequest,
    scope OrderScope,
) (gopinion.Page[Order], error) {
    items, total, err := orderRepository.ListAuthorized(
        ctx.Request().Context(),
        scope,
        request.Offset,
        request.Limit,
    )
    if err != nil {
        return gopinion.Page[Order]{}, err
    }
    return gopinion.NewPage(items, total, request)
}
```

A SQL implementation can apply both values directly:

```go
func (r *OrderRepository) ListAuthorized(ctx context.Context, scope OrderScope, offset, limit int) ([]Order, int64, error) {
    rows, err := r.db.QueryContext(ctx, `
        SELECT id, customer_id, status
        FROM orders
        WHERE customer_id = ?
        ORDER BY id
        LIMIT ? OFFSET ?
    `, scope.CustomerID, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    items, err := scanOrders(rows)
    if err != nil {
        return nil, 0, err
    }

    var total int64
    err = r.db.QueryRowContext(
        ctx,
        `SELECT COUNT(*) FROM orders WHERE customer_id = ?`,
        scope.CustomerID,
    ).Scan(&total)
    return items, total, err
}
```

## Construct a valid page

`NewPage` rejects:

- Limits below one
- Negative offsets
- Negative totals
- More items than the requested limit
- More returned items than the declared total
- Returned items that cannot exist at the requested offset

```go
page, err := gopinion.NewPage(items, total, request)
```

An empty page still serializes `data` as an empty array, never `null`.

## Response envelope

```json
{
  "data": [
    {"id": "o-101", "status": "paid"},
    {"id": "o-102", "status": "pending"}
  ],
  "pagination": {
    "limit": 20,
    "offset": 40,
    "totalItems": 84
  }
}
```

Collection responses also include RFC 5988 Web Linking relations while
preserving non-pagination query parameters:

```text
Link: </orders?limit=20&offset=0>; rel="first", </orders?limit=20&offset=20>; rel="prev", </orders?limit=20&offset=60>; rel="next", </orders?limit=20&offset=80>; rel="last"
```

Only relations that are available are emitted. An empty collection has no
`Link` header.

## Invalid client input

| Input | Result |
| --- | --- |
| `limit=0` | `400 invalid_limit` |
| `limit=abc` | `400 invalid_limit` |
| Limit above maximum | `400 limit_too_large` |
| `offset=-1` | `400 invalid_offset` |
| Offset above maximum | `400 offset_too_large` |
| Duplicate `limit` | `400 invalid_limit` |
| Duplicate `offset` | `400 invalid_offset` |
| Legacy `page` or `page_size` | `400 invalid_pagination` |
| Malformed query encoding | `400 invalid_query` |

## Why ordinary GET cannot return a slice

This fails registration while pagination is required:

```go
gopinion.AuthorizedGet(
    "/orders",
    prepareOrderList,
    func(ctx gopinion.Context, scope OrderScope) ([]Order, error) {
        return orders.AllAuthorized(ctx.Request().Context(), scope)
    },
)
```

Top-level slices, arrays, maps, and `Page[T]` values must not be returned from
a singular route. Dynamic collection values hidden behind `any` are rejected
at response time.

## Disable the requirement

```yaml
pagination:
  mode: disabled
```

This globally allows singular routes to return top-level collections. Typed
`AuthorizedList` routes remain available if some collections should still be
paginated.
