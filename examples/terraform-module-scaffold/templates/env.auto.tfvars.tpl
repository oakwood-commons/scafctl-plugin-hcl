{{- /* Per-environment tfvars seeded from the module's required inputs. */ -}}
# {{ .environment }} environment values -- fill in required inputs.
{{ range .module.inputs }}
{{- if .required }}
{{ .name }} = null # REQUIRED
{{- end }}
{{- end }}
