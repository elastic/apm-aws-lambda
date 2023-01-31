provider "aws" {
  region = var.aws_region
}

module "tags" {
  source  = "github.com/elastic/apm-server//testing/infra/terraform/modules/tags?depth=1"
  project = var.user_name
}

module "ec_deployment" {
  source                 = "github.com/elastic/apm-server//testing/infra/terraform/modules/ec_deployment?depth=1"
  deployment_name_prefix = "apm-aws-lambda-smoke-testing"
  integrations_server    = true
  apm_server_expvar      = false
  apm_server_pprof       = false
  region                 = var.ess_region
  deployment_template    = var.ess_deployment_template
  stack_version          = var.ess_version
  tags                   = module.tags.tags
}

module "lambda_function" {
  source = "terraform-aws-modules/lambda/aws"

  function_name = "smoke-testing-test"
  description   = "Example Lambda function for smoke testing"
  handler       = "index.handler"
  runtime       = "nodejs16.x"

  source_path = "../testdata/function/"

  layers = [
    module.lambda_layer_local.lambda_layer_arn,
    "arn:aws:lambda:${var.aws_region}:267093732750:layer:elastic-apm-node-ver-3-38-0:1",
  ]

  environment_variables = {
    NODE_OPTIONS                  = "-r elastic-apm-node/start"
    ELASTIC_APM_LOG_LEVEL         = var.log_level
    ELASTIC_APM_LAMBDA_APM_SERVER = module.ec_deployment.apm_url
    ELASTIC_APM_SECRET_TOKEN      = module.ec_deployment.apm_secret_token
  }

  tags = merge(module.tags.tags, {
    Name = "my-lambda"
  })
}

module "lambda_layer_local" {
  source = "terraform-aws-modules/lambda/aws"

  create_layer = true

  layer_name          = "apm-lambda-extension-smoke-testing"
  description         = "AWS Lambda Extension Layer for Elastic APM - smoke testing"
  compatible_runtimes = ["nodejs16.x"]

  # The path could change when upgrading GoReleaser.
  # https://goreleaser.com/customization/build/#a-note-about-folder-names-inside-dist
  source_path = "../dist/apm-lambda-extension_linux_amd64_v1/"

  tags = module.tags.tags
}
