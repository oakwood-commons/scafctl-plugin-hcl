---
description: "scafctl provider guidance: method responsibilities, schema design, WhatIf parity, configuration, and tests."
applyTo: "**/*.go"
---

# scafctl Provider Guidance

Use this guidance when editing provider plugin code in `internal/hcl/...`.

## Method Responsibilities

Keep the provider lifecycle methods sharply separated.

- `GetProviders` returns the provider name users reference in solutions (`hcl`).
- `GetProviderDescriptor` describes the provider contract.
- `ExecuteProvider` performs the real work (parse, format, validate, generate).
- `DescribeWhatIf` explains what execution would do without side effects.
- `ConfigureProvider` stores host configuration for later use.
- `ExecuteProviderStream` should only be implemented when streaming output is
  truly supported.
- `ExtractDependencies` should only return resolver dependencies when provider
  inputs reference them.
- `StopProvider` should be safe and idempotent.

## Descriptor Rules

The descriptor is the source of truth for the provider contract.

- Keep `Name`, `DisplayName`, `Description`, `Capabilities`, `Schema`, and
  `OutputSchemas` consistent.
- Every declared capability must have a corresponding entry in `OutputSchemas`.
- Do not advertise capabilities that the implementation does not support.
- A descriptor without `OutputSchemas` fails host registration; the host
  validates descriptors before making providers available.
- Ensure required schema fields match runtime expectations.
- Keep `Examples` realistic and aligned with README snippets and tests.

## OutputSchemas Contract

`OutputSchemas` maps each declared capability to a JSON Schema describing the
provider's output shape for that capability.

- The map key must be a `sdkprovider.Capability` constant (e.g.,
  `sdkprovider.CapabilityFrom`, `sdkprovider.CapabilityTransform`).
- Every capability listed in `Capabilities` must appear in `OutputSchemas`.
- Missing `OutputSchemas` causes the host to reject the provider at registration
  time. The user-facing error is `provider not found` because the provider never
  registers.
- Use `sdkhelper.ObjectSchema` / `sdkhelper.ArrayProp` / `sdkhelper.AnyProp` to
  define output shapes consistently.
- Keep output field names stable across versions (`variables`, `resources`,
  `data`, `modules`, `outputs`, `locals`, `providers`, `terraform`, `moved`,
  `import`, `check`).

## Provider Name vs Binary Name

- The provider identity comes from the name returned over RPC and the published
  catalog artifact name, not the exact binary filename.
- `GetProviders` returns `hcl`; `GetProviderDescriptor("hcl")` describes that
  same provider.
- The executable only needs to be runnable. On Windows it must end in `.exe`.
- Do not rely on the filename alone to make `provider: hcl` resolve.

## Execution Rules

- Reject unknown provider names consistently across all methods.
- Handle nil, empty, and zero-value inputs deliberately (`content`, `path`,
  `paths`, `dir` are mutually exclusive).
- Return structured output with stable field names.
- Avoid hidden global state and cross-call mutation.
- Wrap non-trivial errors with context using `fmt.Errorf("context: %w", err)`.
- Favor deterministic output: sort map-derived keys so generated HCL and parsed
  results are stable across runs.

## WhatIf Rules

`DescribeWhatIf` should be useful and trustworthy.

- Describe the same operation that `ExecuteProvider` would perform.
- Do not perform I/O, mutation, or other side effects.
- Mention important inputs (operation, source of content) when they affect
  behavior.

## Configuration Rules

- Read host-provided settings through `ConfigureProvider`.
- Keep configuration validation close to where values are consumed.
- Do not hardcode secrets, tokens, URLs, or filesystem assumptions; access the
  filesystem only through the `FileReader` interface so logic stays testable.

## Test Expectations

Every provider change should consider tests for:

- known provider vs unknown provider
- happy path execution for each operation (parse, format, validate, generate)
- invalid, nil, or empty input
- descriptor validity and `OutputSchemas` coverage for all capabilities
- `DescribeWhatIf` parity with execution intent
- round-trip fidelity where parse and generate are expected to be inverses
- any new edge case introduced by the change
