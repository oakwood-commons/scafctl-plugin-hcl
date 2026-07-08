// Copyright 2025-2026 Oakwood Commons
// SPDX-License-Identifier: Apache-2.0

package hcl

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestPlugin_Examples_Valid verifies that every descriptor Example is valid YAML
// and that the self-contained (inline content/blocks) examples actually execute
// through the provider without error. Examples that reference the filesystem
// (path/paths/dir) are validated for YAML shape only.
func TestPlugin_Examples_Valid(t *testing.T) {
	t.Parallel()
	p := NewPlugin()
	desc, err := p.GetProviderDescriptor(context.Background(), ProviderName)
	require.NoError(t, err)
	require.NotEmpty(t, desc.Examples)

	for _, ex := range desc.Examples {
		t.Run(ex.Name, func(t *testing.T) {
			t.Parallel()

			var doc struct {
				Resolve struct {
					With []struct {
						Inputs map[string]any `yaml:"inputs"`
					} `yaml:"with"`
				} `yaml:"resolve"`
			}
			require.NoError(t, yaml.Unmarshal([]byte(ex.YAML), &doc),
				"example %q must be valid YAML", ex.Name)
			require.NotEmpty(t, doc.Resolve.With, "example %q must declare resolve.with", ex.Name)

			inputs := doc.Resolve.With[0].Inputs
			require.NotNil(t, inputs, "example %q must declare inputs", ex.Name)

			// Only execute self-contained examples; skip filesystem-backed ones.
			for _, k := range []string{"path", "paths", "dir"} {
				if _, ok := inputs[k]; ok {
					return
				}
			}

			_, err := p.ExecuteProvider(context.Background(), ProviderName, inputs)
			require.NoError(t, err, "example %q inputs should execute cleanly", ex.Name)
		})
	}
}

// TestPlugin_Execute_Validate_Dir_Warnings verifies that deprecation warnings are
// aggregated across files in a directory validate, without affecting validity.
func TestPlugin_Execute_Validate_Dir_Warnings(t *testing.T) {
	t.Parallel()
	mockReader := &MockFileReader{
		DirFiles: []string{"./tf/a.tf", "./tf/b.tf"},
		ReadFileFunc: func(path string) ([]byte, error) {
			files := map[string]string{
				"./tf/a.tf": `variable "x" { type = "string" }`,
				"./tf/b.tf": `variable "y" { type = "number" }`,
			}
			if c, ok := files[path]; ok {
				return []byte(c), nil
			}
			return nil, fmt.Errorf("not found: %s", path)
		},
	}
	p := NewPlugin(WithFileReader(mockReader))

	output, err := p.ExecuteProvider(context.Background(), ProviderName, map[string]any{
		"operation": "validate",
		"dir":       "./tf",
	})
	require.NoError(t, err)

	data := output.Data.(map[string]any)
	assert.True(t, data["valid"].(bool))
	assert.Equal(t, 0, data["error_count"].(int))
	assert.Equal(t, 2, data["warning_count"].(int))
}
