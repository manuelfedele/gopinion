---
name: poweruser-feature-audit
description: Independently audit a substantial GOpinion feature from a production user's perspective. Use for new HTTP capabilities, policy mechanisms, authentication integrations, or lifecycle behavior that needs evidence-backed completeness review.
---

# Power-User Feature Audit

Research the required behavior before reading the implementation so the code does not anchor the expected contract.

## Workflow

1. Scope the feature from the PR or issue without evaluating its implementation approach.
2. Research primary sources first: Go standard-library documentation, relevant RFCs, OWASP guidance, and identity-provider or protocol specifications.
3. Record source URLs and distinguish normative requirements, recommended hardening, and product choices.
4. Design an ideal test catalog before opening the implementation diff.
5. Include happy paths, malformed input, limits, cancellation, concurrency, protocol headers, noncanonical paths, dependency failures, and disabled-policy behavior where relevant.
6. Add architecture requirements that tests alone cannot prove, especially ownership boundaries and fail-closed behavior.
7. Compare the implementation and existing tests against the catalog only after the catalog is complete.
8. Classify each item as covered, partial, missing test, or apparently unimplemented.
9. Independently verify every proposed finding against the current worktree before reporting it.
10. Draft findings ordered by severity with exact file and line references. Do not post review comments or modify the PR without user approval.

## GOpinion Audit Lenses

- Authentication must precede routing and distinguish invalid credentials from operational failures.
- Authorization remains domain-owned and must not be implied by authentication.
- Request bodies and pagination must remain bounded under malformed and adversarial inputs.
- All framework-owned responses must preserve documented JSON envelopes and headers.
- `Run` must have deterministic startup, cancellation, shutdown, and failure behavior.
- Documentation must separate framework guarantees from TLS, secret management, and identity-provider responsibilities.
- Examples must compile and must not present demo credentials as production security.

## Artifacts

For a large audit, keep temporary research outside the repository unless the user requests committed design notes. Produce a final test catalog, gap table, verified findings, and validation results.
