// Package hcl implements the hcl provider plugin for parsing, formatting,
// validating, and generating Terraform/OpenTofu HCL configuration.
package hcl

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/hashicorp/hcl/v2/hclwrite"
	sdkplugin "github.com/oakwood-commons/scafctl-plugin-sdk/plugin"
	sdkprovider "github.com/oakwood-commons/scafctl-plugin-sdk/provider"
	sdkhelper "github.com/oakwood-commons/scafctl-plugin-sdk/provider/schemahelper"
)

const (
	// ProviderName is the unique identifier for this provider.
	ProviderName = "hcl"
)

// FileReader abstracts filesystem access for testability.
type FileReader interface {
	ReadFile(path string) ([]byte, error)
	// ListHCLFiles returns all .tf and .tf.json files in a directory (non-recursive).
	ListHCLFiles(dir string) ([]string, error)
}

// Option is a functional option for configuring the HCL provider.
type Option func(*Provider)

// WithFileReader sets a custom file reader for testing.
func WithFileReader(r FileReader) Option {
	return func(p *Provider) {
		p.fileReader = r
	}
}

// Provider implements the HCL parse/format/validate/generate logic.
type Provider struct {
	fileReader FileReader
}

// Plugin wraps a Provider to implement the ProviderPlugin interface.
type Plugin struct {
	provider *Provider
}

// NewPlugin creates a new Plugin with the given options.
func NewPlugin(opts ...Option) *Plugin {
	p := &Provider{
		fileReader: &osFileReader{},
	}
	for _, opt := range opts {
		opt(p)
	}
	return &Plugin{provider: p}
}

// osFileReader is the default file reader using the OS filesystem.
type osFileReader struct{}

func (r *osFileReader) ReadFile(path string) ([]byte, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", absPath)
	}
	return os.ReadFile(absPath) //nolint:gosec // path is validated above (resolved to abs, stat'd, checked not a dir)
}

func (r *osFileReader) ListHCLFiles(dir string) ([]string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving dir: %w", err)
	}
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".tf") || strings.HasSuffix(name, ".tf.json") {
			files = append(files, filepath.Join(absDir, name))
		}
	}
	return files, nil
}

// Descriptor returns the provider descriptor (convenience for tests).
func (prov *Provider) Descriptor() *sdkprovider.Descriptor {
	return buildDescriptor()
}

