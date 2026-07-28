// Copyright 2025-2026 Oakwood Commons
// SPDX-License-Identifier: Apache-2.0

package hcl

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	sdkprovider "github.com/oakwood-commons/scafctl-plugin-sdk/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tfModuleVariables = `
variable "region" {
  type        = string
  description = "AWS region for resources"
}

variable "instance_count" {
  type    = number
  default = 3
}

variable "db_password" {
  type      = string
  sensitive = true
}
`

const tfModuleMeta = `
terraform {
  required_version = ">= 1.5.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}

output "vpc_id" {
  description = "The ID of the VPC"
  value       = aws_vpc.main.id
}

output "db_secret" {
  value     = var.db_password
  sensitive = true
}

module "subnet" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.1.0"
}

resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}

data "aws_ami" "ubuntu" {
  most_recent = true
}
`

// findByName returns the map entry with a matching "name" field, or nil.
func findByName(items []any, name string) map[string]any {
	for _, it := range items {
		if m, ok := it.(map[string]any); ok && m["name"] == name {
			return m
		}
	}
	return nil
}

func TestPlugin_Execute_Introspect_InlineContent(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := context.Background()

	output, err := p.ExecuteProvider(ctx, ProviderName, map[string]any{
		"operation": "introspect",
		"content":   tfModuleVariables + tfModuleMeta,
	})
	require.NoError(t, err)
	require.NotNil(t, output)

	data := output.Data.(map[string]any)

	// Inputs: required vs optional classification.
	inputs := data["inputs"].([]any)
	require.Len(t, inputs, 3)

	region := findByName(inputs, "region")
	require.NotNil(t, region)
	assert.Equal(t, true, region["required"], "region has no default so it is required")
	assert.Nil(t, region["default"])
	assert.Equal(t, "string", region["type"])
	assert.Equal(t, "AWS region for resources", region["description"])

	count := findByName(inputs, "instance_count")
	require.NotNil(t, count)
	assert.Equal(t, false, count["required"], "instance_count has a default so it is optional")
	assert.NotNil(t, count["default"])

	secret := findByName(inputs, "db_password")
	require.NotNil(t, secret)
	assert.Equal(t, true, secret["required"])
	assert.Equal(t, true, secret["sensitive"])

	// Outputs.
	outputs := data["outputs"].([]any)
	require.Len(t, outputs, 2)
	vpcID := findByName(outputs, "vpc_id")
	require.NotNil(t, vpcID)
	assert.Equal(t, "The ID of the VPC", vpcID["description"])
	dbSecret := findByName(outputs, "db_secret")
	require.NotNil(t, dbSecret)
	assert.Equal(t, true, dbSecret["sensitive"])

	// Provider and core requirements.
	providers := data["required_providers"].([]any)
	require.Len(t, providers, 1)
	aws := findByName(providers, "aws")
	require.NotNil(t, aws)
	assert.Equal(t, "hashicorp/aws", aws["source"])
	assert.Contains(t, aws["version_constraints"].([]any), ">= 5.0")

	core := data["required_core"].([]any)
	assert.Contains(t, core, ">= 1.5.0")

	// Module calls.
	calls := data["module_calls"].([]any)
	require.Len(t, calls, 1)
	subnet := findByName(calls, "subnet")
	require.NotNil(t, subnet)
	assert.Equal(t, "terraform-aws-modules/vpc/aws", subnet["source"])
	assert.Equal(t, "5.1.0", subnet["version"])

	// Resource footprint.
	managed := data["managed_resources"].([]any)
	require.Len(t, managed, 1)
	assert.Equal(t, "aws_vpc", managed[0].(map[string]any)["type"])
	dataResources := data["data_resources"].([]any)
	require.Len(t, dataResources, 1)
	assert.Equal(t, "aws_ami", dataResources[0].(map[string]any)["type"])

	assert.Equal(t, "introspect", output.Metadata["operation"])
	assert.Equal(t, 3, output.Metadata["inputs"])
	assert.Equal(t, 2, output.Metadata["outputs"])
}

func TestPlugin_Execute_Introspect_MultiFileAggregation(t *testing.T) {
	t.Parallel()
	mockReader := &MockFileReader{
		ReadFileFunc: func(path string) ([]byte, error) {
			files := map[string]string{
				"variables.tf": tfModuleVariables,
				"main.tf":      tfModuleMeta,
			}
			if c, ok := files[filepath.Base(path)]; ok {
				return []byte(c), nil
			}
			return nil, fmt.Errorf("file not found: %s", path)
		},
	}
	p := NewPlugin(WithFileReader(mockReader))
	ctx := context.Background()

	output, err := p.ExecuteProvider(ctx, ProviderName, map[string]any{
		"operation": "introspect",
		"paths":     []any{"./variables.tf", "./main.tf"},
	})
	require.NoError(t, err)

	data := output.Data.(map[string]any)
	// Variables from variables.tf, outputs/providers from main.tf: aggregated.
	assert.Len(t, data["inputs"].([]any), 3)
	assert.Len(t, data["outputs"].([]any), 2)
	assert.Len(t, data["required_providers"].([]any), 1)
}

func TestPlugin_Execute_Introspect_Dir(t *testing.T) {
	t.Parallel()
	mockReader := &MockFileReader{
		ListHCLFilesFunc: func(dir string) ([]string, error) {
			return []string{
				filepath.Join(dir, "variables.tf"),
				filepath.Join(dir, "main.tf"),
			}, nil
		},
		ReadFileFunc: func(path string) ([]byte, error) {
			switch filepath.Base(path) {
			case "variables.tf":
				return []byte(tfModuleVariables), nil
			case "main.tf":
				return []byte(tfModuleMeta), nil
			}
			return nil, fmt.Errorf("file not found: %s", path)
		},
	}
	p := NewPlugin(WithFileReader(mockReader))
	ctx := context.Background()

	output, err := p.ExecuteProvider(ctx, ProviderName, map[string]any{
		"operation": "introspect",
		"dir":       "/modules/vpc",
	})
	require.NoError(t, err)

	data := output.Data.(map[string]any)
	assert.Len(t, data["inputs"].([]any), 3)
	assert.Equal(t, "/modules/vpc", data["path"])
	assert.Equal(t, "/modules/vpc", output.Metadata["path"])
}

func TestPlugin_Execute_Introspect_Deterministic(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := context.Background()

	inputs := map[string]any{
		"operation": "introspect",
		"content": `
