terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">=4.28.0"
    }
    ec = {
      source  = "elastic/ec"
      version = ">=0.4.1"
    }
  }
}
