provider "aws" {
  profile = "sandbox"
  region = "us-west-2"
}

locals {
  env = "sandbox"
}

module "sandbox" {
  source = "../"

  env = local.env
  common_tags = {
    Email = "brycejlowe@gmail.com"
    Description = "Packet/pcap to Parquet Converter"
  }
}

output "role-arn" {
  value = module.sandbox.role-arn
}

output "user-arn" {
  value = module.sandbox.user-arn
}

output "s3-bucket" {
  value = module.sandbox.s3-bucket
}

output "queue-url" {
  value = module.sandbox.queue-url
}
