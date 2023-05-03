provider "aws" {
  region = var.aws_region
  default_tags {
    tags = module.tags.tags
  }
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

data "aws_iam_policy_document" "assume_role" {
  statement {
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }

    actions = ["sts:AssumeRole"]
  }
}

resource "aws_iam_role" "lambda" {
  name               = "iam_for_lambda"
  assume_role_policy = data.aws_iam_policy_document.assume_role.json
}

data "archive_file" "lambda" {
  type        = "zip"
  source_file = "../testdata/function/index.js"
  output_path = "lambda_function_payload.zip"
}

resource "aws_lambda_function" "test_lambda" {
  filename      = "lambda_function_payload.zip"
  function_name = "smoke-testing-test"
  role          = aws_iam_role.lambda.arn
  handler       = "index.handler"

  source_code_hash = data.archive_file.lambda.output_base64sha256

  runtime = "nodejs16.x"

  layers = [
    aws_lambda_layer_version.lambda_layer.arn,
    "arn:aws:lambda:${var.aws_region}:267093732750:layer:elastic-apm-node-ver-3-38-0:1",
  ]

  environment {
    variables = {
      NODE_OPTIONS                                = "-r elastic-apm-node/start"
      ELASTIC_APM_LOG_LEVEL                       = var.log_level
      ELASTIC_APM_LAMBDA_APM_SERVER               = module.ec_deployment.apm_url
      ELASTIC_APM_SECRETS_MANAGER_SECRET_TOKEN_ID = aws_secretsmanager_secret.apm_secret_token.id
    }
  }
}

resource "aws_secretsmanager_secret" "apm_secret_token" {
  name                    = "lambda-extension-smoke-testing-secret"
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "apm_secret_token_version" {
  secret_id     = aws_secretsmanager_secret.apm_secret_token.id
  secret_string = module.ec_deployment.apm_secret_token
}

data "aws_iam_policy_document" "policy" {
  statement {
    effect    = "Allow"
    resources = [aws_secretsmanager_secret.apm_secret_token.arn]
    actions   = ["secretsmanager:GetSecretValue"]
  }
}

resource "aws_iam_policy" "secrets_manager_elastic_apm_policy" {
  name        = "secrets_manager_elastic_apm_policy"
  description = "Allows the lambda function to access the APM secret token stored in AWS Secrets Manager."
  policy      = data.aws_iam_policy_document.policy.json
}

resource "aws_iam_role_policy_attachment" "secrets_manager_elastic_apm_policy_attach" {
  role       = aws_iam_role.lambda.name
  policy_arn = aws_iam_policy.secrets_manager_elastic_apm_policy.arn
}

locals {
  zip_files = tolist(fileset("../dist/", "*-linux-amd64.zip"))
}

resource "aws_lambda_layer_version" "lambda_layer" {
  filename   = "../dist/${local.zip_files[0]}"
  layer_name = "lambda_layer_name"

  description         = "AWS Lambda Extension Layer for Elastic APM - smoke testing"
  compatible_runtimes = ["nodejs16.x"]
}
