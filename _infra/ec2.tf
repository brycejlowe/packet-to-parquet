data "aws_vpc" "selected" {
  count = var.instance_count > 0 ? 1 : 0
  state = "available"

  tags = {
    Name = "test-vpc-us-west-2"
  }
}

data "aws_subnet" "private-dynamic" {
  count = var.instance_count > 0 ? 1 : 0
  vpc_id = data.aws_vpc.selected[0].id
  availability_zone = "us-west-2b"

  tags = {
    "Type" = "private-dynamic"
  }
}

data "aws_security_group" "ingress" {
  count = var.instance_count > 0 ? 1 : 0
  vpc_id = data.aws_vpc.selected[0].id

  name = "base-local"
}

data "aws_security_group" "egress" {
  count = var.instance_count > 0 ? 1 : 0
  vpc_id = data.aws_vpc.selected[0].id

  name = "http-https-egress"
}

resource "aws_instance" "instance" {
  count = var.instance_count

  // we're using ubuntu so we don't have to compile a recent version of tshark
  // it's just available
  ami = "ami-0928f4202481dfdf6"
  instance_type = "c5d.xlarge"

  subnet_id = data.aws_subnet.private-dynamic[0].id

  key_name = var.key_name

  iam_instance_profile = aws_iam_instance_profile.profile[0].id

  vpc_security_group_ids = [
    data.aws_security_group.ingress[0].id,
    data.aws_security_group.egress[0].id
  ]

  tags = merge({
    Name = "brycel-packet-to-parquet"
    Description = "Pcap and Parquet Converter Instance"
  }, local.common_tags)
}

resource "aws_iam_instance_profile" "profile" {
  count = var.instance_count > 0 ? 1 : 0

  name = "packet-to-parquet-role"
  role = aws_iam_role.role[0].name
}