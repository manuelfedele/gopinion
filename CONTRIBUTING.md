# Contributing

GOpinion intentionally rejects configuration and endpoint behavior that less
opinionated frameworks permit. Changes should preserve or strengthen those
guarantees rather than add route-level escape hatches.

Before opening a pull request, run:

```sh
gofmt -w .
go mod tidy
go vet ./...
go test -race ./...
```

Behavior changes require tests and corresponding README updates. Security and
authentication changes require explicit maintainer review.

Build the documentation site with Node.js 22.12 or newer:

```sh
cd docs
npm ci
npm audit --audit-level=moderate
npm run build
```
