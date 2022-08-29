provider "aws" {
  region = var.aws_region
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
    "arn:aws:lambda:eu-central-1:267093732750:layer:elastic-apm-node-ver-3-38-0:1",
  ]

  environment_variables = {
    NODE_OPTIONS                  = "-r elastic-apm-node/start"
    ELASTIC_APM_LOG_LEVEL         = var.log_level
    ELASTIC_APM_LAMBDA_APM_SERVER = "CHANGEME"
    ELASTIC_APM_SECRET_TOKEN      = "CHANGEME"
  }

  tags = {
    Name = "my-lambda"
  }
}

module "lambda_layer_local" {
  source = "terraform-aws-modules/lambda/aws"

  create_layer = true

  layer_name          = "apm-lambda-extension-smoke-testing"
  description         = "AWS Lambda Extension Layer for Elastic APM - smoke testing"
  compatible_runtimes = ["nodejs16.x"]

  source_path = "../bin/"
}
