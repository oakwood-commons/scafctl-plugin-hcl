---
description: "Create an implementation plan for a change to the hcl provider plugin: architecture, task breakdown, interface design, and testing strategy."
agent: "planner"
argument-hint: "Describe the change to plan (e.g., 'Add a dedicated lint operation')"
---
Create a structured implementation blueprint for the described change:

1. **Summary** -- what and why.
2. **Architecture decisions** -- packages affected, new types, interface changes,
   and provider-contract impact (capabilities, `Schema`, `OutputSchemas`).
3. **Task breakdown** -- ordered steps with files, complexity, and dependencies.
4. **Interface design** -- define contracts and signatures first.
5. **Error handling** -- wrapping strategy; validate at boundaries.
6. **Testing strategy** -- table-driven unit tests, benchmarks, round-trip
   coverage, and `DescribeWhatIf` parity.
7. **Documentation** -- README, descriptor `Examples`, and copilot-instructions.
8. **Risks & edge cases** -- nil/empty/zero inputs, breaking changes, version bump.

Follow the provider conventions in
`.github/instructions/scafctl-provider.instructions.md`. Plan only -- do not implement.
