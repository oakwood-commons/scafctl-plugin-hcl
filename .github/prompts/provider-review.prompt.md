---
description: "Review Go provider changes for scafctl-specific method semantics, schema correctness, WhatIf parity, and tests."
agent: "go-reviewer"
---

# Review Provider Changes

Use this prompt when reviewing changes to the **hcl** provider plugin.

## Contract coherence

- Do `Capabilities`, `Schema`, and `OutputSchemas` stay consistent?
- Does every declared capability (`CapabilityFrom`, `CapabilityTransform`) have a
  matching `OutputSchemas` entry? A missing entry surfaces to users as
  `provider not found`.
- Are output field names stable across versions?
- Are descriptor `Examples` still accurate and aligned with the README and tests?

## Correctness

- Are unknown provider names rejected consistently across all methods?
- Are `content` / `path` / `paths` / `dir` treated as mutually exclusive, with
  nil/empty inputs handled deliberately?
- Is map-derived output sorted for deterministic results?
- Is the filesystem accessed only through `FileReader`?
- Are errors wrapped with context, with no panics for recoverable failures?
- Does `DescribeWhatIf` describe the same operation as `ExecuteProvider` without
  side effects?

## HCL fidelity

- Does generated HCL avoid deprecated forms (e.g., quoted type constraints such
  as `type = "string"`)?
- Do parse and generate round-trip faithfully for the changed paths?

## Tests

- Are new files covered (no 0% coverage)?
- Are happy path and at least one error path tested?
- Is descriptor validity / `OutputSchemas` coverage asserted?

## Validate

~~~bash
task lint
task test
~~~
