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
  default_size: 25
  maximum_size: 100
```

Clients use one-based `page` and `page_size` parameters:

```text
GET /orders?page=3&page_size=20
```

The handler receives:

```go
gopinion.PageRequest{Page: 3, Size: 20}
```

`request.Offset()` returns `40`.

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
        request.Offset(),
        request.Size,
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

- Page numbers or sizes below one
- Offset arithmetic overflow
- Negative totals
- More items than the requested page size
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
    "page": 3,
    "page_size": 20,
    "total_items": 84,
    "total_pages": 5
  }
}
```

## Invalid client input

| Input | Result |
| --- | --- |
| `page=0` | `400 invalid_page` |
| `page=abc` | `400 invalid_page` |
| `page_size=0` | `400 invalid_page_size` |
| Size above maximum | `400 page_size_too_large` |
| Offset overflow | `400 invalid_page` |
| Duplicate `page` | `400 invalid_page` |
| Duplicate `page_size` | `400 invalid_page_size` |
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
