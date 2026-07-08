---
description: "Apply fixes for triaged PR review comments and CI failures, verify, then respond to and resolve the threads."
agent: "pr-reviewer"
argument-hint: "Optional: PR number, or thread IDs / numbers to fix"
---
Continue the PR review workflow from an existing triage. **Only act on items the
user has approved.**

## Phase 1: Apply fixes

For each approved actionable item (review thread or CI failure):
1. Read the file and understand the context.
2. Make the minimal fix.
3. Add or update tests covering the fix.
4. Report what changed. Do **not** respond to threads yet.

## Phase 2: Verify

1. `go build ./...` and `go vet ./...`
2. `task test`
3. `task lint`
4. Fix any regressions introduced by the changes.

## Phase 3: Land the changes

1. Commit with a signed, DCO-signed conventional commit that references the
   fixed threads/issues.
2. Push the branch so CI re-runs.

## Phase 4: Respond and resolve

Only after fixes pass verification and are pushed:
1. Reply to each addressed thread via
   `addPullRequestReviewThreadReply` (GraphQL), briefly describing the fix.
2. Resolve each thread via `resolveReviewThread` (GraphQL).
3. For items intentionally not fixed, reply with the reasoning and resolve.

Report a final summary: threads resolved, CI status, and any follow-ups.
