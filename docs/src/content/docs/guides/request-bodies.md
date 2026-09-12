---
title: Request Bodies
description: Decode bounded JSON inputs before domain handlers execute.
sidebar:
  order: 3
---

`Post`, `Put`, and `Patch` strictly decode exactly one JSON value into their
generic input type before authorization preparation or domain handling.

```go
type createOrderInput struct {
    ProductID string `json:"product_id"`
    Quantity  int    `json:"quantity"`
}

func createOrder(ctx gopinion.Context, input createOrderInput, scope OrderScope) (Order, error) {
    if input.ProductID == "" {
        return Order{}, gopinion.NewHTTPError(400, "product_required", "product_id is required.")
    }
    if input.Quantity <= 0 {
        return Order{}, gopinion.NewHTTPError(400, "invalid_quantity", "quantity must be positive.")
    }
    return orders.Create(ctx.Request().Context(), scope, input.ProductID, input.Quantity)
}

app.Register(gopinion.AuthorizedPost("/orders", prepareCreateOrder, createOrder))
```

## Unknown struct fields fail

When the input type is a struct, `DisallowUnknownFields` prevents undeclared
object fields from reaching domain code.

```sh
curl -X POST http://localhost:8080/orders \
  -H 'Authorization: Bearer token' \
  -H 'Content-Type: application/json' \
  -d '{"product_id":"sku-1","quantity":2,"admin":true}'
```

```json
{
  "error": {
    "code": "invalid_json",
    "message": "The request body is not valid JSON."
  }
}
```

The unexpected `admin` field never reaches the struct handler. Map and `any`
input types are intentionally open-ended and should only be used when the
application validates their keys itself.

## Multiple values fail

This body is rejected even though both values are individually valid JSON:

```text
{"product_id":"sku-1","quantity":2} {"product_id":"sku-2","quantity":1}
```

GOpinion accepts exactly one JSON document per request.

## Body size is bounded

The default maximum is one MiB. Change it globally:

```yaml
server:
  max_body_bytes: 262144
```

An oversized body receives status `413` and code `body_too_large`.

## Domain validation

Strict decoding validates the wire shape, not business meaning. Keep semantic
rules in constructors or domain services:

```go
func createAccount(ctx gopinion.Context, input createAccountInput) (Account, error) {
    email, err := domain.ParseEmail(input.Email)
    if err != nil {
        return Account{}, gopinion.NewHTTPError(400, "invalid_email", "A valid email is required.")
    }
    return accounts.Create(ctx.Request().Context(), email)
}
```
