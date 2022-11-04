output "base_url" {
  description = "Base URL for triggering the test lambda function"
  value       = aws_apigatewayv2_stage.trigger.invoke_url
}

output "trigger_url" {
  description = "URL for triggering the test lambda function"
  value       = "${aws_apigatewayv2_stage.trigger.invoke_url}${var.lambda_invoke_path}"
}
