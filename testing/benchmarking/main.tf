terraform {
  required_version = ">= 1.1.8, < 2.0.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.0"
    }
    null = {
      source  = "hashicorp/null"
      version = ">=3.1.1"
    }
  }
}

locals {
  load_req_path = "/test"
  runtimeToHandler = {
    "python3.8" = "main.handler"
    "go1.x"     = "main"
  }
}

module "lambda_deployment" {
  source = "../tf-modules/lambda_deployment"

  resource_prefix = var.resource_prefix
  aws_region      = var.aws_region

  build_dir              = "../build"
  apm_aws_extension_path = "../../bin/extension.zip"

  lambda_runtime     = var.lambda_runtime
  lambda_handler     = local.runtimeToHandler[var.lambda_runtime]
  lambda_invoke_path = local.load_req_path

  apm_server_url   = var.apm_server_url
  apm_secret_token = var.apm_secret_token
}

module "artillery_deployment" {
  source = "../tf-modules/artillery_deployment"

  resource_prefix = var.resource_prefix
  aws_region      = var.aws_region
  machine_type    = var.machine_type

  load_duration     = var.load_duration
  load_arrival_rate = var.load_arrival_rate
  load_base_url     = module.lambda_deployment.base_url
  load_req_path     = local.load_req_path
}
