// Copyright 2025-2026 Oakwood Commons
// SPDX-License-Identifier: Apache-2.0

package hcl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
)

// IntrospectModule performs module-level introspection over one or more HCL
// sources, returning a structured document describing the module's inputs,
// outputs, provider requirements, module calls, and resource footprint.
//
// Unlike ParseHCL, which reports the raw block structure of a single file,
// IntrospectModule aggregates across all sources the way Terraform tooling
// does: variables are classified as required or optional, descriptions and
// types are normalized, and provider and core requirements are collected. It
// uses HashiCorp's terraform-config-inspect (tfconfig) -- the same library the
// Terraform Registry uses -- so the result stays compatible with configurations
// targeting different Terraform versions.
//
// modulePath is recorded on the output as the module's location; it is
// informational only and defaults to "." when empty.
func IntrospectModule(sources []hclSource, modulePath string) (map[string]any, error) {
	if modulePath == "" {
		modulePath = "."
	}

	mod := tfconfig.NewModule(modulePath)
	parser := hclparse.NewParser()
	var diags hcl.Diagnostics

	for _, src := range sources {
		var file *hcl.File
		var fileDiags hcl.Diagnostics
		if strings.HasSuffix(src.filename, ".json") {
			file, fileDiags = parser.ParseJSON(src.data, src.filename)
		} else {
			file, fileDiags = parser.ParseHCL(src.data, src.filename)
		}
		diags = append(diags, fileDiags...)
		if file == nil {
			continue
		}
		diags = append(diags, tfconfig.LoadModuleFromFile(file, mod)...)
	}

	// Introspection cannot be trusted when the configuration failed to parse,
	// so error-level diagnostics are fatal -- mirroring the parse operation.
	// Warnings are non-fatal and surfaced on the successful result.
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to introspect module: %s", diags.Error())
	}

	return moduleToMap(mod, diags), nil
}

// moduleToMap converts a tfconfig.Module into a deterministic, JSON-friendly
// document. Map-backed collections are sorted by key so the output is stable
// across runs (tfconfig stores variables, outputs, and so on in Go maps).
func moduleToMap(mod *tfconfig.Module, diags hcl.Diagnostics) map[string]any {
	inputs := make([]any, 0, len(mod.Variables))
	for _, name := range sortedKeys(mod.Variables) {
		inputs = append(inputs, variableToMap(mod.Variables[name]))
	}

	outputs := make([]any, 0, len(mod.Outputs))
	for _, name := range sortedKeys(mod.Outputs) {
		outputs = append(outputs, outputToMap(mod.Outputs[name]))
	}

	requiredProviders := make([]any, 0, len(mod.RequiredProviders))
	for _, name := range sortedKeys(mod.RequiredProviders) {
		requiredProviders = append(requiredProviders, providerRequirementToMap(name, mod.RequiredProviders[name]))
	}

	moduleCalls := make([]any, 0, len(mod.ModuleCalls))
	for _, name := range sortedKeys(mod.ModuleCalls) {
		moduleCalls = append(moduleCalls, moduleCallToMap(mod.ModuleCalls[name]))
	}

	managed := make([]any, 0, len(mod.ManagedResources))
	for _, key := range sortedKeys(mod.ManagedResources) {
		managed = append(managed, resourceToMap(mod.ManagedResources[key]))
	}

	dataResources := make([]any, 0, len(mod.DataResources))
	for _, key := range sortedKeys(mod.DataResources) {
		dataResources = append(dataResources, resourceToMap(mod.DataResources[key]))
	}

	requiredCore := make([]any, 0, len(mod.RequiredCore))
	coreConstraints := append([]string(nil), mod.RequiredCore...)
	sort.Strings(coreConstraints)
	for _, c := range coreConstraints {
		requiredCore = append(requiredCore, c)
	}

	return map[string]any{
		"path":               mod.Path,
		"inputs":             inputs,
		"outputs":            outputs,
		"required_providers": requiredProviders,
		"required_core":      requiredCore,
		"module_calls":       moduleCalls,
		"managed_resources":  managed,
		"data_resources":     dataResources,
		"diagnostics":        diagnosticsToSlice(diags),
	}
}

// variableToMap renders a module input variable. The "default" key is always
// present (null for required variables) so templates can rely on its existence.
func variableToMap(v *tfconfig.Variable) map[string]any {
	entry := map[string]any{
		"name":     v.Name,
		"required": v.Required,
		"default":  v.Default,
		"pos":      posToMap(v.Pos),
	}
	if v.Type != "" {
		entry["type"] = v.Type
	}
	if v.Description != "" {
		entry["description"] = v.Description
	}
	if v.Sensitive {
		entry["sensitive"] = true
	}
	if v.Deprecated != "" {
		entry["deprecated"] = v.Deprecated
	}
	return entry
}

func outputToMap(o *tfconfig.Output) map[string]any {
	entry := map[string]any{
		"name": o.Name,
		"pos":  posToMap(o.Pos),
	}
	if o.Type != "" {
		entry["type"] = o.Type
	}
	if o.Description != "" {
		entry["description"] = o.Description
	}
	if o.Sensitive {
		entry["sensitive"] = true
	}
	if o.Deprecated != "" {
		entry["deprecated"] = o.Deprecated
	}
	return entry
}

func providerRequirementToMap(name string, rp *tfconfig.ProviderRequirement) map[string]any {
	entry := map[string]any{"name": name}
	if rp.Source != "" {
		entry["source"] = rp.Source
	}
	if len(rp.VersionConstraints) > 0 {
		constraints := make([]any, 0, len(rp.VersionConstraints))
		sortedConstraints := append([]string(nil), rp.VersionConstraints...)
		sort.Strings(sortedConstraints)
		for _, c := range sortedConstraints {
			constraints = append(constraints, c)
		}
		entry["version_constraints"] = constraints
	}
	return entry
}

func moduleCallToMap(mc *tfconfig.ModuleCall) map[string]any {
	entry := map[string]any{
		"name":   mc.Name,
		"source": mc.Source,
		"pos":    posToMap(mc.Pos),
	}
	if mc.Version != "" {
		entry["version"] = mc.Version
	}
	return entry
}

func resourceToMap(r *tfconfig.Resource) map[string]any {
	entry := map[string]any{
		"type": r.Type,
		"name": r.Name,
		"pos":  posToMap(r.Pos),
	}
	if r.Provider.Name != "" {
		provider := map[string]any{"name": r.Provider.Name}
		if r.Provider.Alias != "" {
			provider["alias"] = r.Provider.Alias
		}
		entry["provider"] = provider
	}
	return entry
}

func posToMap(p tfconfig.SourcePos) map[string]any {
	return map[string]any{
		"filename": p.Filename,
		"line":     p.Line,
	}
}
