---
name: pushing-commits-to-the-repo
description: Commit, push, and open or update a GOpinion pull request. Use only when the user explicitly requests a commit, push, or PR, and continue until checks and review comments are resolved.
---

# Push Commits To GOpinion

Pushing starts the verification loop. It does not finish the task.

## Before Committing

1. Inspect `git status`, the complete diff, and recent commits.
2. Separate unrelated user changes and stage only the intended files.
3. Run the checks required by `CONTRIBUTING.md`:

```sh
gofmt -w .
go mod tidy
go vet ./...
go test -race ./...
```

4. Run `go run honnef.co/go/tools/cmd/staticcheck@v0.6.0 ./...` and `go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...` for Go changes. Build the docs for documentation or example changes.
5. Run an independent code review. Add a security review for authentication, authorization, parsing, routing, lifecycle, or dependency changes.
6. Re-run affected checks after every review-driven edit.

## Commit

- Commit only when explicitly requested.
- Follow the repository's concise imperative commit style.
- Never force-push, skip hooks, or add AI co-author trailers.
- Do not amend unless the user explicitly requests it.
- Use the contributor's configured Git identity. If it is absent, ask the contributor to configure it rather than guessing or impersonating another person.

## Pull Request

Before opening or updating a PR, inspect the base branch, tracking branch, remote, all included commits, and the complete base-to-head diff.

Keep the PR focused on one logical change. The body should state:

1. Why the change is needed.
2. User-visible behavior and public API changes.
3. Security or compatibility implications.
4. Tests and validation commands run.

Use `gh` for GitHub operations and return the PR URL.

## After Push

1. Watch CI to a terminal state.
2. Diagnose failures and fix those caused by the branch.
3. Read every automated and human review comment.
4. Fix valid findings. Explain invalid findings with concrete code evidence.
5. Never silently discard a comment or force-push follow-up work.
6. Repeat review and validation after code changes.

Security changes, including authentication and authorization changes, require
explicit maintainer review before merge, as required by `CONTRIBUTING.md`.

## Completion

Stop when CI is green and no review comment remains unresolved. If external CI
failures or decisions requiring a maintainer prevent that state, report them as
explicit blockers. Report the commit, branch, PR URL, checks, and any human-only
merge decision.
