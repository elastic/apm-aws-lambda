variable "resource_prefix" {
  type        = string
  description = "Prefix to add to all created resource"
}

variable "build_dir" {
  type        = string
  description = "Prefix to add to all created resource"
}

variable "apm_aws_extension_path" {
  type        = string
  description = "Path to the zip file containing extension code"
}

variable "lambda_runtime" {
  type        = string
  description = "The language-specific lambda runtime"
  default     = "python3.8"
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

variable "apm_server_url" {
  type        = string
  description = "APM Server URL for sending the generated load"
}

variable "apm_secret_token" {
  type        = string
  description = "Secret token for auth against the given server URL"
  sensitive   = true
}
