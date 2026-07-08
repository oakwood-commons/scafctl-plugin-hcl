---
description: "File a GitHub issue with codebase exploration and feasibility assessment."
agent: "issue-creator"
argument-hint: "Describe the change, bug, or feature (e.g., 'Preserve comments on round-trip')"
---
Create a GitHub issue for the described change. Follow this process:

1. **Explore** the codebase for relevant files, patterns, and interfaces.
2. **Assess** feasibility, scope (XS/S/M/L/XL), risks, and affected areas.
3. **Choose the repo**: plugin issues -> `oakwood-commons/scafctl-plugin-hcl`;
   host/CLI issues -> `oakwood-commons/scafctl`.
4. **Wait** for user confirmation before creating anything.
5. **Create** the issue via `gh issue create` with an appropriate title and a
   structured body (summary, reproduction/context, expected behavior, suggested fix).
