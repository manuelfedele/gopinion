---
name: add-route-constructor
description: Add or change a GOpinion route constructor such as Get, Post, or List. Use when introducing an HTTP method, request shape, response kind, or route-level contract.
---

# Add Route Constructor

Add route behavior without weakening GOpinion's global policy guarantees.

## Workflow

1. Read `route.go`, `app.go`, `context.go`, and the related tests before designing the API.
2. Identify the closest existing constructor and preserve its generic handler shape where possible.
3. Map the complete request path: registration, authentication, route matching, input decoding, handler invocation, response validation, serialization, and error handling.
4. Confirm the `net/http.ServeMux` method and pattern semantics against the current stable Go documentation.
5. Decide explicitly whether the route returns a singular response or `Page[T]`. Never create an unbounded collection path while pagination is required.
6. Keep `http.ResponseWriter` unavailable to application handlers so fixed envelopes and status behavior remain enforceable.
7. Reject malformed input before invoking domain code. Preserve body limits and strict JSON decoding for body-bearing routes.
8. Add focused tests in `app_test.go` covering success, authentication, wrong method, exact `Allow` headers, implicit `HEAD` behavior for `GET`, invalid input, handler error, panic, and applicable policy failures.
9. Update `README.md`, `docs/src/content/docs/reference/api.md`, routing guidance, response status documentation, and a runnable example when public behavior changes.

## Validation

```sh
gofmt -w .
go mod tidy
go vet ./...
go test -race ./...
go run honnef.co/go/tools/cmd/staticcheck@v0.6.0 ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
```

Build documentation when examples or public behavior change:

```sh
cd docs
npm ci
npm audit --audit-level=moderate
npm run build
```

## Guardrails

- Authentication remains global and runs before routing.
- Pagination remains a global policy, not a per-route escape hatch.
- Framework responses remain JSON with `X-Content-Type-Options: nosniff`.
- Unknown paths and method mismatches use framework error envelopes.
- Do not add a public API when an existing constructor or option can express the behavior.
- Treat authentication and authorization changes as requiring explicit maintainer review.

## Completion

Report the public API change, contract cases tested, documentation updated, commands run, and any deliberate non-goals.
