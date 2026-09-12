---
name: add-policy-feature
description: Add or change a GOpinion gopinion.yaml policy. Use when introducing configuration, defaults, validation, startup checks, or global request enforcement.
---

# Add Policy Feature

Implement policy as a strict, globally enforced contract rather than an endpoint option.

## Workflow

1. Read `config.go`, `config_test.go`, `app.go`, and `CONTRIBUTING.md`.
2. Define the threat or consistency problem the policy solves.
3. Choose a safe default. Security-sensitive controls should fail closed when dependencies or required values are absent.
4. Add the YAML field to the smallest relevant configuration struct.
5. Preserve `yaml.Decoder.KnownFields(true)` and single-document parsing.
6. Validate values during `gopinion.New`, before routes register or listeners start.
7. Enforce the policy at the framework-owned lifecycle, request, or response point appropriate to its scope. Do not add route-level bypasses.
8. Test omitted defaults, valid values, invalid values, unknown fields, startup failures, and runtime enforcement.
9. Update the complete configuration, installation defaults, guarantee boundary, API behavior, and affected examples.

## Security Questions

- Does omission choose the safest usable behavior?
- Does missing runtime infrastructure prevent startup rather than silently disable protection?
- Can application code bypass the policy through a registered route?
- Are external responsibilities such as TLS, identity verification, authorization, and secret storage stated accurately?
- Does the policy introduce credentials into YAML, logs, errors, fixtures, or repository history? It must not.

## Validation

```sh
gofmt -w .
go mod tidy
go vet ./...
go test -race ./...
go run honnef.co/go/tools/cmd/staticcheck@v0.6.0 ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
```

For documentation changes, also run `npm ci`, `npm audit --audit-level=moderate`, and `npm run build` from `docs/`.

## Completion

Report the default, validation rules, enforcement point, negative tests, documentation changes, and remaining responsibilities outside the framework boundary.
