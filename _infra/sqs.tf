locals {
  queue_name = "packet-to-parquet-${var.env}"
}

resource "aws_sqs_queue" "queue" {
  count = var.enable_queue ? 1 : 0

  name = local.queue_name
  max_message_size = 2048
  message_retention_seconds = 86400 # 1 day
  receive_wait_time_seconds = 10

  policy = data.aws_iam_policy_document.queue-policy.json

  redrive_policy = jsonencode({
    "deadLetterTargetArn": aws_sqs_queue.queue-dl[0].arn
    "maxReceiveCount": 5
  })

  tags = merge({
    Name = "Packet to Parquet - ${var.env}",
    Description = "Pcaps to Convert"
  }, local.common_tags)
}

data "aws_iam_policy_document" "queue-policy" {
  statement {
    effect = "Allow"
    actions = ["sqs:SendMessage"]
    resources = ["arn:aws:sqs:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:${local.queue_name}"]

    condition {
      test = "ArnEquals"
      values = [aws_s3_bucket.bucket.arn]
      variable = "aws:SourceArn"
    }

    principals {
      identifiers = ["*"]
      type = "*"
    }
  }
}

resource "aws_sqs_queue" "queue-dl" {
  count = var.enable_queue ? 1 : 0

  name = "packet-to-parquet-dl-${var.env}"
  max_message_size = 2048
  message_retention_seconds = 432000

  tags = merge({
    Name = "Packet to Parquet DL - ${var.env}",
    Description = "Failed Pcap Conversions"
  }, local.common_tags)
}