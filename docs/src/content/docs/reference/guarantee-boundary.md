---
title: Guarantee Boundary
description: Know exactly what GOpinion can enforce and what requires repository or platform controls.
sidebar:
  order: 4
---

GOpinion guarantees its policies across the HTTP surface owned by `App.Run`.
`New` always selects `gopinion.yaml`, but the deployment remains responsible for
controlling the process working directory and making that file immutable.

## Enforced by the framework

- Every request is authenticated before routing when policy is required.
- No registered route can opt out of global authentication.
- Missing authenticators prevent startup.
- Missing authorizers prevent startup when authorization is required.
- Required authorization rejects application routes without typed preparation.
- Prepared values reach handlers only after an `Allow` decision.
- Singular top-level collections are rejected when pagination is required.
- List handlers receive bounded `limit`/`offset` pagination, return validated
  pages, and emit RFC 5988 navigation links.
- POST request bodies are bounded and strictly decoded.
- The raw mux and response writer are not exposed through framework APIs.
- Network timeouts and graceful shutdown are always configured.
- Internal errors and panics use client-safe responses.

GOpinion does not generate credentials, validate JWTs, implement authorization
policy, or terminate TLS. These remain responsibilities of injected
authenticators and authorizers, domain preparation functions, and the deployment
platform.

## Enforced by Go's type system

- A list handler must return `Page[T]`.
- A route must come from a GOpinion constructor.
- A handler cannot write an arbitrary response through `gopinion.Context`.

## Runtime contract checks

Go interfaces can hide a concrete value. A singular handler returning `any`
could therefore return a slice dynamically. GOpinion checks the concrete value
before serialization and turns that policy violation into a generic `500`.

```go
// Registers because the static type is any, but fails safely at response time.
gopinion.AuthorizedGet(
    "/invalid",
    prepareInvalid,
    func(gopinion.Context, InvalidScope) (any, error) {
        return []string{"unbounded"}, nil
    },
)
```

Authorization enforcement proves that each protected handler was preceded by a
valid plan and an `Allow` decision. It cannot prove that an application selected
the correct action, resource, or trusted attributes when constructing the plan.
Preparation runs before authorization; the framework cannot prove that an
arbitrary Go callback is read-only, so preparation must never mutate state.
Generated `OPTIONS` and `405` responses expose registered method metadata to an
authenticated caller without invoking domain authorization.

## Outside the framework boundary

Arbitrary Go code can start a separate listener:

```go
go http.ListenAndServe(":9090", anotherHandler)
```

No Go library can prevent that from inside the same process. Organizations
that need protection against deliberate bypass should add external controls:

1. Reject direct imports of `net/http` and alternative routers outside approved packages.
2. Scan for additional listener construction in CI.
3. Permit only the configured application port in container and network policy.
4. Enforce identity again at the ingress, gateway, or service mesh.
5. Review changes to authentication and authorization paths explicitly.

The framework primarily prevents accidental inconsistency. Platform controls
handle malicious or intentionally non-compliant application code.
