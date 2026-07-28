{{- /*
  Per-module scaffold generated from introspect-tree output.
  Receives: .module (one entry from the modules[] collection) and .moduleSource.
  Declares each discovered input as a root variable and emits a module call
  wiring every input to var.<name>.
*/ -}}
{{- range .module.inputs }}
variable "{{ .name }}" {
  type = {{ if hasKey . "type" }}{{ .type }}{{ else }}string{{ end }}
}
{{ end }}
module "{{ .module.path }}" {
  source = "{{ .moduleSource }}"
{{- range .module.inputs }}
  {{ .name }} = var.{{ .name }}
{{- end }}
}
