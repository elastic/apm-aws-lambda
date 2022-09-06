provider "aws" {
  region = var.aws_region
}

provider "ec" {}

data "ec_stack" "latest" {
  version_regex = "latest"
  region        = var.ec_region
}

resource "ec_deployment" "ec_aws_lambda_minimal" {
  name                   = "aws-lambda-smoke-testing-deployment"
  region                 = var.ec_region
  version                = data.ec_stack.latest.version
  deployment_template_id = var.ec_deployment_template

  elasticsearch {
    autoscale = "false"

    topology {
      id         = "hot_content"
      size       = "1g"
      zone_count = 1
    }
  }
}

resource "ec_deployment" "basic_datasource" {
  name                   = "aws-lambda-smoke-testing-deployment-data"
  region                 = var.ec_region
  version                = data.ec_stack.latest.version
  deployment_template_id = var.ec_deployment_template

  elasticsearch {
    topology {
      id         = "hot_content"
      size       = "1g"
      zone_count = 1
    }
  }

  kibana {}

  apm {}

  observability {
    deployment_id = ec_deployment.ec_aws_lambda_minimal.id
  }
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
    ELASTIC_APM_LAMBDA_APM_SERVER = ec_deployment.basic_datasource.apm[0].https_endpoint
    ELASTIC_APM_SECRET_TOKEN      = ec_deployment.basic_datasource.apm_secret_token
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
