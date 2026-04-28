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
the **hcl** provider in your solutions:

```yaml
resolvers:
  my-value:
    resolve:
      with:
        - provider: hcl
          inputs:
            value: "hello"
```

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
