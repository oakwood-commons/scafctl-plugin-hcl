# Example input module used by the terraform-module-scaffold solution.
# The scaffold solution introspects this module and generates per-environment
# files (module call, variables.tf, .auto.tfvars) from its inputs and outputs.

variable "region" {
  type        = string
  description = "AWS region for resources"
}

variable "instance_count" {
  type    = number
  default = 2
}

variable "tags" {
  type        = map(string)
  default     = {}
  description = "Tags applied to all resources"
}
