# terraform-library-introspect

A reference scafctl solution that walks a *library* of Terraform/OpenTofu modules
and scaffolds a root module call for each one. Where
[terraform-module-scaffold](../terraform-module-scaffold) introspects a single
module, this example uses the hcl provider's `introspect-tree` operation to
discover every module under a directory and generate one file per module.

```text
hcl (introspect-tree)  ->  go-template (render-tree)  ->  file (write-tree)
   module library            rendered file entries          files on disk
```

## What it does

1. The `hcl` provider walks [`./modules`](./modules) with
   `operation: introspect-tree`, discovering each module subdirectory and
   returning a per-module collection. Each entry mirrors a single-module
   `introspect` result plus a `path` relative to the root.
2. The `go-template` provider (`operation: render-tree`) maps that collection
   into one `<module>.tf` render entry per module, rendering
   [`./templates/module_call.tf.tpl`](./templates/module_call.tf.tpl) with the
   module's introspected inputs.
3. The `file` provider (`operation: write-tree`) writes the rendered files to the
   output directory.

For each discovered module it produces a `<module>.tf` file that declares the
module's inputs as root variables and emits a `module` block wiring each input to
`var.<name>`.

## The module library

```text
modules/
  network/main.tf     # -> network.tf
  database/main.tf    # -> database.tf
```

`introspect-tree` discovers `network` and `database` as modules (each contains
HCL). The `depth` input controls how many directory levels below `modules` are
searched; `depth: 1` (the default) discovers immediate children.

## Run it

```bash
# From this directory. Requires the hcl provider installed locally
# (task publish:local from the repo root).
scafctl run solution -f solution.yaml -r outputDir=./generated
```

Override any input with `-r`, for example a deeper search or a different library:

```bash
scafctl run solution -f solution.yaml \
  -r modulesDir=./modules \
  -r depth=2 \
  -r outputDir=./generated
```

`allowMissing: true` is set on the introspect-tree call, so pointing at a
non-existent directory yields an empty result with a warning diagnostic instead
of a hard failure.

## Test it

The solution ships a functional test (`spec.testing.cases`) that exercises the
introspect-tree + render-tree stages end-to-end through the real plugin. From the
repo root:

```bash
task test:e2e
```

This installs the plugin into the local catalog and runs the functional tests for
every example, including:

```bash
scafctl test functional -f solution.yaml --verbose
```
