// Copyright 2025-2026 Oakwood Commons
// SPDX-License-Identifier: Apache-2.0

package hcl

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// ValidateHCL checks HCL content for syntax errors and returns structured
// diagnostic information. It reports validity, error count, and detailed
// diagnostics with severity, summary, detail, and source position. When the
// content parses successfully it also runs style/deprecation lint checks and
// reports them as non-fatal warnings (warnings do not affect validity).
func ValidateHCL(src []byte, filename string) map[string]any {
	if filename == "" {
		filename = "input.tf"
	}

	file, diags := hclsyntax.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})

	result := map[string]any{
		"valid":       !diags.HasErrors(),
		"error_count": countDiagsBySeverity(diags, hcl.DiagError),
		"diagnostics": diagnosticsToSlice(diags),
	}

	warnings := []any{}
	if !diags.HasErrors() {
		if body, ok := file.Body.(*hclsyntax.Body); ok {
			warnings = lintWarnings(body)
		}
	}
	result["warnings"] = warnings
	result["warning_count"] = len(warnings)

	return result
}

// lintWarnings runs non-fatal style/deprecation checks over a parsed body.
func lintWarnings(body *hclsyntax.Body) []any {
	warnings := []any{}
	for _, block := range body.Blocks {
		if block.Type != "variable" {
			continue
		}
		attr, ok := block.Body.Attributes["type"]
		if !ok {
			continue
		}
		// A quoted type constraint (type = "string") parses as a string-literal
		// template expression. Bare constraints (string, list(string), ...) do
		// not. Quoted type constraints were deprecated in Terraform 0.12.
		if te, ok := attr.Expr.(*hclsyntax.TemplateExpr); ok && te.IsStringLiteral() {
			warnings = append(warnings, map[string]any{
				"severity": "warning",
				"summary":  "Quoted type constraints are deprecated",
				"detail":   `Use a bare type constraint (for example type = string) instead of a quoted string (type = "string"). Quoted type constraints were deprecated in Terraform 0.12 and are invalid for complex types such as list(string).`,
				"range":    rangeToMap(attr.Expr.Range()),
			})
		}
	}
	return warnings
}

// countDiagsBySeverity counts diagnostics matching a given severity.
func countDiagsBySeverity(diags hcl.Diagnostics, sev hcl.DiagnosticSeverity) int {
	count := 0
	for _, d := range diags {
		if d.Severity == sev {
			count++
		}
	}
	return count
}

// diagnosticsToSlice converts HCL diagnostics to a JSON-friendly slice.
func diagnosticsToSlice(diags hcl.Diagnostics) []any {
	result := make([]any, 0, len(diags))
	for _, d := range diags {
		entry := map[string]any{
			"severity": severityString(d.Severity),
			"summary":  d.Summary,
		}
		if d.Detail != "" {
			entry["detail"] = d.Detail
		}
		if d.Subject != nil {
			entry["range"] = rangeToMap(*d.Subject)
		}
		result = append(result, entry)
	}
	return result
}

// severityString converts a diagnostic severity to a human-readable string.
func severityString(s hcl.DiagnosticSeverity) string {
	switch s {
	case hcl.DiagError:
		return "error"
	case hcl.DiagWarning:
		return "warning"
	case hcl.DiagInvalid:
		return "invalid"
	default:
		return "unknown"
	}
}

// rangeToMap converts an HCL source range to a map for JSON output.
func rangeToMap(r hcl.Range) map[string]any {
	return map[string]any{
		"filename": r.Filename,
		"start": map[string]any{
			"line":   r.Start.Line,
			"column": r.Start.Column,
			"byte":   r.Start.Byte,
		},
		"end": map[string]any{
			"line":   r.End.Line,
			"column": r.End.Column,
			"byte":   r.End.Byte,
		},
	}
}
