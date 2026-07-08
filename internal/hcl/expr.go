// Copyright 2025-2026 Oakwood Commons
// SPDX-License-Identifier: Apache-2.0

package hcl

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/typeexpr"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// The plugin represents HCL values in a structured (JSON/YAML) model using
// Terraform's own conventions so the model interoperates with the Terraform CLI
// and other tooling:
//
//   - a value expression is written as a whole-string interpolation "${ ... }"
//   - a type constraint or a static address/reference is written bare
//     (positionally known, e.g. variable.type or moved.from)
//   - everything else is a literal value
//
// This replaces the previous lossy content heuristic (looksLikeExpression),
// which quoted primitive type constraints (type = "string") and mis-handled
// literal strings that happened to look like expressions.

// wrapExpr renders a raw expression source as a Terraform interpolation string.
func wrapExpr(source string) string {
	return "${" + source + "}"
}

// interpolationExpr returns the inner expression source and true when s is a
// whole-string Terraform interpolation such as "${ ... }". Partial interpolation
// (for example "a-${x}") and escaped literals ("$${x}") are treated as literal
// strings, matching Terraform's own JSON semantics.
func interpolationExpr(s string) (string, bool) {
	if len(s) < 3 || !strings.HasPrefix(s, "${") || !strings.HasSuffix(s, "}") {
		return "", false
	}
	inner := s[2 : len(s)-1]
	if !balancedBraces(inner) {
		return "", false
	}
	return inner, true
}

// balancedBraces reports whether every '{' in s is matched by a later '}' and
// the string never closes more braces than it opens. Braces inside double-quoted
// HCL string literals are ignored, so a whole-string interpolation whose
// expression contains a brace in a string (for example replace(x, "}", "")) is
// still recognized. It is used to reject partial interpolations like "${a}-${b}"
// whose inner text is not balanced.
func balancedBraces(s string) bool {
	depth := 0
	inString := false
	escaped := false
	for _, r := range s {
		if escaped {
			escaped = false
			continue
		}
		switch {
		case r == '\\':
			escaped = true
		case r == '"':
			inString = !inString
		case inString:
			// Ignore braces inside string literals.
		case r == '{':
			depth++
		case r == '}':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// exprSource returns the bare expression source for a value that is expected to
// be an expression or address. It accepts either a "${ ... }" interpolation or
// an already-bare source string.
func exprSource(s string) string {
	if inner, ok := interpolationExpr(s); ok {
		return inner
	}
	return s
}

// unwrapExpr strips a "${ ... }" wrapper from a string value, returning the bare
// source. This is used for schema positions that always hold a bare type
// constraint or address (for example variable.type or moved.from). Non-string
// or non-wrapped values are returned unchanged.
func unwrapExpr(v any) any {
	if s, ok := v.(string); ok {
		if inner, ok := interpolationExpr(s); ok {
			return inner
		}
	}
	return v
}

// exprTokens lexes an HCL expression source into hclwrite tokens, validating the
// syntax in the process. It is used to emit expressions, addresses, and type
// constraints verbatim (unquoted) rather than as string literals.
func exprTokens(source string) (hclwrite.Tokens, error) {
	f, diags := hclwrite.ParseConfig([]byte("_ = "+source+"\n"), "expr.hcl", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("invalid HCL expression %q: %s", source, diags.Error())
	}
	attr := f.Body().GetAttribute("_")
	if attr == nil {
		return nil, fmt.Errorf("could not parse HCL expression %q", source)
	}
	return attr.Expr().BuildTokens(nil), nil
}

// typeConstraintTokens validates source as a Terraform type constraint and emits
// it as bare HCL tokens. Rejects quoted or otherwise invalid type constraints.
func typeConstraintTokens(source string) (hclwrite.Tokens, error) {
	expr, diags := hclsyntax.ParseExpression([]byte(source), "type.hcl", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("invalid type constraint %q: %s", source, diags.Error())
	}
	if _, diags := typeexpr.TypeConstraint(expr); diags.HasErrors() {
		return nil, fmt.Errorf("invalid type constraint %q: %s", source, diags.Error())
	}
	return exprTokens(source)
}
