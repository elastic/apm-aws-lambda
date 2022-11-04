provider "aws" {
  region = var.aws_region
}

resource "tls_private_key" "artillery" {
  algorithm = "RSA"
  rsa_bits  = "4096"
}

data "tls_public_key" "artillery" {
  private_key_openssh = tls_private_key.artillery.private_key_openssh
}

data "aws_ami" "ubuntu" {
  most_recent = true

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-focal-20.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  owners = ["099720109477"] # Canonical
}

resource "aws_vpc" "artillery" {
  cidr_block = "172.16.0.0/28"

  tags = {
    Name = "${var.resource_prefix}_apm_aws_lambda_artillery"
  }
}

resource "aws_subnet" "artillery" {
  vpc_id                  = aws_vpc.artillery.id
  cidr_block              = "172.16.0.0/28"
  map_public_ip_on_launch = true

  tags = {
    Name = "${var.resource_prefix}_apm_aws_lambda_artillery"
  }
}

resource "aws_internet_gateway" "artillery" {
  vpc_id = aws_vpc.artillery.id

  tags = {
    Name = "${var.resource_prefix}_apm_aws_lambda_artillery"
  }
}

resource "aws_route" "artillery" {
  destination_cidr_block = "0.0.0.0/0"
  route_table_id         = aws_vpc.artillery.default_route_table_id
  gateway_id             = aws_internet_gateway.artillery.id
}

resource "aws_security_group" "artillery" {
  name   = "${var.resource_prefix}_apm_aws_lambda_artillery"
  vpc_id = aws_vpc.artillery.id
  egress = [
    {
      description      = "Allow all egress traffic"
      cidr_blocks      = ["0.0.0.0/0"]
      ipv6_cidr_blocks = []
      prefix_list_ids  = []
      security_groups  = []
      from_port        = 0
      to_port          = 0
      protocol         = "-1"
      self             = false
    }
  ]
  ingress = [
    {
      description      = "Allow SSH from all sources"
      cidr_blocks      = ["0.0.0.0/0"]
      ipv6_cidr_blocks = []
      prefix_list_ids  = []
      security_groups  = []
      from_port        = 22
      to_port          = 22
      protocol         = "tcp"
      self             = false
    },
  ]
}

resource "aws_key_pair" "artillery" {
  key_name   = "${var.resource_prefix}_apm_aws_lambda_artillery"
  public_key = data.tls_public_key.artillery.public_key_openssh
}

resource "aws_instance" "artillery" {
  ami                    = data.aws_ami.ubuntu.id
  instance_type          = var.machine_type
  key_name               = aws_key_pair.artillery.key_name
  subnet_id              = aws_subnet.artillery.id
  vpc_security_group_ids = [aws_security_group.artillery.id]

  connection {
    type        = "ssh"
    host        = self.public_ip
    user        = "ubuntu"
    private_key = tls_private_key.artillery.private_key_openssh
    timeout     = "1m"
  }

  provisioner "remote-exec" {
    script = "${path.module}/files/install_artillery.sh"
  }
}

resource "null_resource" "run_artillery" {
  triggers = {
    always_run = "${timestamp()}"
  }

  depends_on = [
    aws_instance.artillery
  ]

  connection {
    type        = "ssh"
    host        = aws_instance.artillery.public_ip
    user        = "ubuntu"
    private_key = tls_private_key.artillery.private_key_openssh
    timeout     = "1m"
  }

  provisioner "file" {
    content = templatefile(
      "${path.module}/files/config.yml.tpl",
      {
        load_base_url     = var.load_base_url
        load_req_path     = var.load_req_path
        load_duration     = var.load_duration
        load_arrival_rate = var.load_arrival_rate
      }
    )
    destination = "config.yml"
  }

  provisioner "file" {
    source      = "${path.module}/files/coldstart-metrics.js"
    destination = "coldstart-metrics.js"
  }

  provisioner "remote-exec" {
    script = "${path.module}/files/run_artillery.sh"
  }
}
