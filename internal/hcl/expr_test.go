// Copyright 2025-2026 Oakwood Commons
// SPDX-License-Identifier: Apache-2.0

package hcl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateHCL_TypeConstraints verifies that variable type constraints are
// emitted bare (not quoted) in HCL and as plain strings in Terraform JSON, for
// both primitive and complex types. This is the fix for the quoted-type defect.
func TestGenerateHCL_TypeConstraints(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		typ  string
	}{
		{"string", "string"},
		{"number", "number"},
		{"bool", "bool"},
		{"any", "any"},
		{"list", "list(string)"},
		{"map", "map(string)"},
		{"set", "set(number)"},
		{"object", "object({ name = string })"},
		{"tuple", "tuple([string, number])"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			input := map[string]any{
				"variables": []any{
					map[string]any{"name": "v", "type": tt.typ},
				},
			}

			hclOut, err := GenerateHCL(input)
			require.NoError(t, err)
			assert.Contains(t, hclOut, "type = "+tt.typ)
			assert.NotContains(t, hclOut, `type = "`, "type constraint must not be quoted in HCL")

			jsonOut, err := GenerateHCLJSON(input)
			require.NoError(t, err)
			assert.Contains(t, jsonOut, `"type": "`+tt.typ+`"`)
		})
	}
}

// TestGenerateHCL_LiteralLooksLikeExpression verifies that a genuine string
// literal whose content resembles an expression is still emitted quoted, rather
// than being misinterpreted as an expression (the inverse of the old heuristic).
func TestGenerateHCL_LiteralLooksLikeExpression(t *testing.T) {
	t.Parallel()
	tests := []string{"merge(a, b)", "var.not_a_ref", "a ? b : c"}
	for _, lit := range tests {
		t.Run(lit, func(t *testing.T) {
			t.Parallel()
			input := map[string]any{
				"variables": []any{
					map[string]any{"name": "v", "default": lit},
				},
			}
			out, err := GenerateHCL(input)
			require.NoError(t, err)
			assert.Contains(t, out, `default = "`+lit+`"`)
		})
	}
}

// TestGenerateHCL_Interpolation verifies that a whole-string interpolation is
// emitted as a bare expression in HCL and preserved as ${...} in Terraform JSON.
func TestGenerateHCL_Interpolation(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"outputs": []any{
			map[string]any{"name": "o", "value": "${local.subnet_id}"},
		},
	}
	hclOut, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, hclOut, "value = local.subnet_id")
	assert.NotContains(t, hclOut, `"local.subnet_id"`)

	jsonOut, err := GenerateHCLJSON(input)
	require.NoError(t, err)
	assert.Contains(t, jsonOut, `"value": "${local.subnet_id}"`)
}

// TestGenerateHCL_Addresses verifies that moved/import address positions render
// bare in HCL and as plain (non-interpolated) strings in Terraform JSON.
func TestGenerateHCL_Addresses(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"moved": []any{
			map[string]any{"from": "aws_instance.a", "to": "aws_instance.b"},
		},
	}
	hclOut, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, hclOut, "from = aws_instance.a")
	assert.Contains(t, hclOut, "to   = aws_instance.b")
	assert.NotContains(t, hclOut, `"aws_instance.a"`)
}

// TestGenerateHCL_InvalidExpressionErrors verifies that malformed expressions
// and invalid type constraints are rejected with an error rather than emitted.
func TestGenerateHCL_InvalidExpressionErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input map[string]any
	}{
		{
			name: "malformed interpolation",
			input: map[string]any{
				"outputs": []any{map[string]any{"name": "o", "value": "${1 +}"}},
			},
		},
		{
			name: "invalid type constraint",
			input: map[string]any{
				"variables": []any{map[string]any{"name": "v", "type": "not_a_real_type"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := GenerateHCL(tt.input)
			require.Error(t, err)
		})
	}
}

// TestRoundTrip_ParseGenerate verifies that parsing HCL and regenerating it
// preserves the literal/expression/type distinction losslessly.
func TestRoundTrip_ParseGenerate(t *testing.T) {
	t.Parallel()
	src := `variable "region" {
  type    = string
  default = "us-east-1"
}

output "subnet" {
  value = local.subnet_id
}
`
	parsed, err := ParseHCL([]byte(src), "in.tf")
	require.NoError(t, err)
	out, err := GenerateHCL(parsed)
	require.NoError(t, err)

	assert.Contains(t, out, "type    = string")
	assert.Contains(t, out, `default = "us-east-1"`)
	assert.Contains(t, out, "value = local.subnet_id")
	assert.NotContains(t, out, `type = "string"`)
	assert.NotContains(t, out, `"local.subnet_id"`)
	// Regenerated output must be parseable again.
	require.False(t, strings.Contains(out, `"string"`))
}

// TestGenerateHCL_DependsOn verifies that a depends_on address list is emitted
// with bare addresses (not quoted string literals) in both HCL and JSON, and
// that a non-string entry is rejected.
func TestGenerateHCL_DependsOn(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"modules": []any{
			map[string]any{
				"name":       "m",
				"source":     "./m",
				"depends_on": []any{"module.a", "aws_instance.b"},
			},
		},
	}
	hclOut, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, hclOut, "depends_on = [module.a, aws_instance.b]")
	assert.NotContains(t, hclOut, `"module.a"`)

	jsonOut, err := GenerateHCLJSON(input)
	require.NoError(t, err)
	assert.Contains(t, jsonOut, `"depends_on": [`)
	assert.Contains(t, jsonOut, `"module.a"`)

	// A non-string address entry is rejected.
	bad := map[string]any{
		"outputs": []any{
			map[string]any{"name": "o", "value": "x", "depends_on": []any{float64(1)}},
		},
	}
	_, err = GenerateHCL(bad)
	require.Error(t, err)
}