func buildDescriptor() *sdkprovider.Descriptor {
	version := semver.MustParse("2.0.0")

	outputSchema := sdkhelper.ObjectSchema(nil, map[string]*jsonschema.Schema{
		"variables": sdkhelper.ArrayProp("Extracted variable blocks"),
		"resources": sdkhelper.ArrayProp("Extracted resource blocks"),
		"data":      sdkhelper.ArrayProp("Extracted data source blocks"),
		"modules":   sdkhelper.ArrayProp("Extracted module blocks"),
		"outputs":   sdkhelper.ArrayProp("Extracted output blocks"),
		"locals":    sdkhelper.AnyProp("Extracted locals as key-value pairs"),
		"providers": sdkhelper.ArrayProp("Extracted provider configuration blocks"),
		"terraform": sdkhelper.AnyProp("Extracted terraform configuration block"),
		"moved":     sdkhelper.ArrayProp("Extracted moved blocks"),
		"import":    sdkhelper.ArrayProp("Extracted import blocks"),
		"check":     sdkhelper.ArrayProp("Extracted check blocks"),
	})

	return &sdkprovider.Descriptor{
		Name:        ProviderName,
		DisplayName: "HCL",
		Description: "Parse, format, validate, and generate Terraform/OpenTofu HCL configuration. Operations: 'parse' extracts structured blocks (variables, resources, data sources, modules, outputs, locals, providers, terraform, moved, import, check) into typed maps; 'format' rewrites content to canonical HCL style; 'validate' checks syntax and returns diagnostics with source positions; 'generate' produces HCL (.tf) or Terraform JSON (.tf.json) from structured block data. Accepts inline content, a single file path, multiple file paths, or a directory of .tf/.tf.json files.",
		APIVersion:  "v1",
		Version:     version,
		Category:    "data",
		Beta:        true,
		Tags:        []string{"hcl", "terraform", "opentofu", "parse", "format", "validate", "generate", "config"},
		Capabilities: []sdkprovider.Capability{
			sdkprovider.CapabilityFrom,
			sdkprovider.CapabilityTransform,
		},
		Schema: sdkhelper.ObjectSchema(nil, map[string]*jsonschema.Schema{
			"operation": sdkhelper.StringProp("Operation to perform: 'parse' (default) extracts structured blocks; 'format' canonically formats; 'validate' checks syntax; 'generate' produces HCL from structured input.",
				sdkhelper.WithEnum("parse", "format", "validate", "generate")),
			"content": sdkhelper.StringProp("Raw HCL content to process. Provide 'content', 'path', 'paths', or 'dir' -- these are mutually exclusive.",
				sdkhelper.WithMaxLength(10485760),
			),
			"path": sdkhelper.StringProp("Path to a single HCL file. Mutually exclusive with 'content', 'paths', and 'dir'.",
				sdkhelper.WithMaxLength(4096),
				sdkhelper.WithExample("./main.tf"),
			),
			"paths": sdkhelper.ArrayProp("Array of HCL file paths to process. Results are merged (parse) or returned per file (format/validate). Mutually exclusive with 'content', 'path', and 'dir'.",
				sdkhelper.WithMaxItems(1000),
				sdkhelper.WithItems(sdkhelper.StringProp("Path to an HCL file")),
			),
			"dir": sdkhelper.StringProp("Directory path. All .tf and .tf.json files in the directory are processed. Mutually exclusive with 'content', 'path', and 'paths'.",
				sdkhelper.WithMaxLength(4096),
				sdkhelper.WithExample("./terraform"),
			),
			"blocks":        sdkhelper.AnyProp("Structured block data for the 'generate' operation. Uses the same schema as parse output: {variables: [...], resources: [...], ...}."),
			"output_format": sdkhelper.StringProp("Output format for the 'generate' operation: 'hcl' (default) produces native HCL syntax (.tf); 'json' produces Terraform JSON syntax (.tf.json).", sdkhelper.WithEnum("hcl", "json")),
		}),
		OutputSchemas: map[sdkprovider.Capability]*jsonschema.Schema{
			sdkprovider.CapabilityFrom:      outputSchema,
			sdkprovider.CapabilityTransform: outputSchema,
		},
		Examples: []sdkprovider.Example{
			{
				Name:        "Parse inline HCL",
				Description: "Parse HCL content provided as a string to extract variable definitions",
				YAML: `name: tf-vars
resolve:
  with:
    - provider: hcl
      inputs:
        content: |
          variable "region" {
            type        = string
            default     = "us-east-1"
            description = "AWS region"
          }`,
			},
			{
				Name:        "Parse HCL file",
				Description: "Read and parse a Terraform configuration file",
				YAML: `name: tf-config
resolve:
  with:
    - provider: hcl
      inputs:
        path: ./main.tf`,
			},
			{
				Name:        "Parse a directory of .tf files",
				Description: "Parse all .tf files in a directory and merge the results",
				YAML: `name: tf-full
resolve:
  with:
    - provider: hcl
      inputs:
        dir: ./terraform`,
			},
			{
				Name:        "Format inline HCL",
				Description: "Canonically format HCL content",
				YAML: `name: tf-fmt
resolve:
  with:
    - provider: hcl
      inputs:
        operation: format
        content: |
          variable "region" {
          type=string
          default="us-east-1"
          }`,
			},
			{
				Name:        "Validate HCL syntax",
				Description: "Check HCL for syntax errors without parsing blocks",
				YAML: `name: tf-validate
resolve:
  with:
    - provider: hcl
      inputs:
        operation: validate
        path: ./main.tf`,
			},
			{
				Name:        "Generate HCL from structured data",
				Description: "Produce HCL text from a map following the parse output schema",
				YAML: `name: tf-gen
resolve:
  with:
    - provider: hcl
      inputs:
        operation: generate
        blocks:
          variables:
            - name: region
              type: string
              default: us-east-1
              description: "AWS region"`,
			},
		},
		Links: []sdkprovider.Link{
			{
				Name: "HCL Language",
				URL:  "https://github.com/hashicorp/hcl",
			},
			{
				Name: "OpenTofu",
				URL:  "https://opentofu.org",
			},
		},
	}
}

