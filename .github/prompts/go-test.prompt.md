---
description: "Run Go tests with race detection, check coverage, and diagnose failures for the hcl provider."
agent: "go-fixer"
---
Run the Go test suite and report results:

1. Run `task test` (`go test -race -count=1 -shuffle=on ./...`) -- report any failures.
2. Run `go test -coverprofile=coverage.out ./...` -- check coverage.
3. Run `go tool cover -func=coverage.out | tail -1` -- report total coverage.
4. If failures exist, diagnose the root cause and suggest or apply minimal fixes.
5. Flag packages or changed functions below 80% coverage and recommend specific
   missing test cases.

Focus on recently changed packages first; use `git diff --name-only -- '*.go'`
to identify them. Remove `coverage.out` when done.