// TestGenerateHCL_QuotedObjectKey verifies that object keys which are not valid
// bare HCL identifiers are quoted, while valid identifiers (including HCL's
// dash-containing identifiers) are left bare.
func TestGenerateHCL_QuotedObjectKey(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"resources": []any{
			map[string]any{
				"type": "aws_instance",
				"name": "web",
				"attributes": map[string]any{
					"tags": map[string]any{
						"needs quote": "x", // space -> not a valid identifier
						"with-dash":   "y", // valid HCL identifier
						"valid":       "z",
					},
				},
			},
		},
	}
	out, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, out, `"needs quote" = "x"`)
	assert.Contains(t, out, `with-dash = "y"`)
	assert.Contains(t, out, `valid = "z"`)
	assert.NotContains(t, out, `"valid" =`)
}

// TestGenerateHCL_ImportProvider verifies that import.provider renders as a bare
// provider reference (not a quoted string), matching Terraform's import block.
func TestGenerateHCL_ImportProvider(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"import": []any{
			map[string]any{
				"to":       "aws_instance.web",
				"id":       "i-1234567890abcdef0",
				"provider": "aws.west",
			},
		},
	}
	out, err := GenerateHCL(input)
	require.NoError(t, err)
	assert.Contains(t, out, "provider = aws.west")
	assert.NotContains(t, out, `"aws.west"`)
	assert.NotContains(t, out, `"aws_instance.web"`)
	assert.Contains(t, out, `"i-1234567890abcdef0"`)
}

// TestRoundTrip_ExpressionWithBraceInString verifies that a whole-string
// interpolation whose expression contains a brace inside a string literal is
// preserved as a bare expression (not escaped into a literal).
func TestRoundTrip_ExpressionWithBraceInString(t *testing.T) {
	t.Parallel()
	src := "output \"o\" {\n  value = replace(var.name, \"}\", \"\")\n}\n"
	parsed, err := ParseHCL([]byte(src), "in.tf")
	require.NoError(t, err)
	out, err := GenerateHCL(parsed)
	require.NoError(t, err)
	assert.Contains(t, out, `value = replace(var.name, "}", "")`)
	assert.NotContains(t, out, "$${")
}

// TestRoundTrip_DependsOnJSONArray verifies that a depends_on list of addresses
// is parsed into a structured []any and emitted as a Terraform JSON array (not a
// single interpolation string) and as a bare HCL list.
func TestRoundTrip_DependsOnJSONArray(t *testing.T) {
	t.Parallel()
	src := "module \"m\" {\n  source     = \"./m\"\n  depends_on = [module.a, aws_instance.b]\n}\n"
	parsed, err := ParseHCL([]byte(src), "in.tf")
	require.NoError(t, err)

	mods := parsed["modules"].([]any)
	m := mods[0].(map[string]any)
	dep, ok := m["depends_on"].([]any)
	require.True(t, ok, "depends_on should be a structured list")
	assert.Equal(t, []any{"module.a", "aws_instance.b"}, dep)

	jsonOut, err := GenerateHCLJSON(parsed)
	require.NoError(t, err)
	assert.Contains(t, jsonOut, `"depends_on": [`)
	assert.Contains(t, jsonOut, `"module.a"`)
	assert.NotContains(t, jsonOut, `"${[`)

	hclOut, err := GenerateHCL(parsed)
	require.NoError(t, err)
	assert.Contains(t, hclOut, "depends_on = [module.a, aws_instance.b]")
}

// TestGenerateHCL_BarePromotedNonStringErrors verifies that a non-string value in
// a bare type/address position is rejected with an error rather than silently
// emitting an invalid quoted literal.
func TestGenerateHCL_BarePromotedNonStringErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input map[string]any
	}{
		{
			name:  "variable.type non-string",
			input: map[string]any{"variables": []any{map[string]any{"name": "x", "type": float64(1)}}},
		},
		{
			name:  "moved.from non-string",
			input: map[string]any{"moved": []any{map[string]any{"from": float64(1), "to": "aws_instance.b"}}},
		},
		{
			name:  "import.provider non-string",
			input: map[string]any{"import": []any{map[string]any{"to": "aws_instance.web", "provider": float64(1)}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := GenerateHCL(tt.input)
			require.Error(t, err)
		})
	}
}
