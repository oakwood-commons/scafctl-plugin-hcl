---
mode: agent
description: "Implement changes to the hcl provider plugin while keeping the provider contract coherent."
---

# Implement Provider Changes

Use this prompt when implementing changes to the **hcl** provider plugin.

## Before you start

- Read `internal/hcl/provider.go` to understand the current descriptor,
  capabilities (`CapabilityFrom`, `CapabilityTransform`), and `OutputSchemas`.
- Read the operation implementations you will touch: `parser.go`, `generate.go`,
  `validate.go`.
- Follow `.github/instructions/scafctl-provider.instructions.md`,
  `.github/instructions/go-conventions.instructions.md`, and
  `.github/instructions/go-testing.instructions.md`.

## Implementation checklist

1. **Keep the contract coherent.** If you add or change a capability, update
   `Capabilities`, `Schema`, and `OutputSchemas` together. Every capability must
   have a matching `OutputSchemas` entry or the host reports `provider not found`.
2. **Reject unknown providers** consistently in `ExecuteProvider`,
   `GetProviderDescriptor`, and `DescribeWhatIf`.
3. **Handle inputs deliberately.** `content`, `path`, `paths`, and `dir` are
   mutually exclusive. Handle nil/empty values explicitly.
4. **Keep output deterministic.** Sort map-derived keys so parse and generate
   results are stable.
5. **Access the filesystem only through `FileReader`** so behavior stays testable.
6. **Wrap errors** with `fmt.Errorf("context: %w", err)`; never panic for
   recoverable failures.
7. **Update `DescribeWhatIf`** to stay in parity with `ExecuteProvider`, without
   side effects.
8. **Keep `Examples` in the descriptor** aligned with README and tests.

## Tests

Add or update tests in the same change:

- known vs unknown provider name
- happy path for each affected operation
- invalid, nil, and empty input
- descriptor validity and `OutputSchemas` coverage for all capabilities
- round-trip fidelity where parse/generate are expected to be inverses

## Validate

~~~bash
task lint
task test
task build
~~~
