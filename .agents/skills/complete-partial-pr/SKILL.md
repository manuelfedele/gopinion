---
name: complete-partial-pr
description: Evaluate and complete a GOpinion issue or pull request that fixes only a narrow symptom. Use when a contribution may miss adjacent policy, lifecycle, HTTP, tests, examples, or documentation contracts.
---

# Complete Partial PR

Treat the submitted patch as evidence, not as the boundary of the required fix.

## Workflow

1. Read `CONTRIBUTING.md`, the issue, PR description, review comments, current diff, and relevant history.
2. Separate the submitted patch, the underlying user problem, and the complete framework contract.
3. Map adjacent paths before editing:
   - configuration default and validation
   - startup and shutdown
   - authentication and principal propagation
   - route registration and method matching
   - request decoding and size limits
   - pagination and response validation
   - error envelopes, headers, and logging
   - README, reference docs, guides, and runnable examples
4. Check symmetric cases such as success/error, authenticated/unauthenticated, valid/malformed, exact/noncanonical path, first/later decode, empty/non-empty page, and startup/runtime failure.
5. Use authoritative Go or protocol documentation for external semantics. Record facts separately from inference.
6. Classify discovered work as required, regression coverage, optional follow-up, or out of scope.
7. Implement the smallest complete fix. Preserve public compatibility unless the contract requires a deliberate break.
8. Add tests at the public behavior level and update every example affected by the behavior.
9. Run an independent code and security review for authentication, parsing, routing, or lifecycle changes.

## GitHub Context

Use `gh pr view` and `gh issue view` when a number or URL is provided. For fork PRs, verify the head repository, branch, and `maintainerCanModify` before considering a push. Never force-push.

## Validation

Run the commands in `CONTRIBUTING.md`. Add
`go run honnef.co/go/tools/cmd/staticcheck@v0.6.0 ./...` and
`go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...` for substantive runtime
changes. Build the docs when examples or contracts change.

## Completion

Report the original gap, adjacent surfaces checked, changes made, tests added, commands run, and any intentionally deferred work.
