---
description: "Fix Go build errors, go vet warnings, and linter issues with minimal surgical changes."
agent: "go-fixer"
---
Diagnose and fix the current Go build and lint issues:

1. Run `go build ./...` to identify compilation errors.
2. Run `go vet ./...` to find vet warnings.
3. Run `task lint` for linter issues.
4. Apply minimal, surgical fixes -- do not refactor.
5. Run `go build ./...` and `task test` again to verify nothing broke.

Stop if the same error persists after 3 attempts or the fix requires
architectural changes -- surface options instead.
