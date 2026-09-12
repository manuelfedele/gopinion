# GOpinion Agent Guide

## Purpose

GOpinion is a policy-first Go framework for bounded JSON HTTP applications.
Preserve or strengthen its global guarantees. Do not add endpoint-level escape
hatches for authentication, pagination, response envelopes, or server ownership.

## Repository Map

- `config.go`: strict `gopinion.yaml` defaults, decoding, and validation.
- `app.go`: application construction, routing, policy enforcement, and lifecycle.
- `auth.go`: authenticator and principal contracts.
- `route.go`: typed route constructors and request decoding.
- `pagination.go`: validated page requests and collection envelopes.
- `context.go`, `errors.go`: handler context and client-safe errors.
- `examples/todos`: runnable local example.
- `docs/src/content/docs`: authored documentation; `docs/dist` is generated.
- `.agents/skills`: task-specific contribution workflows.

## Invariants

- Required authentication runs before route selection and fails startup when no
  authenticator is supplied.
- Invalid credentials return `401`; authenticator infrastructure failures return
  a generic `500` and are logged.
- Authorization remains explicit in domain handlers.
- Pagination-required applications cannot return unbounded top-level collections.
- Request bodies remain size-bounded and strictly decode exactly one JSON value.
- Framework-owned responses use fixed JSON envelopes and `nosniff`.
- `App.Run` owns the listener, timeouts, cancellation, and graceful shutdown.
- GOpinion does not provide TLS, JWT verification, authorization, or secret storage.
- Security changes require explicit maintainer review before merge.

## Change Rules

- Prefer the smallest complete change and existing abstractions.
- Add regression tests for every behavior change.
- Update README, reference docs, guides, and runnable examples with the behavior
  they describe in the same change.
- Keep unknown YAML fields rejected and validate configuration before startup.
- Never put credentials or generated secrets in code, YAML, fixtures, logs, or docs.
- Do not edit generated `docs/dist`, `.astro`, `node_modules`, or `coverage.out`.
- Use the matching skill for policy, route, partial-PR, audit, or push work.

## Validation

Run for Go changes:

```sh
gofmt -w .
go mod tidy
go vet ./...
go test -race ./...
go run honnef.co/go/tools/cmd/staticcheck@v0.6.0 ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
```

Run in `docs/` for documentation or example changes:

```sh
npm ci
npm audit --audit-level=moderate
npm run build
```

## Git

- Do not commit, push, or open a PR unless explicitly requested.
- Never force-push, skip hooks, rewrite unrelated changes, or add AI authorship.
- Keep each PR to one logical change and resolve CI and review findings.
