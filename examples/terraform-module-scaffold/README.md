# terraform-module-scaffold

A reference scafctl solution that turns a Terraform/OpenTofu module into a set of
per-environment scaffold files. It demonstrates the canonical
single-responsibility pipeline where each provider does one job:

```text
hcl (introspect)  ->  go-template (render-tree)  ->  file (write-tree)
   module metadata        rendered file entries         files on disk
```

## What it does

1. The `hcl` provider introspects [`./module`](./module) with
   `operation: introspect`, returning the module's required/optional inputs,
   outputs, and provider requirements.
2. The `go-template` provider (`operation: render-tree`) fans three templates in
   [`./templates`](./templates) across every environment, rendering each with the
   introspected module data plus the per-entry environment name.
3. The `file` provider (`operation: write-tree`) writes the rendered tree to the
   output directory, preserving the `<env>/...` structure.

For each environment it produces:

- `<env>/variables.tf` -- aggregated variable declarations mirroring the module.
- `<env>/<moduleName>_base.tf` -- a module call wiring each input to `var.<name>`.
- `<env>/<env>.auto.tfvars` -- a seed tfvars listing the required inputs.

## Run it

```bash
# From this directory. Requires the hcl provider installed locally
# (task publish:local from the repo root).
scafctl run solution -f solution.yaml -r outputDir=./generated
```

Override any input with `-r`, for example:

```bash
scafctl run solution -f solution.yaml \
  -r moduleDir=./module \
  -r moduleName=network \
  -r outputDir=./generated
```

## Test it

The solution ships a functional test (`spec.testing.cases`) that exercises the
introspect + render-tree stages end-to-end through the real plugin. From the repo
root:

```bash
task test:solution
```

This installs the plugin into the local catalog and runs:

```bash
scafctl test functional -f solution.yaml --verbose
```