// GetProviders returns the list of providers exposed by this plugin.
func (p *Plugin) GetProviders(_ context.Context) ([]string, error) {
	return []string{ProviderName}, nil
}

// GetProviderDescriptor returns the descriptor for the named provider.
func (p *Plugin) GetProviderDescriptor(_ context.Context, providerName string) (*sdkprovider.Descriptor, error) {
	if providerName != ProviderName {
		return nil, fmt.Errorf("unknown provider: %s", providerName)
	}
	return buildDescriptor(), nil
}

// ExecuteProvider executes the named provider with the given input.
func (p *Plugin) ExecuteProvider(ctx context.Context, providerName string, input map[string]any) (*sdkprovider.Output, error) {
	if providerName != ProviderName {
		return nil, fmt.Errorf("unknown provider: %s", providerName)
	}
	return p.provider.execute(ctx, input)
}

// DescribeWhatIf returns a description of what the provider would do.
func (p *Plugin) DescribeWhatIf(_ context.Context, providerName string, input map[string]any) (string, error) {
	if providerName != ProviderName {
		return "", fmt.Errorf("unknown provider: %s", providerName)
	}

	operation, _ := input["operation"].(string)
	if operation == "" {
		operation = "parse"
	}
	var target string
	if pa, ok := input["path"].(string); ok && pa != "" {
		target = pa
	} else if d, ok := input["dir"].(string); ok && d != "" {
		target = d
	} else if _, ok := input["content"].(string); ok {
		target = "inline content"
	}
	if target != "" {
		return fmt.Sprintf("Would %s HCL from %s", operation, target), nil
	}
	return fmt.Sprintf("Would %s HCL", operation), nil
}

// ConfigureProvider stores host-side configuration.
func (p *Plugin) ConfigureProvider(_ context.Context, _ string, _ sdkplugin.ProviderConfig) error {
	return nil
}

// ExecuteProviderStream is not supported.
func (p *Plugin) ExecuteProviderStream(_ context.Context, _ string, _ map[string]any, _ func(sdkplugin.StreamChunk)) error {
	return sdkplugin.ErrStreamingNotSupported
}

// ExtractDependencies returns resolver keys this input depends on.
func (p *Plugin) ExtractDependencies(_ context.Context, _ string, _ map[string]any) ([]string, error) {
	return nil, nil
}

// StopProvider performs cleanup for the named provider.
func (p *Plugin) StopProvider(_ context.Context, _ string) error {
	return nil
}

// execute processes HCL content according to the requested operation.
func (prov *Provider) execute(ctx context.Context, inputs map[string]any) (*sdkprovider.Output, error) {
	lgr := logr.FromContextOrDiscard(ctx)

	lgr.V(1).Info("executing provider", "provider", ProviderName)

	operation := "parse"
	if op, ok := inputs["operation"].(string); ok && op != "" {
		operation = op
	}

	validOps := map[string]bool{"parse": true, "format": true, "validate": true, "generate": true}
	if !validOps[operation] {
		return nil, fmt.Errorf("%s: unsupported operation %q; must be one of: parse, format, validate, generate", ProviderName, operation)
	}

	// Generate uses "blocks" input, not content/path/paths/dir.
	if operation == "generate" {
		return prov.executeGenerate(ctx, lgr, inputs)
	}

	// Resolve source(s) for parse/format/validate.
	sources, err := prov.resolveSources(ctx, inputs)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ProviderName, err)
	}

	if sdkprovider.DryRunFromContext(ctx) {
		return dryRunOutput(operation, sources), nil
	}

	switch operation {
	case "parse":
		return prov.executeParse(lgr, sources)
	case "format":
		return prov.executeFormat(lgr, sources)
	case "validate":
		return prov.executeValidate(lgr, sources)
	default:
		return nil, fmt.Errorf("%s: unhandled operation %q", ProviderName, operation)
	}
}

