// Copyright 2025-2026 Oakwood Commons
// SPDX-License-Identifier: Apache-2.0

package hcl

import (
	"context"
	"fmt"
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
