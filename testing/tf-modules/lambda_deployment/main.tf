locals {
  name_from_runtime    = replace(var.lambda_runtime, ".", "_")
  lambda_function_path = "${var.build_dir}/${local.name_from_runtime}.zip"
  lambda_function_name = "${var.resource_prefix}_${local.name_from_runtime}_apm_aws_lambda"
}

resource "aws_iam_role" "iam_for_lambda" {
  name = "${var.resource_prefix}_apm_aws_lambda_iam"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "aws_lambda_layer_version" "extn_layer" {
  filename   = var.apm_aws_extension_path
  layer_name = "${var.resource_prefix}_apm_aws_lambda_extn"
}

# TODO: @lahsivjar Add in cloudwatch integration for visualizing logs
resource "aws_lambda_function" "test_fn" {
  filename         = local.lambda_function_path
  function_name    = local.lambda_function_name
  role             = aws_iam_role.iam_for_lambda.arn
  handler          = "main.handler"
  runtime          = var.lambda_runtime
  source_code_hash = filebase64sha256(local.lambda_function_path)
  layers           = [aws_lambda_layer_version.extn_layer.arn]
  timeout          = var.lambda_timeout

  environment {
    variables = {
      ELASTIC_APM_LAMBDA_APM_SERVER   = var.apm_server_url
      ELASTIC_APM_SECRET_TOKEN        = var.apm_secret_token
      ELASTIC_APM_LAMBDA_CAPTURE_LOGS = "true"
    }
  }
}

resource "aws_apigatewayv2_api" "trigger" {
  name          = local.lambda_function_name
  protocol_type = "HTTP"
  description   = "API Gateway to trigger lambda for testing apm-aws-lambda"
}

resource "aws_apigatewayv2_stage" "trigger" {
  api_id = aws_apigatewayv2_api.trigger.id

  name        = "${var.resource_prefix}_apm-aws-lambda-test-tf"
  auto_deploy = true
}

resource "aws_apigatewayv2_integration" "trigger" {
  api_id = aws_apigatewayv2_api.trigger.id

  integration_uri    = aws_lambda_function.test_fn.invoke_arn
  integration_type   = "AWS_PROXY"
  integration_method = "POST"
}

resource "aws_apigatewayv2_route" "trigger" {
  api_id = aws_apigatewayv2_api.trigger.id

  route_key = "GET ${var.lambda_invoke_path}"
  target    = "integrations/${aws_apigatewayv2_integration.trigger.id}"
}

resource "aws_lambda_permission" "trigger" {
  statement_id  = "invoke-from-api-gw"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.test_fn.function_name
  principal     = "apigateway.amazonaws.com"

  source_arn = "${aws_apigatewayv2_api.trigger.execution_arn}/*/*"
}
