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
    ec = {
      source  = "elastic/ec"
      version = ">=0.4.0"
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

provider "ec" {}

provider "aws" {
  region = var.aws_region
}


module "ec_deployment" {
  source = "github.com/elastic/apm-server/testing/infra/terraform/modules/ec_deployment"

  region        = var.ess_region
  stack_version = var.stack_version

  deployment_template    = var.deployment_template
  deployment_name_prefix = "${var.resource_prefix}_aws_lambda_test"

  elasticsearch_size       = var.elasticsearch_size
  elasticsearch_zone_count = var.elasticsearch_zone_count

  integrations_server = true
  apm_server_expvar   = false
  apm_server_pprof    = false
}

module "lambda_deployment" {
  source = "../tf-modules/lambda_deployment"

  resource_prefix = var.resource_prefix

  build_dir              = "../build"
  apm_aws_extension_path = "../../bin/extension.zip"

  lambda_runtime     = var.lambda_runtime
  lambda_handler     = local.runtimeToHandler[var.lambda_runtime]
  lambda_invoke_path = local.load_req_path
  lambda_memory_size = var.lambda_memory_size

  apm_server_url   = module.ec_deployment.apm_url
  apm_secret_token = module.ec_deployment.apm_secret_token
}

module "artillery_deployment" {
  source = "../tf-modules/artillery_deployment"

  depends_on = [
    module.ec_deployment,
    module.lambda_deployment,
  ]

  resource_prefix = var.resource_prefix
  machine_type    = var.machine_type

  load_duration     = var.load_duration
  load_arrival_rate = var.load_arrival_rate
  load_base_url     = module.lambda_deployment.base_url
  load_req_path     = local.load_req_path
}
