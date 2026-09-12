---
title: Authorization
description: Prepare trusted resource facts and authorize every application route.
sidebar:
  order: 2
---

Authorization is required by default. Application construction fails without an
authorizer, and route registration rejects constructors without a typed
authorization phase.

## Implement the decision contract

```go
type orderAuthorizer struct{}

func (orderAuthorizer) Authorize(
    _ context.Context,
    principal gopinion.Principal,
    request gopinion.AuthorizationRequest,
) (gopinion.AuthorizationDecision, error) {
    switch request.Action {
    case "order:read":
        ownerID, ok := request.Resource.Attributes["owner_id"].(string)
        if !ok {
            return gopinion.Deny, errors.New("order owner is missing")
        }
        if ownerID == principal.Subject {
            return gopinion.Allow, nil
        }
        return gopinion.Deny, nil
    default:
        return gopinion.Deny, nil
    }
}
```

Unknown actions deny by default. Malformed trusted data is an operational error,
not a denial, and becomes a generic `500` response.

Supply the dependency at construction:

```go
app, err := gopinion.New(
    "gopinion.yaml",
    gopinion.WithAuthenticator(authenticator),
    gopinion.WithAuthorizer(orderAuthorizer{}),
)
```

## Prepare and authorize a resource

The preparation phase loads the resource and supplies only trusted facts:

```go
func prepareOrder(ctx gopinion.Context) (gopinion.AuthorizationPlan[Order], error) {
    order, err := orders.Find(ctx.Request().Context(), ctx.PathValue("id"))
    if err != nil {
        return gopinion.AuthorizationPlan[Order]{}, err
    }
    return gopinion.AuthorizationPlan[Order]{
        Request: gopinion.AuthorizationRequest{
            Action: "order:read",
            Resource: gopinion.AuthorizationResource{
                Type: "order",
                ID: order.ID,
                Attributes: map[string]any{
                    "owner_id": order.OwnerID,
                },
            },
        },
        Value: order,
    }, nil
}

app.Register(gopinion.AuthorizedGet(
    "/orders/{id}",
    prepareOrder,
    func(_ gopinion.Context, order Order) (OrderView, error) {
        return toOrderView(order), nil
    },
))
```

The handler cannot run or receive `order` unless the authorizer returns
`gopinion.Allow`.

## Collections

Authorization must constrain the query before pagination. Pass a prepared scope
to the repository and calculate totals from authorized rows. Do not paginate an
unbounded collection and then remove unauthorized items. That leaks incorrect
totals and produces sparse pages.

## Failure semantics

| Authorizer result | HTTP result |
| --- | --- |
| `Allow, nil` | Handler executes |
| `Deny, nil` | `403 forbidden` |
| Any error | Generic `500`, error logged |
| Invalid decision | Generic `500`, error logged |

Preparation may return a client-safe `HTTPError`, such as `404` for a missing
resource. GOpinion validates that action, resource type, and resource ID are all
non-empty before calling the authorizer.

A `403` confirms that the resource exists. Applications that must conceal
existence should use a principal-scoped lookup during preparation and return the
same `404` for missing and inaccessible resources.

Generated `OPTIONS` and `405` responses expose the methods registered for an
authenticated path without running domain authorization. Do not treat route
shape as secret authorization data.

## Disable the global requirement

```yaml
authorization:
  mode: disabled
```

This permits ordinary route constructors. It does not bypass explicitly
authorized routes, which still require and invoke an authorizer.