// hclSource represents a single unit of HCL content to process.
type hclSource struct {
	filename string
	data     []byte
}

// resolveSources resolves the input specification into one or more HCL source units.
func (prov *Provider) resolveSources(ctx context.Context, inputs map[string]any) ([]hclSource, error) {
	content, hasContent := inputs["content"].(string)
	path, hasPath := inputs["path"].(string)
	rawPaths, hasPaths := inputs["paths"]
	dir, hasDir := inputs["dir"].(string)

	set := 0
	if hasContent {
		set++
	}
	if hasPath {
		set++
	}
	if hasPaths {
		set++
	}
	if hasDir {
		set++
	}
	if set == 0 {
		return nil, fmt.Errorf("one of 'content', 'path', 'paths', or 'dir' must be provided")
	}
	if set > 1 {
		return nil, fmt.Errorf("'content', 'path', 'paths', and 'dir' are mutually exclusive")
	}

	switch {
	case hasContent:
		return []hclSource{{filename: "input.tf", data: []byte(content)}}, nil
	case hasPath:
		absPath, resolveErr := resolvePath(ctx, path)
		if resolveErr != nil {
			return nil, fmt.Errorf("resolving path: %w", resolveErr)
		}
		data, err := prov.fileReader.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", path, err)
		}
		return []hclSource{{filename: absPath, data: data}}, nil
	case hasPaths:
		return prov.resolvePathsList(ctx, rawPaths)
	case hasDir:
		return prov.resolveDir(ctx, dir)
	default:
		return nil, fmt.Errorf("one of 'content', 'path', 'paths', or 'dir' must be provided")
	}
}

func (prov *Provider) resolvePathsList(ctx context.Context, rawPaths any) ([]hclSource, error) {
	pathSlice, ok := rawPaths.([]any)
	if !ok {
		return nil, fmt.Errorf("'paths' must be an array of strings")
	}
	if len(pathSlice) == 0 {
		return nil, fmt.Errorf("'paths' array must not be empty")
	}
	var sources []hclSource
	for _, raw := range pathSlice {
		filePath, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("each item in 'paths' must be a string, got %T", raw)
		}
		absPath, resolveErr := resolvePath(ctx, filePath)
		if resolveErr != nil {
			return nil, fmt.Errorf("resolving path %s: %w", filePath, resolveErr)
		}
		data, err := prov.fileReader.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
		}
		sources = append(sources, hclSource{filename: absPath, data: data})
	}
	return sources, nil
}

func (prov *Provider) resolveDir(ctx context.Context, dir string) ([]hclSource, error) {
	absDir, resolveErr := resolvePath(ctx, dir)
	if resolveErr != nil {
		return nil, fmt.Errorf("resolving dir: %w", resolveErr)
	}
	files, err := prov.fileReader.ListHCLFiles(absDir)
	if err != nil {
		return nil, fmt.Errorf("listing directory %s: %w", dir, err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no .tf or .tf.json files found in directory: %s", dir)
	}
	var sources []hclSource
	for _, f := range files {
		data, err := prov.fileReader.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", f, err)
		}
		sources = append(sources, hclSource{filename: f, data: data})
	}
	return sources, nil
}