variable "zebra" { type = string }
variable "alpha" { type = string }
variable "mike"  { type = string }
`,
	}

	// Repeated runs must yield identically ordered inputs (maps are unordered).
	var first []string
	for i := 0; i < 5; i++ {
		output, err := p.ExecuteProvider(ctx, ProviderName, inputs)
		require.NoError(t, err)
		data := output.Data.(map[string]any)
		var names []string
		for _, it := range data["inputs"].([]any) {
			names = append(names, it.(map[string]any)["name"].(string))
		}
		if i == 0 {
			first = names
			assert.Equal(t, []string{"alpha", "mike", "zebra"}, names)
			continue
		}
		assert.Equal(t, first, names)
	}
}

func TestPlugin_Execute_Introspect_NoVariables(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := context.Background()

	output, err := p.ExecuteProvider(ctx, ProviderName, map[string]any{
		"operation": "introspect",
		"content":   `resource "null_resource" "noop" {}`,
	})
	require.NoError(t, err)

	data := output.Data.(map[string]any)
	assert.Empty(t, data["inputs"].([]any))
	assert.Empty(t, data["outputs"].([]any))
	assert.Len(t, data["managed_resources"].([]any), 1)
}

func TestPlugin_Execute_Introspect_InvalidHCL(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := context.Background()

	_, err := p.ExecuteProvider(ctx, ProviderName, map[string]any{
		"operation": "introspect",
		"content":   `variable "x" { type = `,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "introspect")
}

func TestPlugin_Execute_Introspect_DryRun(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := sdkprovider.WithDryRun(context.Background(), true)

	output, err := p.ExecuteProvider(ctx, ProviderName, map[string]any{
		"operation": "introspect",
		"content":   tfModuleVariables,
	})
	require.NoError(t, err)

	data := output.Data.(map[string]any)
	assert.Empty(t, data["inputs"].([]any))
	assert.Empty(t, data["outputs"].([]any))
	assert.Equal(t, "dry-run", output.Metadata["mode"])
	assert.Equal(t, "introspect", output.Metadata["operation"])
}

func TestPlugin_DescribeWhatIf_Introspect(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := context.Background()

	desc, err := p.DescribeWhatIf(ctx, ProviderName, map[string]any{
		"operation": "introspect",
		"dir":       "./modules/vpc",
	})
	require.NoError(t, err)
	assert.Contains(t, desc, "introspect")
	assert.Contains(t, desc, "./modules/vpc")
}

func TestIntrospectModule_DefaultPath(t *testing.T) {
	t.Parallel()
	result, err := IntrospectModule(
		[]hclSource{{filename: "input.tf", data: []byte(`variable "x" { type = string }`)}},
		"",
	)
	require.NoError(t, err)
	assert.Equal(t, ".", result["path"])
	require.Len(t, result["inputs"].([]any), 1)
}

func TestIntrospectModule_ResourceProviderAlias(t *testing.T) {
	t.Parallel()
	result, err := IntrospectModule(
		[]hclSource{{filename: "input.tf", data: []byte(`
resource "aws_vpc" "west" {
  provider = aws.west
}
`)}},
		"",
	)
	require.NoError(t, err)

	managed := result["managed_resources"].([]any)
	require.Len(t, managed, 1)
	provider := managed[0].(map[string]any)["provider"].(map[string]any)
	assert.Equal(t, "aws", provider["name"])
	assert.Equal(t, "west", provider["alias"])
}

// fakeTree models a small directory hierarchy for introspect-tree tests. Keys
// are absolute directory/file paths (introspect-tree resolves paths before
// walking, so absolute inputs pass through unchanged).
type fakeTree struct {
	subdirs  map[string][]string
	hclFiles map[string][]string
	files    map[string]string
}

func (f fakeTree) reader() *MockFileReader {
	return &MockFileReader{
		ListSubdirsFunc: func(dir string) ([]string, error) {
			return f.subdirs[dir], nil
		},
		ListHCLFilesFunc: func(dir string) ([]string, error) {
			return f.hclFiles[dir], nil
		},
		ReadFileFunc: func(path string) ([]byte, error) {
			if c, ok := f.files[path]; ok {
				return []byte(c), nil
			}
			return nil, fmt.Errorf("file not found: %s", path)
		},
	}
}

func TestIntrospectLibrary_SortedAndShape(t *testing.T) {
	t.Parallel()
	mods := []libModule{
		{relPath: "zebra", sources: []hclSource{{filename: "zebra/main.tf", data: []byte(`variable "z" { type = string }`)}}},
		{relPath: "alpha", sources: []hclSource{{filename: "alpha/main.tf", data: []byte(`variable "a" { type = string }`)}}},
	}

	result, err := IntrospectLibrary("./modules", mods)
	require.NoError(t, err)
	assert.Equal(t, "./modules", result["root"])

	modules := result["modules"].([]any)
	require.Len(t, modules, 2)
	// Entries are sorted by relative path regardless of input order.
	assert.Equal(t, "alpha", modules[0].(map[string]any)["path"])
	assert.Equal(t, "zebra", modules[1].(map[string]any)["path"])
	// Each entry carries the full single-module document shape.
	assert.Contains(t, modules[0].(map[string]any), "inputs")
	assert.Contains(t, modules[0].(map[string]any), "outputs")
	assert.Contains(t, modules[0].(map[string]any), "required_providers")
}

func TestIntrospectLibrary_Empty(t *testing.T) {
	t.Parallel()
	result, err := IntrospectLibrary("", nil)
	require.NoError(t, err)
	assert.Equal(t, ".", result["root"])
	assert.Empty(t, result["modules"].([]any))
	assert.Empty(t, result["diagnostics"].([]any))
}

func TestIntrospectLibrary_ParseErrorFatal(t *testing.T) {
	t.Parallel()
	mods := []libModule{
		{relPath: "broken", sources: []hclSource{{filename: "broken/main.tf", data: []byte(`variable "x" { type = `)}}},
	}
	_, err := IntrospectLibrary("./modules", mods)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken")
}

func TestPlugin_Execute_IntrospectTree_ImmediateChildren(t *testing.T) {
	t.Parallel()
	tree := fakeTree{
		subdirs: map[string][]string{
			"/modules": {"/modules/network", "/modules/database", "/modules/docs"},
		},
		hclFiles: map[string][]string{
			"/modules/network":  {"/modules/network/main.tf"},
			"/modules/database": {"/modules/database/variables.tf"},
			"/modules/docs":     {}, // no HCL -> skipped
		},
		files: map[string]string{
			"/modules/network/main.tf":       tfModuleMeta,
			"/modules/database/variables.tf": tfModuleVariables,
		},
	}
	p := NewPlugin(WithFileReader(tree.reader()))
	ctx := context.Background()

	output, err := p.ExecuteProvider(ctx, ProviderName, map[string]any{
		"operation": "introspect-tree",
		"dir":       "/modules",
	})
	require.NoError(t, err)

	data := output.Data.(map[string]any)
	assert.Equal(t, "/modules", data["root"])
	modules := data["modules"].([]any)
	require.Len(t, modules, 2, "docs has no HCL and is skipped")
	// Deterministically sorted by relative path: database before network.
	assert.Equal(t, "database", modules[0].(map[string]any)["path"])
	assert.Equal(t, "network", modules[1].(map[string]any)["path"])
	assert.Len(t, modules[0].(map[string]any)["inputs"].([]any), 3)

	assert.Equal(t, "introspect-tree", output.Metadata["operation"])
	assert.Equal(t, 2, output.Metadata["modules"])
	assert.Equal(t, 1, output.Metadata["depth"])
}

func TestPlugin_Execute_IntrospectTree_DepthTraversal(t *testing.T) {
	t.Parallel()
	tree := fakeTree{
		subdirs: map[string][]string{
			"/lib":       {"/lib/group"},
			"/lib/group": {"/lib/group/network"},
		},
		hclFiles: map[string][]string{
			"/lib/group":         {}, // intermediate, no HCL of its own
			"/lib/group/network": {"/lib/group/network/main.tf"},
		},
		files: map[string]string{
			"/lib/group/network/main.tf": tfModuleMeta,
		},
	}
	ctx := context.Background()

	// depth 1 does not descend into the nested module.
	p := NewPlugin(WithFileReader(tree.reader()))
	output, err := p.ExecuteProvider(ctx, ProviderName, map[string]any{
		"operation": "introspect-tree",
		"dir":       "/lib",
		"depth":     1,
	})
	require.NoError(t, err)
	assert.Empty(t, output.Data.(map[string]any)["modules"].([]any))

	// depth 2 discovers the nested module with a slash-joined relative path.
	output, err = p.ExecuteProvider(ctx, ProviderName, map[string]any{
		"operation": "introspect-tree",
		"dir":       "/lib",
		"depth":     2,
	})
	require.NoError(t, err)
	modules := output.Data.(map[string]any)["modules"].([]any)
	require.Len(t, modules, 1)
	assert.Equal(t, "group/network", modules[0].(map[string]any)["path"])
	assert.Equal(t, 2, output.Metadata["depth"])
}

func TestPlugin_Execute_IntrospectTree_InvalidDepth(t *testing.T) {
	t.Parallel()
	p := NewPlugin(WithFileReader(&MockFileReader{}))
	ctx := context.Background()

	_, err := p.ExecuteProvider(ctx, ProviderName, map[string]any{
		"operation": "introspect-tree",
		"dir":       "/modules",
		"depth":     0,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "depth")
}

func TestPlugin_Execute_IntrospectTree_AllowMissing(t *testing.T) {
	t.Parallel()
	reader := &MockFileReader{
		ListSubdirsFunc: func(_ string) ([]string, error) {
			return nil, fmt.Errorf("reading directory: %w", fs.ErrNotExist)
		},
	}
	p := NewPlugin(WithFileReader(reader))
	ctx := context.Background()

	output, err := p.ExecuteProvider(ctx, ProviderName, map[string]any{
		"operation":    "introspect-tree",
		"dir":          "/not-yet-materialized",
		"allowMissing": true,
	})
	require.NoError(t, err)

	data := output.Data.(map[string]any)
	assert.Empty(t, data["modules"].([]any))
	assert.Len(t, data["diagnostics"].([]any), 1)
	assert.Equal(t, 0, output.Metadata["modules"])
}

func TestPlugin_Execute_IntrospectTree_MissingFatal(t *testing.T) {
	t.Parallel()
	reader := &MockFileReader{
		ListSubdirsFunc: func(_ string) ([]string, error) {
			return nil, fmt.Errorf("reading directory: %w", fs.ErrNotExist)
		},
	}
	p := NewPlugin(WithFileReader(reader))
	ctx := context.Background()

	_, err := p.ExecuteProvider(ctx, ProviderName, map[string]any{
		"operation": "introspect-tree",
		"dir":       "/missing",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), ProviderName)
}

func TestPlugin_Execute_IntrospectTree_MissingDirInput(t *testing.T) {
	t.Parallel()
	p := NewPlugin(WithFileReader(&MockFileReader{}))
	ctx := context.Background()

	_, err := p.ExecuteProvider(ctx, ProviderName, map[string]any{
		"operation": "introspect-tree",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dir")
}

func TestPlugin_Execute_IntrospectTree_ParseErrorFatal(t *testing.T) {
	t.Parallel()
	tree := fakeTree{
		subdirs: map[string][]string{
			"/modules": {"/modules/broken"},
		},
		hclFiles: map[string][]string{
			"/modules/broken": {"/modules/broken/main.tf"},
		},
		files: map[string]string{
			"/modules/broken/main.tf": `variable "x" { type = `,
		},
	}
	p := NewPlugin(WithFileReader(tree.reader()))
	ctx := context.Background()

	_, err := p.ExecuteProvider(ctx, ProviderName, map[string]any{
		"operation": "introspect-tree",
		"dir":       "/modules",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken")
}

func TestPlugin_Execute_IntrospectTree_DryRun(t *testing.T) {
	t.Parallel()
	p := NewPlugin(WithFileReader(&MockFileReader{}))
	ctx := sdkprovider.WithDryRun(context.Background(), true)

	output, err := p.ExecuteProvider(ctx, ProviderName, map[string]any{
		"operation": "introspect-tree",
		"dir":       "/modules",
	})
	require.NoError(t, err)

	data := output.Data.(map[string]any)
	assert.Empty(t, data["modules"].([]any))
	assert.Equal(t, "dry-run", output.Metadata["mode"])
	assert.Equal(t, "introspect-tree", output.Metadata["operation"])
}

func TestPlugin_DescribeWhatIf_IntrospectTree(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := context.Background()

	desc, err := p.DescribeWhatIf(ctx, ProviderName, map[string]any{
		"operation": "introspect-tree",
		"dir":       "./modules",
		"depth":     2,
	})
	require.NoError(t, err)
	assert.Contains(t, desc, "library")
	assert.Contains(t, desc, "./modules")
	assert.Contains(t, desc, "depth 2")
}

