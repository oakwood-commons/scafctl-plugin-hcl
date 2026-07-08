# scafctl-plugin-hcl

Parse format validate and generate HCL configuration

## Installation

```bash
# Build from source
task build

# Or download from releases
gh release download --repo github.com/oakwood-commons/scafctl-plugin-hcl
```

## Usage

Register this plugin in your scafctl configuration, then reference
the **hcl** provider in your solutions. The provider supports four operations
via the `operation` input:

| Operation | Purpose |
| --- | --- |
| `parse` (default) | Extract structured blocks from HCL for discovery/inspection |
| `format` | Rewrite content to canonical HCL style |
| `validate` | Check syntax; report errors and deprecation warnings |
| `generate` | Produce HCL (`.tf`) or Terraform JSON (`.tf.json`) from structured data |

Provide input as inline `content`, a single `path`, a list of `paths`, or a
`dir` of `.tf`/`.tf.json` files.

```yaml
resolvers:
  tf-vars:
    resolve:
      with:
        - provider: hcl
          inputs:
            content: |
              variable "region" {
                type    = string
                default = "us-east-1"
              }
```

### Value conventions (parse output / generate input)

Structured values follow Terraform's own JSON conventions, so they interoperate
with the Terraform CLI and other tooling:

- **Expressions / references** are interpolation strings: `"${var.region}"`, `"${local.subnet}"`.
- **Type constraints** and **addresses** are bare strings: `"list(string)"`, `"aws_instance.web"`.
- **Everything else** is a literal value.

`generate` renders these to canonical HCL — bare type constraints (`type = string`,
not the deprecated `type = "string"`), bare expressions, and quoted literals — or
to spec-compliant Terraform JSON when `output_format: json` is set.


## Development

```bash
# Run tests
task test

# Run linter
task lint

# Build
task build

# Full CI pipeline
task ci
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

Apache-2.0 -- see [LICENSE](LICENSE) for details.
