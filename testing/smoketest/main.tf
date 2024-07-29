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
  source                   = "github.com/elastic/apm-server//testing/infra/terraform/modules/ec_deployment?depth=1"
  deployment_name_prefix   = "apm-aws-lambda-smoke-testing"
  integrations_server      = true
  elasticsearch_size       = "1g"
  elasticsearch_zone_count = 1
  apm_server_expvar        = false
  apm_server_pprof         = false
  region                   = var.ess_region
  deployment_template      = var.ess_deployment_template
  stack_version            = var.ess_version
  tags                     = module.tags.tags
}

locals {
  runtimeVars = {
    "nodejs" = {
      "source_file" = "./function/index.js"
      "handler"     = "index.handler"
      "runtime"     = "nodejs18.x"
      "agent_layer" = "arn:aws:lambda:${var.aws_region}:267093732750:layer:elastic-apm-node-ver-4-3-0:1"
      "envvars" = {
        "NODE_OPTIONS" = "-r elastic-apm-node/start"
      }
    }
    "python" = {
      "source_file" = "./function/main.py"
      "handler"     = "main.handler"
      "runtime"     = "python3.9"
      "agent_layer" = "arn:aws:lambda:${var.aws_region}:267093732750:layer:elastic-apm-python-ver-6-22-3:1"
      "envvars" = {
        "AWS_LAMBDA_EXEC_WRAPPER" = "/opt/python/bin/elasticapm-lambda"
      }
    }
  }
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
  name_prefix        = "apm-aws-lambda-smoke-testing-iam-role"
  assume_role_policy = data.aws_iam_policy_document.assume_role.json
}

data "archive_file" "lambda" {
  type        = "zip"
  source_file = local.runtimeVars[var.function_runtime]["source_file"]
  output_path = "lambda_function_payload.zip"
}

resource "aws_lambda_function" "test_lambda" {
  filename      = "lambda_function_payload.zip"
  function_name = "${var.user_name}-smoke-testing-test"
  role          = aws_iam_role.lambda.arn
  handler       = local.runtimeVars[var.function_runtime]["handler"]

  runtime = local.runtimeVars[var.function_runtime]["runtime"]

  source_code_hash = data.archive_file.lambda.output_base64sha256

  layers = [
    aws_lambda_layer_version.lambda_layer.arn,
    local.runtimeVars[var.function_runtime]["agent_layer"]
  ]

  environment {
    variables = merge({
      ELASTIC_APM_LOG_LEVEL                       = var.log_level
      ELASTIC_APM_LAMBDA_APM_SERVER               = module.ec_deployment.apm_url
      ELASTIC_APM_SECRETS_MANAGER_SECRET_TOKEN_ID = aws_secretsmanager_secret.apm_secret_token.id
    }, local.runtimeVars[var.function_runtime]["envvars"])
  }

  depends_on = [
    aws_iam_role_policy_attachment.lambda_logs,
    aws_iam_role_policy_attachment.secrets_manager_elastic_apm_policy_attach,
    aws_cloudwatch_log_group.example,
  ]
}

resource "aws_cloudwatch_log_group" "example" {
  name              = "/aws/lambda/${var.user_name}-smoke-testing-test"
  retention_in_days = 1
}

data "aws_iam_policy_document" "lambda_logging" {
  statement {
    effect = "Allow"

    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents",
    ]

    resources = ["arn:aws:logs:*:*:*"]
  }
}

resource "aws_iam_policy" "lambda_logging" {
  name        = "smoketest_extension_lambda_logging"
  path        = "/"
  description = "IAM policy for logging during smoketest for apm aws lambda extension"
  policy      = data.aws_iam_policy_document.lambda_logging.json
}

resource "aws_iam_role_policy_attachment" "lambda_logs" {
  role       = aws_iam_role.lambda.name
  policy_arn = aws_iam_policy.lambda_logging.arn
}

resource "aws_secretsmanager_secret" "apm_secret_token" {
  name_prefix             = "apm-aws-lambda-smoke-testing-secret"
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
  name_prefix = "apm-aws-lambda-smoke-testing-iam-policy"
  description = "Allows the lambda function to access the APM secret token stored in AWS Secrets Manager."
  policy      = data.aws_iam_policy_document.policy.json
}

resource "aws_iam_role_policy_attachment" "secrets_manager_elastic_apm_policy_attach" {
  role       = aws_iam_role.lambda.name
  policy_arn = aws_iam_policy.secrets_manager_elastic_apm_policy.arn
}

locals {
  zip_files = tolist(fileset("../../dist/", "*-linux-amd64.zip"))
}

resource "aws_lambda_layer_version" "lambda_layer" {
  filename   = "../../dist/${local.zip_files[0]}"
  layer_name = "apm-aws-lambda-smoke-testing-lambda_layer_name"

  description = "AWS Lambda Extension Layer for Elastic APM - smoke testing"
}
