variable "resource_prefix" {
  type        = string
  description = "Prefix to add to all created resource"
}

variable "apm_aws_extension_path" {
  type        = string
  description = "Path to the zip file containing extension code"
}

variable "lambda_function_zip" {
  type        = string
  description = "Path to the zip package containing the lambda function to deploy"
}

variable "lambda_function_name" {
  type        = string
  description = "The name of the lambda function"
}

variable "lambda_runtime" {
  type        = string
  description = "The language-specific lambda runtime"
  default     = "python3.9"
}

variable "lambda_handler" {
  type        = string
  description = "Entrypoint for the lambda function"
  default     = "main.handler"
}

variable "lambda_timeout" {
  type        = number
  description = "Timeout of the lambda function in seconds"
  default     = 15
}

variable "lambda_invoke_path" {
  type        = string
  description = "Request path to invoke the test lambda function"
  default     = "/test"
}

variable "lambda_memory_size" {
  type        = number
  description = "Amount of memory (in MB) the lambda function can use"
  default     = 128
}

variable "custom_lambda_extension_arn" {
  type        = string
  description = "Specific lambda extension to use, will use the latest build if not specified"
  default     = ""
}

variable "additional_lambda_layers" {
  type        = list(string)
  description = "Additional lambda layer ARNs to add to the lambda function"
  default     = []
}

variable "environment_variables" {
  type        = map(string)
  description = "Additional environment variables to add to the lambda function"
  default     = {}
}

variable "apm_server_url" {
  type        = string
  description = "APM Server URL for sending the generated load"
}

variable "apm_secret_token" {
  type        = string
  description = "Secret token for auth against the given server URL"
  sensitive   = true
}
