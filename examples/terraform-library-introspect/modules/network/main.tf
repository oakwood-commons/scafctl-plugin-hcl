// Fixture module: introspect-tree discovers this directory as the "network"
// module and scaffolds a root module call from its inputs.

variable "cidr_block" {
  type        = string
  description = "CIDR range for the VPC"
}

variable "subnet_count" {
  type    = number
  default = 2
}

output "vpc_id" {
  value = "vpc-example"
}
