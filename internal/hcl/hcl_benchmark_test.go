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

const benchSource = `variable "region" {
  type    = string
  default = "us-east-1"
}

variable "tags" {
  type = map(string)
}

resource "aws_instance" "web" {
  ami           = "ami-123"
  instance_type = local.instance_type
  subnet_id     = var.subnet_id

  tags = {
    Name = "web"
    Env  = "prod"
  }
}

output "id" {
  value = aws_instance.web.id
}
`

func BenchmarkParseHCL(b *testing.B) {
	src := []byte(benchSource)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = ParseHCL(src, "bench.tf")
	}
}

func BenchmarkGenerateHCL(b *testing.B) {
	parsed, err := ParseHCL([]byte(benchSource), "bench.tf")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = GenerateHCL(parsed)
	}
}

func BenchmarkGenerateHCLJSON(b *testing.B) {
	parsed, err := ParseHCL([]byte(benchSource), "bench.tf")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = GenerateHCLJSON(parsed)
	}
}
