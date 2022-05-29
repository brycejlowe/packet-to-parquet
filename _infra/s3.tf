resource "aws_s3_bucket" "bucket" {
  bucket = "packet-to-parquet-${var.env}"
  acl = "private"

  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm     = "aws:kms"
        kms_master_key_id = data.aws_kms_key.s3_default.arn
      }
    }
  }

  tags = merge({
    Name = "Packet to Parquet - ${var.env}"
    Description = "Pcap and Parquet Storage"
  }, local.common_tags)
}

resource "aws_s3_bucket_public_access_block" "bucket-block" {
  bucket = aws_s3_bucket.bucket.id

  block_public_acls = true
  block_public_policy = true
  ignore_public_acls = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_ownership_controls" "bucket-ownership" {
  bucket = aws_s3_bucket.bucket.id

  rule {
    object_ownership = "BucketOwnerPreferred"
  }
}

resource "aws_s3_bucket_notification" "bucket-notification" {
  count = var.enable_queue ? 1 : 0

  bucket = aws_s3_bucket.bucket.id
  queue {
    events = ["s3:ObjectCreated:*"]
    queue_arn = aws_sqs_queue.queue[0].arn
    filter_suffix = ".pcap"
  }
}

data "aws_kms_key" "s3_default" {
  key_id = "alias/aws/s3"
}