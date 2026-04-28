// Copyright 2025-2026 Oakwood Commons
// SPDX-License-Identifier: Apache-2.0

package hcl

import (
	"context"
	"testing"

	sdkprovider "github.com/oakwood-commons/scafctl-plugin-sdk/provider"
)

func BenchmarkPlugin_Execute_DryRun(b *testing.B) {
	p := NewPlugin()

	ctx := sdkprovider.WithDryRun(context.Background(), true)
	inputs := map[string]any{
		"content": `variable "name" { default = "test" }`,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = p.ExecuteProvider(ctx, ProviderName, inputs)
	}
}
