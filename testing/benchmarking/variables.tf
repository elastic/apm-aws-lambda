variable "resource_prefix" {
  type        = string
  description = "Prefix to add to all created resource"
}

variable "aws_region" {
  type        = string
  description = "AWS region to deploy lambda function"
  default     = "us-west-2"
}

variable "machine_type" {
  type        = string
  description = "Machine type for artillery nodes"
  default     = "t2.medium"
}

variable "load_duration" {
  type        = number
  description = "Duration over which to generate new virtual users"
  default     = 10
}

variable "load_arrival_rate" {
  type        = number
  description = "Rate(per second) at which the virtual users are generated"
  default     = 50
}

variable "lambda_runtime" {
  type        = string
  description = "The language-specific lambda runtime"
  default     = "python3.8"
}

variable "lambda_timeout" {
  type        = number
  description = "Timeout of the lambda function in seconds"
  default     = 15
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
