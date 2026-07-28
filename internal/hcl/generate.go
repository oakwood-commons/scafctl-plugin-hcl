// Copyright 2025-2026 Oakwood Commons
// SPDX-License-Identifier: Apache-2.0

package hcl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// GenerateHCL converts a structured map representation into canonical HCL text.
// The input map follows the same schema as ParseHCL output: top-level keys are
// block types (variable, resource, module, output, locals, provider, terraform,
// moved, import, data, check) with arrays of block definitions.
//
// Values follow Terraform's JSON conventions: a string wrapped as "${ ... }" is
// emitted as a bare expression, type constraints and addresses (for example
// variable.type or moved.from) are emitted bare, and every other value is a
// quoted literal.
func GenerateHCL(input map[string]any) (string, error) {
	f := hclwrite.NewEmptyFile()
	body := f.Body()

	// blockOrder lists the input map keys in desired output order.
	// The parse output uses plural keys (variables, resources, ...), so we
	// map each input key to the singular HCL block type for code generation.
	type blockEntry struct {
		inputKey  string // key in the input map (parse output schema)
		blockType string // HCL block type name (singular)
	}
	blockOrder := []blockEntry{
		{"terraform", "terraform"},
		{"variables", "variable"},
		{"locals", "locals"},
		{"data", "data"},
		{"resources", "resource"},
		{"modules", "module"},
		{"outputs", "output"},
		{"providers", "provider"},
		{"moved", "moved"},
		{"import", "import"},
		{"check", "check"},
	}

	needsNewline := false
	for _, entry := range blockOrder {
		val, ok := input[entry.inputKey]
		if !ok {
			continue
		}

		switch entry.inputKey {
		case "terraform":
			m, ok := val.(map[string]any)
			if !ok || len(m) == 0 {
				continue
			}
			if needsNewline {
				body.AppendNewline()
			}
			if err := generateTerraformBlock(body, m); err != nil {
				return "", fmt.Errorf("generating terraform block: %w", err)
			}
			needsNewline = true

		case "locals":
			m, ok := val.(map[string]any)
			if !ok || len(m) == 0 {
				continue
			}
			if needsNewline {
				body.AppendNewline()
			}
			if err := generateLocalsBlock(body, m); err != nil {
				return "", fmt.Errorf("generating locals block: %w", err)
			}
			needsNewline = true

		default:
			items, ok := val.([]any)
			if !ok || len(items) == 0 {
				continue
			}
			for _, item := range items {
				itemMap, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if needsNewline {
					body.AppendNewline()
				}
				if err := generateBlock(body, entry.blockType, itemMap); err != nil {
					return "", fmt.Errorf("generating %s block: %w", entry.blockType, err)
				}
				needsNewline = true
			}
		}
	}

	return string(f.Bytes()), nil
}

