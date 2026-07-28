// Fixture module: introspect-tree discovers this directory as the "database"
// module and scaffolds a root module call from its inputs.

variable "engine" {
  type        = string
  description = "Database engine to provision"
}

variable "instance_class" {
  type    = string
  default = "db.t3.micro"
}

output "endpoint" {
  value = "db.example.internal"
}
