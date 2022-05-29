resource "aws_iam_role" "role" {
  count = var.instance_count > 0 ? 1 : 0

  name = "packet-to-parquet-${var.env}"

  assume_role_policy = data.aws_iam_policy_document.assume-role.json

  tags = merge({
    Name = "Packet to Parquet Application Role - ${var.env}"
  }, local.common_tags)
}

data "aws_iam_policy_document" "assume-role" {
  statement {
    actions = ["sts:AssumeRole"]
    effect = "Allow"

    principals {
      type = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }

    dynamic "principals" {
      for_each = toset(local.role_principals)
      content {
        type = "AWS"
        identifiers = [principals.key]
      }
    }
  }
}

resource "aws_iam_user_policy" "user-s3-policy" {
  policy = data.aws_iam_policy_document.s3-policy.json
  user = aws_iam_user.user.id
}

resource "aws_iam_user" "user" {
  name = "app-packet-to-parquet-${var.env}"

  tags = merge({
    Name = "Packet to Parquet Application User - ${var.env}"
  }, local.common_tags)
}

resource "aws_iam_role_policy" "role-sqs-policy" {
  count = var.instance_count > 0 && var.enable_queue ? 1 : 0

  policy = data.aws_iam_policy_document.sqs-policy[0].json
  role = aws_iam_role.role[0].id
}

resource "aws_iam_role_policy" "role-s3-policy" {
  count = var.instance_count > 0 ? 1 : 0

  policy = data.aws_iam_policy_document.s3-policy.json
  role = aws_iam_role.role[0].id
}

data "aws_iam_policy_document" "sqs-policy" {
  count = var.enable_queue ? 1 : 0

  # allow fetching and deleting messages off of the queue
  statement {
    actions = ["sqs:GetQueueUrl", "sqs:ReceiveMessage", "sqs:DeleteMessage"]
    effect = "Allow"

    resources = [aws_sqs_queue.queue[0].arn]
  }
}

data "aws_iam_policy_document" "s3-policy" {
  # allow reading from s3
  statement {
    actions = ["s3:ListBucket"]
    effect = "Allow"

    resources = [aws_s3_bucket.bucket.arn]
  }

  statement {
    actions = ["s3:GetObject", "s3:GetObjectVersion"]
    effect = "Allow"

    resources = ["${aws_s3_bucket.bucket.arn}/*"]
  }

  # allow updating in s3
  statement {
    actions = ["s3:DeleteObject", "s3:PutObject"]
    effect = "Allow"

    resources = ["${aws_s3_bucket.bucket.arn}/*"]
  }
}