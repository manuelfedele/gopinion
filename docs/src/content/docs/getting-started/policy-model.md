---
title: Policy Model
description: Understand GOpinion's global policy and enforcement boundaries.
sidebar:
  order: 3
---

GOpinion separates **policy** from **dependencies** and **domain behavior**.

| Concern | Source | Example |
| --- | --- | --- |
| Global policy | `gopinion.yaml` | Authentication is required |
| Runtime dependency | Go option | Which authenticator validates identity |
| Domain behavior | Handler code | Whether this user may update this order |

## One global decision

Authentication and pagination each have two modes:

- `required` enforces the policy across the entire application.
- `disabled` removes the global requirement.

There is intentionally no `optional` mode. Optional global authentication
would make the identity contract vary implicitly from request to request.

```yaml
authentication:
  mode: required
```

With this policy, every request passes through the configured authenticator
before route matching. Unknown routes and method mismatches are authenticated
too. Endpoint code cannot mark itself public.

## Opt out globally

An application that intentionally has no authentication must say so in the
single policy file:

```yaml
authentication:
  mode: disabled
```

```go
app, err := gopinion.New("gopinion.yaml")
```

Supplying no authenticator while mode is `required` fails application
construction. Supplying an authenticator does not override `disabled`; policy
still comes from the file.

## Invalid routes fail early

With pagination required, this route is rejected during registration:

```go
app.Register(gopinion.Get("/users", func(ctx gopinion.Context) ([]User, error) {
    return repository.All(ctx.Request().Context())
}))
```

Use a typed list route instead:

```go
app.Register(gopinion.List("/users", func(
    ctx gopinion.Context,
    request gopinion.PageRequest,
) (gopinion.Page[User], error) {
    users, total, err := repository.Page(
        ctx.Request().Context(),
        request.Offset(),
        request.Size,
    )
    if err != nil {
        return gopinion.Page[User]{}, err
    }
    return gopinion.NewPage(users, total, request)
}))
```

## Enforcement layers

1. Configuration rejects unknown fields and unsupported values.
2. Application construction fails when required dependencies are missing.
3. Route registration rejects incompatible response contracts.
4. Request handling authenticates before dispatch and validates inputs.
5. Response handling checks dynamic collection values and fixed envelopes.

No host language can prevent a developer from starting a second raw
`net/http` listener. See the [guarantee boundary](../../reference/guarantee-boundary/)
for deliberate-bypass controls.
