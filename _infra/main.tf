locals {
  common_tags = merge({
    "Environment": var.env
  }, var.common_tags)

  role_principals = compact([var.source_role_arn])
}

data "aws_region" "current" {}
data "aws_caller_identity" "current" {}

output "role-arn" {
  value = var.instance_count > 0 ? aws_iam_role.role[0].arn : ""
}

output "user-arn" {
  value = aws_iam_user.user.arn
}

output "s3-bucket" {
  value = aws_s3_bucket.bucket.arn
}

output "queue-arn" {
  value = var.enable_queue ? aws_sqs_queue.queue[0].arn : ""
}

output "queue-url" {
  value = var.enable_queue ? aws_sqs_queue.queue[0].id : ""
}

output "queue-dl-arn" {
  value = var.enable_queue ? aws_sqs_queue.queue-dl[0].arn : ""
}

output "queue-dl-url" {
  value = var.enable_queue ? aws_sqs_queue.queue-dl[0].id : ""
}