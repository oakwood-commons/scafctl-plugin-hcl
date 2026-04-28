// Copyright 2025-2026 Oakwood Commons
// SPDX-License-Identifier: Apache-2.0

package hcl

import (
	"context"
	"path/filepath"
	"testing"

	sdkplugin "github.com/oakwood-commons/scafctl-plugin-sdk/plugin"

	sdkprovider "github.com/oakwood-commons/scafctl-plugin-sdk/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlugin_Descriptor(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	desc, err := p.GetProviderDescriptor(context.Background(), ProviderName)
	require.NoError(t, err)

	assert.Equal(t, "hcl", desc.Name)
	assert.Equal(t, "HCL", desc.DisplayName)
	assert.Equal(t, "v1", desc.APIVersion)
	assert.NotNil(t, desc.Version)
	assert.NotEmpty(t, desc.Description)
	assert.Equal(t, "data", desc.Category)
	assert.True(t, desc.Beta)
	assert.Contains(t, desc.Capabilities, sdkprovider.CapabilityFrom)
	assert.Contains(t, desc.Capabilities, sdkprovider.CapabilityTransform)
	assert.NotEmpty(t, desc.Tags)
	assert.Contains(t, desc.Tags, "hcl")
	assert.Contains(t, desc.Tags, "terraform")
	assert.Contains(t, desc.Tags, "opentofu")
	assert.NotEmpty(t, desc.Schema.Properties)
	assert.NotEmpty(t, desc.Examples)
	assert.NotEmpty(t, desc.Links)
	assert.NotEmpty(t, desc.OutputSchemas)
	assert.NotNil(t, desc.OutputSchemas[sdkprovider.CapabilityFrom])
	assert.NotNil(t, desc.OutputSchemas[sdkprovider.CapabilityTransform])
}

