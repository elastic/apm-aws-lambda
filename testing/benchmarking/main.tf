terraform {
  required_version = ">= 1.1.8, < 2.0.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.18.1"
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
  load_req_path        = "/test"
  name_from_runtime    = replace(var.lambda_runtime, ".", "_")
  lambda_function_zip  = "../build/${local.name_from_runtime}.zip"
  lambda_function_name = "${var.resource_prefix}_${local.name_from_runtime}_apm_aws_lambda"
  runtimeVars = {
    "python3.9" = {
      "handler" = "main.handler"
      "layers" = ["arn:aws:lambda:${var.aws_region}:267093732750:layer:elastic-apm-python-ver-6-18-0:1"]
      "envvars" = {
        "AWS_LAMBDA_EXEC_WRAPPER" = "/opt/python/bin/elasticapm-lambda"
      }
    }
    "go1.x" = {
      "handler" = "main"
      "layers" = []
      "envvars" = {}
    }
  }
}

provider "ec" {}

provider "aws" {
  region = var.aws_region
  default_tags {
    tags = module.tags.tags
  }
}

module "tags" {
  source  = "../tf-modules/tags"
  project = "lambda-extension-benchmarks"
  build   = var.github_workflow_id
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

  tags = module.tags.tags
}

module "lambda_deployment" {
  source = "../tf-modules/lambda_deployment"

  resource_prefix = var.resource_prefix

  apm_aws_extension_path = var.lambda_apm_aws_extension_path

  lambda_runtime              = var.lambda_runtime
  lambda_memory_size          = var.lambda_memory_size
  lambda_function_zip         = local.lambda_function_zip
  lambda_function_name        = local.lambda_function_name
  lambda_invoke_path          = local.load_req_path
  additional_lambda_layers    = local.runtimeVars[var.lambda_runtime]["layers"]
  lambda_handler              = local.runtimeVars[var.lambda_runtime]["handler"]
  environment_variables       = local.runtimeVars[var.lambda_runtime]["envvars"]
  custom_lambda_extension_arn = var.custom_lambda_extension_arn

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