func (prov *Provider) executeParse(lgr logr.Logger, sources []hclSource) (*sdkprovider.Output, error) {
	merged := emptyParseResult()
	totalBytes := 0
	var filenames []string

	for _, src := range sources {
		lgr.V(1).Info("parsing HCL content", "bytes", len(src.data), "filename", src.filename)
		result, err := ParseHCL(src.data, src.filename)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ProviderName, err)
		}
		mergeParseResults(merged, result)
		totalBytes += len(src.data)
		filenames = append(filenames, src.filename)
	}

	varCount, resCount, modCount := countBlocks(merged)
	lgr.V(1).Info("provider completed", "provider", ProviderName,
		"operation", "parse", "files", len(sources),
		"variables", varCount, "resources", resCount, "modules", modCount,
	)

	meta := map[string]any{
		"operation": "parse",
		"bytes":     totalBytes,
		"files":     len(sources),
	}
	if len(sources) == 1 {
		meta["filename"] = filenames[0]
	} else {
		meta["filenames"] = filenames
	}

	return &sdkprovider.Output{Data: merged, Metadata: meta}, nil
}

func (prov *Provider) executeFormat(lgr logr.Logger, sources []hclSource) (*sdkprovider.Output, error) {
	if len(sources) == 1 {
		src := sources[0]
		lgr.V(1).Info("formatting HCL content", "bytes", len(src.data), "filename", src.filename)
		formatted := hclwrite.Format(src.data)
		changed := !bytes.Equal(src.data, formatted)
		lgr.V(1).Info("provider completed", "provider", ProviderName, "operation", "format", "changed", changed)
		return &sdkprovider.Output{
			Data: map[string]any{"formatted": string(formatted), "changed": changed},
			Metadata: map[string]any{
				"filename": src.filename, "bytes": len(src.data), "operation": "format",
			},
		}, nil
	}

	results := make([]any, 0, len(sources))
	anyChanged := false
	for _, src := range sources {
		lgr.V(1).Info("formatting HCL content", "bytes", len(src.data), "filename", src.filename)
		formatted := hclwrite.Format(src.data)
		changed := !bytes.Equal(src.data, formatted)
		if changed {
			anyChanged = true
		}
		results = append(results, map[string]any{
			"filename": src.filename, "formatted": string(formatted), "changed": changed,
		})
	}

	lgr.V(1).Info("provider completed", "provider", ProviderName, "operation", "format", "files", len(sources), "anyChanged", anyChanged)
	return &sdkprovider.Output{
		Data:     map[string]any{"files": results, "changed": anyChanged},
		Metadata: map[string]any{"operation": "format", "files": len(sources)},
	}, nil
}

func (prov *Provider) executeValidate(lgr logr.Logger, sources []hclSource) (*sdkprovider.Output, error) {
	if len(sources) == 1 {
		src := sources[0]
		lgr.V(1).Info("validating HCL content", "bytes", len(src.data), "filename", src.filename)
		result := ValidateHCL(src.data, src.filename)
		lgr.V(1).Info("provider completed", "provider", ProviderName, "operation", "validate", "valid", result["valid"])
		return &sdkprovider.Output{
			Data: result,
			Metadata: map[string]any{
				"filename": src.filename, "bytes": len(src.data), "operation": "validate",
			},
		}, nil
	}

	results := make([]any, 0, len(sources))
	allValid := true
	totalErrors := 0
	for _, src := range sources {
		lgr.V(1).Info("validating HCL content", "bytes", len(src.data), "filename", src.filename)
		result := ValidateHCL(src.data, src.filename)
		result["filename"] = src.filename
		if valid, ok := result["valid"].(bool); ok && !valid {
			allValid = false
		}
		if ec, ok := result["error_count"].(int); ok {
			totalErrors += ec
		}
		results = append(results, result)
	}

	lgr.V(1).Info("provider completed", "provider", ProviderName, "operation", "validate", "files", len(sources), "allValid", allValid)
	return &sdkprovider.Output{
		Data: map[string]any{
			"valid": allValid, "error_count": totalErrors, "files": results,
		},
		Metadata: map[string]any{"operation": "validate", "files": len(sources)},
	}, nil
}