func TestPlugin_GetProviderDescriptor_UnknownProvider(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	_, err := p.GetProviderDescriptor(context.Background(), "unknown")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestPlugin_GetProviders(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	providers, err := p.GetProviders(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{ProviderName}, providers)
}

func TestPlugin_ExecuteProvider_UnknownProvider(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	_, err := p.ExecuteProvider(context.Background(), "unknown", map[string]any{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestPlugin_Execute_WithContent(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := context.Background()

	inputs := map[string]any{
		"content": `
variable "region" {
  type    = string
  default = "us-east-1"
}

resource "aws_instance" "web" {
  ami           = "ami-12345"
  instance_type = "t3.micro"
}
`,
	}

	output, err := p.ExecuteProvider(ctx, ProviderName, inputs)
	require.NoError(t, err)
	require.NotNil(t, output)

	data := output.Data.(map[string]any)
	vars := data["variables"].([]any)
	require.Len(t, vars, 1)
	assert.Equal(t, "region", vars[0].(map[string]any)["name"])

	resources := data["resources"].([]any)
	require.Len(t, resources, 1)
	assert.Equal(t, "aws_instance", resources[0].(map[string]any)["type"])
	assert.Equal(t, "web", resources[0].(map[string]any)["name"])

	assert.Equal(t, "input.tf", output.Metadata["filename"])
	assert.NotZero(t, output.Metadata["bytes"])
}

func TestPlugin_Execute_WithPath(t *testing.T) {
	t.Parallel()
	mockReader := &MockFileReader{
		Content: []byte(`
variable "env" {
  type    = string
  default = "dev"
}
`),
	}

	p := NewPlugin(WithFileReader(mockReader))
	ctx := context.Background()

	inputs := map[string]any{
		"path": "/tmp/test.tf",
	}

	output, err := p.ExecuteProvider(ctx, ProviderName, inputs)
	require.NoError(t, err)
	require.NotNil(t, output)

	data := output.Data.(map[string]any)
	vars := data["variables"].([]any)
	require.Len(t, vars, 1)
	assert.Equal(t, "env", vars[0].(map[string]any)["name"])
	assert.Equal(t, "dev", vars[0].(map[string]any)["default"])

	assert.Equal(t, "/tmp/test.tf", output.Metadata["filename"])
}

func TestPlugin_Execute_MissingInputs(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := context.Background()

	_, err := p.ExecuteProvider(ctx, ProviderName, map[string]any{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "one of 'content', 'path', 'paths', or 'dir' must be provided")
}

func TestPlugin_Execute_BothInputs(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := context.Background()

	inputs := map[string]any{
		"content": `variable "x" {}`,
		"path":    "/tmp/test.tf",
	}

	_, err := p.ExecuteProvider(ctx, ProviderName, inputs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestPlugin_Execute_DryRun(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := sdkprovider.WithDryRun(context.Background(), true)

	inputs := map[string]any{
		"content": `variable "x" {}`,
	}

	output, err := p.ExecuteProvider(ctx, ProviderName, inputs)
	require.NoError(t, err)
	require.NotNil(t, output)

	data := output.Data.(map[string]any)
	assert.Empty(t, data["variables"].([]any))
	assert.Empty(t, data["resources"].([]any))
	assert.Empty(t, data["modules"].([]any))
	assert.Equal(t, "dry-run", output.Metadata["mode"])
}

func TestPlugin_Execute_InvalidHCL(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := context.Background()

	inputs := map[string]any{
		"content": `this is { not valid hcl !!!`,
	}

	_, err := p.ExecuteProvider(ctx, ProviderName, inputs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse HCL")
}

func TestPlugin_Execute_FileReadError(t *testing.T) {
	t.Parallel()
	mockReader := &MockFileReader{ReadFileErr: true}

	p := NewPlugin(WithFileReader(mockReader))
	ctx := context.Background()

	inputs := map[string]any{
		"path": "/nonexistent/file.tf",
	}

	_, err := p.ExecuteProvider(ctx, ProviderName, inputs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestPlugin_Execute_EmptyContent(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := context.Background()

	inputs := map[string]any{
		"content": "",
	}

	output, err := p.ExecuteProvider(ctx, ProviderName, inputs)
	require.NoError(t, err)
	require.NotNil(t, output)

	data := output.Data.(map[string]any)
	assert.Empty(t, data["variables"].([]any))
	assert.Empty(t, data["resources"].([]any))
}

func TestPlugin_Execute_WithFileReaderFunc(t *testing.T) {
	t.Parallel()

	calledWithPath := ""
	mockReader := &MockFileReader{
		ReadFileFunc: func(path string) ([]byte, error) {
			calledWithPath = path
			return []byte(`module "vpc" { source = "./modules/vpc" }`), nil
		},
	}

	p := NewPlugin(WithFileReader(mockReader))
	ctx := context.Background()

	inputs := map[string]any{
		"path": "/my/terraform/main.tf",
	}

	output, err := p.ExecuteProvider(ctx, ProviderName, inputs)
	require.NoError(t, err)
	require.NotNil(t, output)

	assert.Equal(t, "/my/terraform/main.tf", calledWithPath)

	data := output.Data.(map[string]any)
	modules := data["modules"].([]any)
	require.Len(t, modules, 1)
	assert.Equal(t, "vpc", modules[0].(map[string]any)["name"])
	assert.Equal(t, "./modules/vpc", modules[0].(map[string]any)["source"])
}

func TestPlugin_Execute_TransformCapability(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := context.Background()

	inputs := map[string]any{
		"content": `
output "result" {
  value       = "hello"
  description = "A test output"
}
`,
	}

	output, err := p.ExecuteProvider(ctx, ProviderName, inputs)
	require.NoError(t, err)
	require.NotNil(t, output)

	data := output.Data.(map[string]any)
	outputs := data["outputs"].([]any)
	require.Len(t, outputs, 1)
	assert.Equal(t, "result", outputs[0].(map[string]any)["name"])
	assert.Equal(t, "hello", outputs[0].(map[string]any)["value"])
	assert.Equal(t, "A test output", outputs[0].(map[string]any)["description"])
}

func TestPlugin_Execute_Format_UnformattedContent(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := context.Background()

	unformatted := `variable "region" {
type=string
default="us-east-1"
}`

	inputs := map[string]any{
		"operation": "format",
		"content":   unformatted,
	}

	output, err := p.ExecuteProvider(ctx, ProviderName, inputs)
	require.NoError(t, err)
	require.NotNil(t, output)

	data := output.Data.(map[string]any)
	assert.True(t, data["changed"].(bool), "changed should be true for unformatted input")
	formatted := data["formatted"].(string)
	assert.NotEmpty(t, formatted)
	assert.Contains(t, formatted, "type")
	assert.Contains(t, formatted, "region")
	assert.Equal(t, "format", output.Metadata["operation"])
}

func TestPlugin_Execute_Format_AlreadyFormatted(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := context.Background()

	alreadyFormatted := `variable "region" {
  type    = string
  default = "us-east-1"
}
`

	inputs := map[string]any{
		"operation": "format",
		"content":   alreadyFormatted,
	}

	output, err := p.ExecuteProvider(ctx, ProviderName, inputs)
	require.NoError(t, err)
	require.NotNil(t, output)

	data := output.Data.(map[string]any)
	assert.False(t, data["changed"].(bool), "changed should be false when content is already canonical")
	assert.Equal(t, alreadyFormatted, data["formatted"].(string))
}

func TestPlugin_Execute_Format_WithFilePath(t *testing.T) {
	t.Parallel()
	p := NewPlugin(WithFileReader(&MockFileReader{
		Content: []byte(`resource "aws_s3_bucket" "b" {
bucket="my-bucket"
}`),
	}))
	ctx := context.Background()

	inputs := map[string]any{
		"operation": "format",
		"path":      "./main.tf",
	}

	output, err := p.ExecuteProvider(ctx, ProviderName, inputs)
	require.NoError(t, err)
	require.NotNil(t, output)

	data := output.Data.(map[string]any)
	assert.True(t, data["changed"].(bool))
	assert.Contains(t, data["formatted"].(string), "aws_s3_bucket")
	assert.Equal(t, "format", output.Metadata["operation"])
	assert.Equal(t, "main.tf", filepath.Base(output.Metadata["filename"].(string)))
}

func TestPlugin_Execute_Format_DryRun(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := sdkprovider.WithDryRun(context.Background(), true)

	inputs := map[string]any{
		"operation": "format",
		"content":   `variable "x" { type=string }`,
	}

	output, err := p.ExecuteProvider(ctx, ProviderName, inputs)
	require.NoError(t, err)
	require.NotNil(t, output)

	data := output.Data.(map[string]any)
	assert.Equal(t, "", data["formatted"])
	assert.Equal(t, false, data["changed"])
	assert.Equal(t, "dry-run", output.Metadata["mode"])
}

func TestPlugin_Execute_InvalidOperation(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := context.Background()

	inputs := map[string]any{
		"operation": "unknown",
		"content":   `variable "x" {}`,
	}

	_, err := p.ExecuteProvider(ctx, ProviderName, inputs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported operation")
}

func TestPlugin_DescribeWhatIf(t *testing.T) {
	p := NewPlugin()
	ctx := context.Background()

	tests := []struct {
		name     string
		input    map[string]any
		contains string
	}{
		{
			name:     "parse with path",
			input:    map[string]any{"operation": "parse", "path": "/tf/main.tf"},
			contains: "/tf/main.tf",
		},
		{
			name:     "format with dir",
			input:    map[string]any{"operation": "format", "dir": "/tf/"},
			contains: "/tf/",
		},
		{
			name:     "validate with inline content",
			input:    map[string]any{"operation": "validate", "content": `variable "x" {}`},
			contains: "inline content",
		},
		{
			name:     "generate without target",
			input:    map[string]any{"operation": "generate"},
			contains: "generate",
		},
		{
			name:     "default operation (no op field)",
			input:    map[string]any{"path": "/tf/vars.tf"},
			contains: "/tf/vars.tf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := p.DescribeWhatIf(ctx, ProviderName, tt.input)
			require.NoError(t, err)
			if tt.contains != "" {
				assert.Contains(t, msg, tt.contains)
			}
		})
	}
}

func TestPlugin_DescribeWhatIf_UnknownProvider(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	_, err := p.DescribeWhatIf(context.Background(), "unknown", map[string]any{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestPlugin_ConfigureProvider(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	err := p.ConfigureProvider(context.Background(), ProviderName, sdkplugin.ProviderConfig{})
	assert.NoError(t, err)
}

func TestPlugin_StopProvider(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	err := p.StopProvider(context.Background(), ProviderName)
	assert.NoError(t, err)
}

func TestPlugin_ExtractDependencies(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	deps, err := p.ExtractDependencies(context.Background(), ProviderName, map[string]any{})
	require.NoError(t, err)
	assert.Nil(t, deps)
}
