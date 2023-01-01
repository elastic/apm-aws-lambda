resource "aws_iam_role" "iam_for_lambda" {
  name = "${var.resource_prefix}_apm_aws_lambda_iam"
  tags = var.tags

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
  count            = var.custom_lambda_extension_arn == "" ? 1 : 0
  filename         = var.apm_aws_extension_path
  layer_name       = "${var.resource_prefix}_apm_aws_lambda_extn"
  source_code_hash = filebase64sha256(var.apm_aws_extension_path)
}

resource "aws_iam_role_policy_attachment" "cw" {
  role       = aws_iam_role.iam_for_lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_cloudwatch_log_group" "cw_log_group" {
  name              = "/aws/lambda/${var.lambda_function_name}"
  retention_in_days = 1
  tags              = var.tags
}

resource "aws_lambda_function" "test_fn" {
  filename         = var.lambda_function_zip
  function_name    = var.lambda_function_name
  role             = aws_iam_role.iam_for_lambda.arn
  handler          = var.lambda_handler
  runtime          = var.lambda_runtime
  source_code_hash = filebase64sha256(var.lambda_function_zip)
  layers           = [var.custom_lambda_extension_arn == "" ? aws_lambda_layer_version.extn_layer[0].arn : var.custom_lambda_extension_arn]
  timeout          = var.lambda_timeout
  memory_size      = var.lambda_memory_size
  tags             = var.tags

  depends_on = [
    aws_cloudwatch_log_group.cw_log_group,
  ]

  environment {
    variables = {
      ELASTIC_APM_LAMBDA_APM_SERVER   = var.apm_server_url
      ELASTIC_APM_SECRET_TOKEN        = var.apm_secret_token
      ELASTIC_APM_LAMBDA_CAPTURE_LOGS = "true"
    }
  }
}

resource "aws_apigatewayv2_api" "trigger" {
  name          = var.lambda_function_name
  protocol_type = "HTTP"
  description   = "API Gateway to trigger lambda for testing apm-aws-lambda"
  tags          = var.tags
}

resource "aws_apigatewayv2_stage" "trigger" {
  api_id = aws_apigatewayv2_api.trigger.id

  name        = "${var.resource_prefix}_apm-aws-lambda-test-tf"
  auto_deploy = true
  tags        = var.tags
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
