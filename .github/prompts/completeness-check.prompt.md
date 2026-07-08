---
mode: agent
description: "Check whether recent changes have matching tests, examples, docs, and provider-contract updates. Report gaps only -- creates nothing."
argument-hint: "Optional: specific area to check"
---

Review the current changes and check whether the supporting artifacts exist.
Report present vs missing as a checklist. **Do not create or modify anything --
just report the gaps.**

1. Run `git diff --cached --stat` to identify staged changes.
2. If nothing is staged, fall back to `git diff --stat HEAD` and
   `git log origin/main..HEAD --stat` to check unstaged and pushed changes.
3. For each feature, type, operation, or interface change, verify the following.

## Tests

- [ ] Unit tests in the corresponding `*_test.go` file (table-driven, testify)
- [ ] Happy path plus at least one error/edge path for new logic
- [ ] Benchmarks for parsing/rendering hot paths (`internal/hcl/*_benchmark_test.go`)
- [ ] Multi-file / directory behavior covered (`multifile_test.go`) when relevant
- [ ] Round-trip coverage where `parse` and `generate` are expected to be inverses
- [ ] `mock.go` (e.g. `FileReader`) updated if an interface changed

## Provider contract

- [ ] Every declared capability has a matching `OutputSchemas` entry
- [ ] New/changed inputs are reflected in the descriptor `Schema` (and enums)
- [ ] Every `operation` enum value is actually handled in `execute`
- [ ] `DescribeWhatIf` stays in parity with `ExecuteProvider`
- [ ] Descriptor `Examples` are accurate and would parse/validate

## Documentation

- [ ] `README.md` updated for user-facing behavior or new conventions
- [ ] Doc comments updated on exported types and functions that changed
- [ ] Descriptor `Description`/`Examples` match actual behavior
- [ ] `.github/copilot-instructions.md` updated if workflow/conventions changed

## Versioning and hygiene

- [ ] Provider `Version` bumped appropriately for breaking changes
- [ ] No leftover stubs, `TODO`, `panic("not implemented")`, or dead code
- [ ] `go vet ./...`, `task lint`, and `task test` pass

## Output

Produce a single checklist marking each item present or missing, grouped by the
sections above, and list the specific files or symbols still needing artifacts.
Do not make any edits.
