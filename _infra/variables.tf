variable "common_tags" {
  type = map(string)
  default = {}
}

variable "enable_queue" {
  type = bool
  default = true
}

variable "env" {
  type = string
}

variable "instance_count" {
  type = number
  default = 0
}

variable "key_name" {
  type = string
  default = ""
}

variable "source_role_arn" {
  type = string
  default = ""
}

variable "vpc_name" {
  type = string
  default = ""
}