func (prov *Provider) executeGenerate(ctx context.Context, lgr logr.Logger, inputs map[string]any) (*sdkprovider.Output, error) {
	outputFormat := "hcl"
	if f, ok := inputs["output_format"].(string); ok && f != "" {
		outputFormat = f
	}

	if sdkprovider.DryRunFromContext(ctx) {
		return &sdkprovider.Output{
			Data:     map[string]any{"hcl": ""},
			Metadata: map[string]any{"mode": "dry-run", "operation": "generate", "output_format": outputFormat},
		}, nil
	}

	blocks, ok := inputs["blocks"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: 'blocks' input is required for the generate operation and must be a map", ProviderName)
	}

	lgr.V(1).Info("generating HCL", "provider", ProviderName, "output_format", outputFormat)

	var generated string
	var err error
	switch outputFormat {
	case "json":
		generated, err = GenerateHCLJSON(blocks)
	default:
		generated, err = GenerateHCL(blocks)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ProviderName, err)
	}

	lgr.V(1).Info("provider completed", "provider", ProviderName, "operation", "generate", "output_format", outputFormat, "bytes", len(generated))
	return &sdkprovider.Output{
		Data:     map[string]any{"hcl": generated},
		Metadata: map[string]any{"operation": "generate", "output_format": outputFormat, "bytes": len(generated)},
	}, nil
}

func dryRunOutput(operation string, sources []hclSource) *sdkprovider.Output {
	meta := map[string]any{"mode": "dry-run", "operation": operation}
	switch operation {
	case "format":
		if len(sources) == 1 {
			return &sdkprovider.Output{
				Data: map[string]any{"formatted": "", "changed": false}, Metadata: meta,
			}
		}
		return &sdkprovider.Output{
			Data: map[string]any{"files": []any{}, "changed": false}, Metadata: meta,
		}
	case "validate":
		if len(sources) == 1 {
			return &sdkprovider.Output{
				Data: map[string]any{"valid": true, "error_count": 0, "diagnostics": []any{}}, Metadata: meta,
			}
		}
		return &sdkprovider.Output{
			Data: map[string]any{"valid": true, "error_count": 0, "files": []any{}}, Metadata: meta,
		}
	default:
		return &sdkprovider.Output{Data: emptyParseResult(), Metadata: meta}
	}
}

func resolvePath(ctx context.Context, path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	if cwd, ok := sdkprovider.WorkingDirectoryFromContext(ctx); ok && cwd != "" {
		return filepath.Clean(filepath.Join(cwd, path)), nil
	}
	return filepath.Abs(path)
}

func emptyParseResult() map[string]any {
	return map[string]any{
		"variables": []any{},
		"resources": []any{},
		"data":      []any{},
		"modules":   []any{},
		"outputs":   []any{},
		"locals":    map[string]any{},
		"providers": []any{},
		"terraform": map[string]any{},
		"moved":     []any{},
		"import":    []any{},
		"check":     []any{},
	}
}

func mergeParseResults(target, source map[string]any) {
	arrayKeys := []string{"variables", "resources", "data", "modules", "outputs", "providers", "moved", "import", "check"}
	for _, key := range arrayKeys {
		if srcArr, ok := source[key].([]any); ok && len(srcArr) > 0 {
			if tgtArr, ok := target[key].([]any); ok {
				target[key] = append(tgtArr, srcArr...)
			} else {
				target[key] = srcArr
			}
		}
	}
	if srcLocals, ok := source["locals"].(map[string]any); ok {
		if tgtLocals, ok := target["locals"].(map[string]any); ok {
			for k, v := range srcLocals {
				tgtLocals[k] = v
			}
		}
	}
	if srcTF, ok := source["terraform"].(map[string]any); ok && len(srcTF) > 0 {
		target["terraform"] = srcTF
	}
}

func countBlocks(result map[string]any) (int, int, int) {
	var vc, rc, mc int
	if arr, ok := result["variables"].([]any); ok {
		vc = len(arr)
	}
	if arr, ok := result["resources"].([]any); ok {
		rc = len(arr)
	}
	if arr, ok := result["modules"].([]any); ok {
		mc = len(arr)
	}
	return vc, rc, mc
}
