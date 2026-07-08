# scafctl-plugin-hcl - AI Agent Instructions

## Overview

scafctl **provider** plugin implementing the **hcl** provider: parse, format,
validate, and generate Terraform/OpenTofu HCL configuration.

## Key Patterns

- **Plugin SDK**: Uses `github.com/oakwood-commons/scafctl-plugin-sdk`
- **Entry point**: `cmd/scafctl-plugin-hcl/main.go` is a thin startup layer that
  calls `sdkplugin.Serve(hcl.NewPlugin())`
- **Implementation**: Core logic lives in `internal/hcl/...`
  (`parser.go`, `generate.go`, `validate.go`, `provider.go`)

## MCP Preference

If this workspace has the scafctl MCP server configured in VS Code, prefer MCP
tools over shell commands for scafctl-specific tasks.

- Provider and capability discovery: `list_providers`, `get_provider_schema`,
  `get_provider_output_shape`
- Expression and template checks: `validate_expression`, `evaluate_cel`,
  `evaluate_go_template`
- Solution-side debugging when testing this plugin: `inspect_solution`,
  `preview_resolvers`, `preview_action`, `dry_run_solution`
- Fall back to CLI when you need the exact user-facing command line, local build
  output, or behavior outside MCP coverage.

## Provider Semantics

Keep the provider contract coherent across all methods.

- `GetProviderDescriptor` defines the public contract: name, description,
  capabilities, `Schema`, and `OutputSchemas`.
- `OutputSchemas` must include an entry for **every** declared capability
  (`CapabilityFrom`, `CapabilityTransform`). A missing output schema causes host
  registration failure, surfaced to users as `provider not found`.
- `ExecuteProvider` implements the contract and rejects unknown provider names
  consistently.
- `DescribeWhatIf` describes the same action without side effects.
- `ConfigureProvider` stores host configuration; it should not perform heavy work.
- Only customize `ExecuteProviderStream` and `ExtractDependencies` when the
  provider genuinely needs them.

See `.github/instructions/scafctl-provider.instructions.md` for detailed provider
implementation guidance. Use `.github/prompts/provider-implementation.prompt.md`
when implementing changes and `.github/prompts/provider-review.prompt.md` when
reviewing them.

### Provider Naming And Binary Resolution

- Provider identity comes from the name returned by `GetProviders` /
  `GetProviderDescriptor` and the published catalog artifact name — not the raw
  binary filename.
- Keep the published plugin/catalog name (`hcl`) aligned with the provider name
  users reference in solutions (`provider: hcl`).
- The executable just needs to be runnable. On Windows it must end in `.exe`.

## Conventions

- **Commits**: Use [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/)
- **Signing**: All commits must be GPG/SSH signed (`-S`) and include DCO sign-off (`-s`)
- **Errors**: Return errors with `fmt.Errorf("context: %w", err)`, don't panic for
  recoverable failures

## Build & Test Commands

~~~bash
task build          # Build the plugin binary
task test           # Run tests (race, shuffled)
task test:cover     # Run tests with coverage
task lint           # Run linter
task lint:fix       # Run linter with auto-fix
task bench          # Run benchmarks
task release:local  # Build and install a versioned local catalog artifact
task ci             # Full CI pipeline (lint + test + build)
~~~

## Local Testing Workflow

The most reliable way to verify the provider end-to-end is to install it locally
and run a sample solution through the host:

~~~bash
task release:local VERSION=2.0.0
scafctl run provider hcl operation=parse content='variable "x" { type = string }'
~~~

Direct `--plugin-dir` testing may not exercise the same registration path as
catalog-installed plugins.

## Release and Catalog Publishing

A tagged release must publish **both** the provider artifact and refresh the
catalog index; publishing the artifact alone does not make the provider
discoverable. Tag with `task release:tag VERSION=x.y.z`, which triggers the CI
release workflow.

Catalog auth in CI: write `~/.docker/config.json` directly rather than running
`scafctl auth login ... --write-registry-auth`, which fails in headless CI
(no keyring). `scafctl catalog push` / `index push` read the Docker config.
Use an org token with `repo`, `read:packages`, and `write:packages` scopes — the
default repo `GITHUB_TOKEN` cannot write the org-level catalog index (403).

## Critical Rules

- **No hardcoded paths**: Use the SDK interfaces for all host interactions.
- **Test coverage**: Every new file must have tests. Target 70%+ patch coverage.
- **Git safety**: Never run git commit/push unless explicitly asked.