// generateBlock creates a single HCL block from a map definition.
func generateBlock(body *hclwrite.Body, blockType string, def map[string]any) error {
	labels := labelsForBlock(blockType, def)
	block := body.AppendNewBlock(blockType, labels)
	blockBody := block.Body()

	// Write promoted attributes first (well-known fields).
	for _, key := range promotedKeysForBlock(blockType) {
		if v, ok := def[key]; ok {
			if err := writePromoted(blockBody, blockType, key, v); err != nil {
				return err
			}
		}
	}

	// Write remaining attributes from the "attributes" map.
	if attrs, ok := def["attributes"].(map[string]any); ok {
		if err := writeAttrs(blockBody, attrs); err != nil {
			return err
		}
	}

	// Write validation sub-blocks (for variable blocks).
	if validations, ok := def["validation"].([]any); ok {
		for _, v := range validations {
			if vm, ok := v.(map[string]any); ok {
				if err := writeAttrs(blockBody.AppendNewBlock("validation", nil).Body(), vm); err != nil {
					return err
				}
			}
		}
	}

	// Write assertions sub-blocks (for check blocks).
	if assertions, ok := def["assertions"].([]any); ok {
		for _, a := range assertions {
			if am, ok := a.(map[string]any); ok {
				if err := writeAttrs(blockBody.AppendNewBlock("assert", nil).Body(), am); err != nil {
					return err
				}
			}
		}
	}

	// Write data sub-blocks (for check blocks).
	if dataBlocks, ok := def["data"].([]any); ok {
		for _, d := range dataBlocks {
			if dm, ok := d.(map[string]any); ok {
				dataLabels := []string{}
				if t, ok := dm["type"].(string); ok {
					dataLabels = append(dataLabels, t)
				}
				if n, ok := dm["name"].(string); ok {
					dataLabels = append(dataLabels, n)
				}
				dBlock := blockBody.AppendNewBlock("data", dataLabels)
				if attrs, ok := dm["attributes"].(map[string]any); ok {
					if err := writeAttrs(dBlock.Body(), attrs); err != nil {
						return err
					}
				}
			}
		}
	}

	// Write generic sub-blocks from "blocks" array.
	if blocks, ok := def["blocks"].([]any); ok {
		for _, b := range blocks {
			bm, ok := b.(map[string]any)
			if !ok {
				continue
			}
			bType, _ := bm["type"].(string)
			var bLabels []string
			if ls, ok := bm["labels"].([]any); ok {
				for _, l := range ls {
					if s, ok := l.(string); ok {
						bLabels = append(bLabels, s)
					}
				}
			}
			subBody := blockBody.AppendNewBlock(bType, bLabels).Body()
			if attrs, ok := bm["attributes"].(map[string]any); ok {
				if err := writeAttrs(subBody, attrs); err != nil {
					return err
				}
			}
			// Recurse into nested blocks.
			if nested, ok := bm["blocks"].([]any); ok {
				for _, nb := range nested {
					if nbm, ok := nb.(map[string]any); ok {
						nbType, _ := nbm["type"].(string)
						if err := generateBlock(subBody, nbType, nbm); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

// bareKind classifies a promoted attribute whose value must be emitted bare
// (a type constraint, address reference, or list of addresses) rather than as a
// quoted string literal.
type bareKind uint8

const (
	bareNone bareKind = iota
	bareType
	bareAddr
	bareAddrList
)

// promotedBareKind reports how a promoted key's value must be rendered. These
// are the schema positions where a value is always a type constraint or a
// static address, never an arbitrary literal.
func promotedBareKind(blockType, key string) bareKind {
	switch blockType {
	case "variable":
		if key == "type" {
			return bareType
		}
	case "moved":
		if key == "from" || key == "to" {
			return bareAddr
		}
	case "import":
		if key == "to" || key == "provider" {
			return bareAddr
		}
	case "module", "output":
		if key == "depends_on" {
			return bareAddrList
		}
	}
	return bareNone
}

// writePromoted writes a promoted attribute, rendering type constraints and
// address references bare (never quoted) and all other values normally. Bare
// positions are schema-defined to be strings, so a non-string value is rejected
// with a clear error rather than silently emitting an invalid quoted literal.
func writePromoted(body *hclwrite.Body, blockType, key string, v any) error {
	switch promotedBareKind(blockType, key) {
	case bareType:
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("attribute %q: type constraint must be a string, got %T", key, v)
		}
		toks, err := typeConstraintTokens(exprSource(s))
		if err != nil {
			return fmt.Errorf("attribute %q: %w", key, err)
		}
		body.SetAttributeRaw(key, toks)
		return nil
	case bareAddr:
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("attribute %q: address must be a string, got %T", key, v)
		}
		toks, err := exprTokens(exprSource(s))
		if err != nil {
			return fmt.Errorf("attribute %q: %w", key, err)
		}
		body.SetAttributeRaw(key, toks)
		return nil
	case bareAddrList:
		switch vv := v.(type) {
		case []any:
			parts := make([]string, 0, len(vv))
			for _, it := range vv {
				s, ok := it.(string)
				if !ok {
					return fmt.Errorf("attribute %q: address entries must be strings, got %T", key, it)
				}
				parts = append(parts, exprSource(s))
			}
			toks, err := exprTokens("[" + strings.Join(parts, ", ") + "]")
			if err != nil {
				return fmt.Errorf("attribute %q: %w", key, err)
			}
			body.SetAttributeRaw(key, toks)
			return nil
		case string:
			// A whole-string expression such as a parsed "${[...]}" list.
			toks, err := exprTokens(exprSource(vv))
			if err != nil {
				return fmt.Errorf("attribute %q: %w", key, err)
			}
			body.SetAttributeRaw(key, toks)
			return nil
		default:
			return fmt.Errorf("attribute %q: must be a list of addresses or an expression, got %T", key, v)
		}
	}
	return writeAttribute(body, key, v)
}

// writeAttrs writes every attribute in sorted key order, returning the first error.
func writeAttrs(body *hclwrite.Body, attrs map[string]any) error {
	for _, k := range sortedKeys(attrs) {
		if err := writeAttribute(body, k, attrs[k]); err != nil {
			return err
		}
	}
	return nil
}

// generateTerraformBlock creates a terraform { ... } block.
func generateTerraformBlock(body *hclwrite.Body, def map[string]any) error {
	blockBody := body.AppendNewBlock("terraform", nil).Body()

	if rv, ok := def["required_version"].(string); ok {
		if err := writeAttribute(blockBody, "required_version", rv); err != nil {
			return err
		}
	}

	if rp, ok := def["required_providers"].(map[string]any); ok && len(rp) > 0 {
		if err := writeAttrs(blockBody.AppendNewBlock("required_providers", nil).Body(), rp); err != nil {
			return err
		}
	}

	if backend, ok := def["backend"].(map[string]any); ok {
		bType, _ := backend["type"].(string)
		labels := []string{}
		if bType != "" {
			labels = append(labels, bType)
		}
		bBlock := blockBody.AppendNewBlock("backend", labels)
		if attrs, ok := backend["attributes"].(map[string]any); ok {
			if err := writeAttrs(bBlock.Body(), attrs); err != nil {
				return err
			}
		}
	}

	if cloud, ok := def["cloud"].(map[string]any); ok && len(cloud) > 0 {
		if err := writeAttrs(blockBody.AppendNewBlock("cloud", nil).Body(), cloud); err != nil {
			return err
		}
	}

	if attrs, ok := def["attributes"].(map[string]any); ok {
		if err := writeAttrs(blockBody, attrs); err != nil {
			return err
		}
	}

	return nil
}

// generateLocalsBlock creates a locals { ... } block.
func generateLocalsBlock(body *hclwrite.Body, locals map[string]any) error {
	return writeAttrs(body.AppendNewBlock("locals", nil).Body(), locals)
}

// writeAttribute writes a key-value attribute to an hclwrite body.
// Values are converted to HCL token representations.
func writeAttribute(body *hclwrite.Body, key string, val any) error {
	tokens, err := valueToTokens(val)
	if err != nil {
		return fmt.Errorf("attribute %q: %w", key, err)
	}
	body.SetAttributeRaw(key, tokens)
	return nil
}

// valueToTokens converts a Go value into hclwrite tokens. Strings that are
// whole-string Terraform interpolations ("${ ... }") are emitted as bare
// expressions; every other string is emitted as a quoted string literal.
func valueToTokens(val any) (hclwrite.Tokens, error) {
	switch v := val.(type) {
	case string:
		if inner, ok := interpolationExpr(v); ok {
			return exprTokens(inner)
		}
		return hclwrite.TokensForValue(ctyStringVal(v)), nil
	case bool:
		return hclwrite.TokensForValue(ctyBoolVal(v)), nil
	case int:
		return hclwrite.TokensForValue(ctyNumberIntVal(int64(v))), nil
	case int64:
		return hclwrite.TokensForValue(ctyNumberIntVal(v)), nil
	case float64:
		return hclwrite.TokensForValue(ctyNumberFloatVal(v)), nil
	case []any:
		return tokensForList(v)
	case map[string]any:
		return tokensForObject(v)
	case nil:
		return hclwrite.TokensForIdentifier("null"), nil
	default:
		return hclwrite.TokensForValue(ctyStringVal(fmt.Sprintf("%v", v))), nil
	}
}

// tokensForList renders a Go slice as HCL list tokens: [a, b, c].
func tokensForList(items []any) (hclwrite.Tokens, error) {
	return exprTokens(valueToHCLSource(items))
}

// tokensForObject renders a Go map as HCL object tokens: { k = v, ... }.
func tokensForObject(m map[string]any) (hclwrite.Tokens, error) {
	return exprTokens(valueToHCLSource(m))
}

// valueToHCLSource renders any Go value as HCL expression source text,
// recursing into lists and objects. Whole-string interpolations become bare
// expressions; every other string becomes a quoted literal.
func valueToHCLSource(val any) string {
	switch v := val.(type) {
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, valueToHCLSource(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		keys := sortedKeys(v)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, objectKey(k)+" = "+valueToHCLSource(v[k]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return valueToHCLString(v)
	}
}

// objectKey renders an object attribute key, quoting it when it is not a valid
// bare HCL identifier.
func objectKey(k string) string {
	if hclsyntax.ValidIdentifier(k) {
		return k
	}
	return quoteHCLString(k)
}

// valueToHCLString renders a scalar Go value as an HCL literal string.
func valueToHCLString(val any) string {
	switch v := val.(type) {
	case string:
		if inner, ok := interpolationExpr(v); ok {
			return inner
		}
		return quoteHCLString(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%g", v)
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%q", fmt.Sprint(v))
	}
}

// quoteHCLString renders s as a quoted HCL string literal, escaping template
// interpolation sequences so a literal "${" or "%{" is not evaluated.
func quoteHCLString(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
		"${", `$${`,
		"%{", `%%{`,
	)
	return `"` + r.Replace(s) + `"`
}

// labelsForBlock returns the appropriate HCL block labels based on block type.
func labelsForBlock(blockType string, def map[string]any) []string {
	switch blockType {
	case "variable", "module", "output", "check":
		if name, ok := def["name"].(string); ok {
			return []string{name}
		}
	case "resource", "data":
		var labels []string
		if t, ok := def["type"].(string); ok {
			labels = append(labels, t)
		}
		if n, ok := def["name"].(string); ok {
			labels = append(labels, n)
		}
		return labels
	case "provider":
		if name, ok := def["name"].(string); ok {
			return []string{name}
		}
	}
	return nil
}

// promotedKeysForBlock returns well-known attribute keys that should be written
// before generic attributes, in order. These are keys that ParseHCL promotes
// out of the raw attributes map.
func promotedKeysForBlock(blockType string) []string {
	switch blockType {
	case "variable":
		return []string{"type", "default", "description", "sensitive", "nullable"}
	case "output":
		return []string{"value", "description", "sensitive", "depends_on"}
	case "module":
		return []string{"source", "version", "count", "for_each", "depends_on", "providers"}
	case "provider":
		return []string{"alias", "region"}
	case "moved":
		return []string{"from", "to"}
	case "import":
		return []string{"to", "id", "provider", "for_each"}
	default:
		return nil
	}
}

// sortedKeys returns the keys of a map in sorted order.
func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
