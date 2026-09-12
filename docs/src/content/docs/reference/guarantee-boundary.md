---
title: Guarantee Boundary
description: Know exactly what GOpinion can enforce and what requires repository or platform controls.
sidebar:
  order: 4
---

GOpinion guarantees its policies across the HTTP surface owned by `App.Run`.

## Enforced by the framework

- Every request is authenticated before routing when policy is required.
- No registered route can opt out of global authentication.
- Missing authenticators prevent startup.
- Singular top-level collections are rejected when pagination is required.
- List handlers receive bounded pagination and return validated pages.
- POST request bodies are bounded and strictly decoded.
- The raw mux and response writer are not exposed through framework APIs.
- Network timeouts and graceful shutdown are always configured.
- Internal errors and panics use client-safe responses.

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
gopinion.Get("/invalid", func(gopinion.Context) (any, error) {
    return []string{"unbounded"}, nil
})
```

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
