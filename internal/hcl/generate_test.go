// Copyright 2025-2026 Oakwood Commons
// SPDX-License-Identifier: Apache-2.0

package hcl

import (
	"context"
	"strings"
	"testing"

	sdkprovider "github.com/oakwood-commons/scafctl-plugin-sdk/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateHCL_Variables(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"variables": []any{
			map[string]any{
				"name":        "region",
				"type":        "string",
				"default":     "us-east-1",
				"description": "AWS region",
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, `variable "region"`)
	assert.Contains(t, generated, `"us-east-1"`)
	assert.Contains(t, generated, `"AWS region"`)
}

func TestGenerateHCL_Resources(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"resources": []any{
			map[string]any{
				"type": "aws_instance",
				"name": "web",
				"attributes": map[string]any{
					"ami":           "ami-12345",
					"instance_type": "t3.micro",
				},
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, `resource "aws_instance" "web"`)
	assert.Contains(t, generated, `"ami-12345"`)
	assert.Contains(t, generated, `"t3.micro"`)
}

func TestGenerateHCL_Outputs(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"outputs": []any{
			map[string]any{
				"name":        "result",
				"value":       "var.region",
				"description": "The region",
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, `output "result"`)
	assert.Contains(t, generated, "var.region")
	assert.Contains(t, generated, `"The region"`)
}

func TestGenerateHCL_Locals(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"locals": map[string]any{
			"env":    "prod",
			"region": "us-east-1",
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, "locals")
	assert.Contains(t, generated, `"prod"`)
	assert.Contains(t, generated, `"us-east-1"`)
}

func TestGenerateHCL_Terraform(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"terraform": map[string]any{
			"required_version": ">= 1.0",
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, "terraform")
	assert.Contains(t, generated, `">= 1.0"`)
}

func TestGenerateHCL_Modules(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"modules": []any{
			map[string]any{
				"name":   "vpc",
				"source": "./modules/vpc",
				"attributes": map[string]any{
					"cidr": "10.0.0.0/16",
				},
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, `module "vpc"`)
	assert.Contains(t, generated, `"./modules/vpc"`)
	assert.Contains(t, generated, `"10.0.0.0/16"`)
}

func TestGenerateHCL_DataSources(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"data": []any{
			map[string]any{
				"type": "aws_ami",
				"name": "latest",
				"attributes": map[string]any{
					"most_recent": true,
				},
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, `data "aws_ami" "latest"`)
	assert.Contains(t, generated, "most_recent")
}

func TestGenerateHCL_EmptyInput(t *testing.T) {
	t.Parallel()
	generated, err := GenerateHCL(map[string]any{})
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(generated))
}

func TestGenerateHCL_BoolAndNumericValues(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"variables": []any{
			map[string]any{
				"name":      "enabled",
				"default":   true,
				"sensitive": false,
			},
			map[string]any{
				"name":    "count",
				"default": float64(3),
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, "true")
	assert.Contains(t, generated, `variable "enabled"`)
	assert.Contains(t, generated, `variable "count"`)
}

func TestGenerateHCL_Providers(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"providers": []any{
			map[string]any{
				"name":  "aws",
				"alias": "west",
				"attributes": map[string]any{
					"region": "us-west-2",
				},
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, `provider "aws"`)
	assert.Contains(t, generated, "west")
}

func TestGenerateHCL_Moved(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"moved": []any{
			map[string]any{
				"from": "aws_instance.old",
				"to":   "aws_instance.new",
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, "moved")
	assert.Contains(t, generated, "aws_instance.old")
	assert.Contains(t, generated, "aws_instance.new")
}

func TestGenerateHCL_Import(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"import": []any{
			map[string]any{
				"to": "aws_instance.web",
				"id": "i-12345",
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, "import")
	assert.Contains(t, generated, "aws_instance.web")
	assert.Contains(t, generated, `"i-12345"`)
}

func TestGenerateHCL_MultipleBlockTypes(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"variables": []any{
			map[string]any{"name": "env", "type": "string"},
		},
		"resources": []any{
			map[string]any{"type": "aws_instance", "name": "web", "attributes": map[string]any{"ami": "ami-123"}},
		},
		"outputs": []any{
			map[string]any{"name": "id", "value": "aws_instance.web.id"},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)

	varIdx := strings.Index(generated, `variable "env"`)
	resIdx := strings.Index(generated, `resource "aws_instance" "web"`)
	outIdx := strings.Index(generated, `output "id"`)
	assert.Greater(t, resIdx, varIdx, "resources should come after variables")
	assert.Greater(t, outIdx, resIdx, "outputs should come after resources")
}

func TestPlugin_Execute_Generate(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := context.Background()

	inputs := map[string]any{
		"operation": "generate",
		"blocks": map[string]any{
			"variables": []any{
				map[string]any{
					"name":    "region",
					"type":    "string",
					"default": "us-east-1",
				},
			},
		},
	}

	output, err := p.ExecuteProvider(ctx, ProviderName, inputs)
	require.NoError(t, err)
	require.NotNil(t, output)

	data := output.Data.(map[string]any)
	generated := data["hcl"].(string)
	assert.Contains(t, generated, `variable "region"`)
	assert.Contains(t, generated, `"us-east-1"`)
	assert.Equal(t, "generate", output.Metadata["operation"])
	assert.NotZero(t, output.Metadata["bytes"])
}

func TestPlugin_Execute_Generate_DryRun(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := sdkprovider.WithDryRun(context.Background(), true)

	inputs := map[string]any{
		"operation": "generate",
		"blocks": map[string]any{
			"variables": []any{
				map[string]any{"name": "x", "type": "string"},
			},
		},
	}

	output, err := p.ExecuteProvider(ctx, ProviderName, inputs)
	require.NoError(t, err)
	require.NotNil(t, output)

	data := output.Data.(map[string]any)
	assert.Equal(t, "", data["hcl"])
	assert.Equal(t, "dry-run", output.Metadata["mode"])
}

func TestPlugin_Execute_Generate_MissingBlocks(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	ctx := context.Background()

	inputs := map[string]any{
		"operation": "generate",
	}

	_, err := p.ExecuteProvider(ctx, ProviderName, inputs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocks")
}

func TestGenerateHCL_Expressions(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"outputs": []any{
			map[string]any{
				"name":  "id",
				"value": "var.region",
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, "var.region")
	assert.NotContains(t, generated, `"var.region"`)
}

func TestGenerateHCL_NilValue(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"variables": []any{
			map[string]any{
				"name":    "optional",
				"default": nil,
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, "null")
}

func TestGenerateHCL_ListAttribute(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"resources": []any{
			map[string]any{
				"type": "aws_security_group",
				"name": "web",
				"attributes": map[string]any{
					"ingress_ports": []any{float64(80), float64(443)},
				},
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, "ingress_ports")
	assert.Contains(t, generated, "80")
	assert.Contains(t, generated, "443")
}

func TestGenerateHCL_MapAttribute(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"resources": []any{
			map[string]any{
				"type": "aws_instance",
				"name": "web",
				"attributes": map[string]any{
					"tags": map[string]any{
						"Name": "web-server",
						"Env":  "prod",
					},
				},
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, "tags")
	assert.Contains(t, generated, `"web-server"`)
	assert.Contains(t, generated, `"prod"`)
}

func TestGenerateHCL_VariableWithValidation(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"variables": []any{
			map[string]any{
				"name":        "region",
				"type":        "string",
				"description": "AWS region",
				"validation": []any{
					map[string]any{
						"condition":     `can(regex("^us-", var.region))`,
						"error_message": "Region must start with us-",
					},
				},
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, "validation")
	assert.Contains(t, generated, "condition")
	assert.Contains(t, generated, "error_message")
}

func TestGenerateHCL_TerraformInvalidValue(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"terraform": "not-a-map",
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(generated))
}

func TestGenerateHCL_TerraformThenVariables_NeedNewline(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"terraform": map[string]any{"required_version": ">= 1.0"},
		"variables": []any{
			map[string]any{"name": "env", "type": "string"},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, "terraform")
	assert.Contains(t, generated, `variable "env"`)
}

func TestGenerateHCL_LocalsInvalidValue(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"locals": "not-a-map",
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(generated))
}

func TestGenerateHCL_BlockTypeNonSlice(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"variables": "not-a-slice",
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(generated))
}

func TestGenerateHCL_BlockTypeNonMapItem(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"variables": []any{
			"not-a-map-item",
			map[string]any{"name": "valid", "type": "string"},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, `variable "valid"`)
}

func TestGenerateHCL_CheckBlockWithAssertions(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"check": []any{
			map[string]any{
				"name": "health_check",
				"assertions": []any{
					map[string]any{
						"condition":     "true",
						"error_message": "must be true",
					},
				},
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, "check")
	assert.Contains(t, generated, "assert")
	assert.Contains(t, generated, "error_message")
}

func TestGenerateHCL_GenericSubBlocks(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"resources": []any{
			map[string]any{
				"type": "aws_security_group",
				"name": "web",
				"blocks": []any{
					map[string]any{
						"type": "ingress",
						"attributes": map[string]any{
							"from_port": float64(80),
							"to_port":   float64(80),
							"protocol":  "tcp",
						},
					},
				},
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, "ingress")
	assert.Contains(t, generated, "from_port")
}

func TestGenerateHCL_NestedSubBlocks(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"resources": []any{
			map[string]any{
				"type": "aws_lb",
				"name": "main",
				"blocks": []any{
					map[string]any{
						"type": "access_logs",
						"blocks": []any{
							map[string]any{
								"type": "s3_config",
								"attributes": map[string]any{
									"bucket": "my-bucket",
								},
							},
						},
					},
				},
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, "access_logs")
}

func TestValueToHCLString_Types(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		val      any
		contains string
	}{
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"int", int(42), "42"},
		{"int64", int64(100), "100"},
		{"float64 whole", float64(7), "7"},
		{"float64 fractional", float64(3.14), "3.14"},
		{"nil", nil, "null"},
		{"default (other type)", []string{"a", "b"}, "["},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := valueToHCLString(tt.val)
			assert.Contains(t, result, tt.contains)
		})
	}
}

func TestGenerateHCL_BlocksWithLabels(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"resources": []any{
			map[string]any{
				"type": "aws_instance",
				"name": "web",
				"blocks": []any{
					map[string]any{
						"type":   "ephemeral_block_device",
						"labels": []any{"device"},
						"attributes": map[string]any{
							"device_name": "/dev/xvda",
						},
					},
				},
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, "ephemeral_block_device")
}

func TestCtyNumberIntVal(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"variables": []any{
			map[string]any{
				"name":    "count",
				"default": int(5),
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, "5")
}

func TestGenerateHCL_Int64Value(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"variables": []any{
			map[string]any{
				"name":    "port",
				"default": int64(8080),
			},
		},
	}
	generated, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, generated, "8080")
}
