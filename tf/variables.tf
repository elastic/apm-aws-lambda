variable "aws_region" {
  type        = string
  description = "aws region"
  default     = "eu-central-1"
}

variable "user_name" {
  description = "Required username to use for prefixes"
  type        = string
}

variable "log_level" {
  type        = string
  description = "lambda extension log level"
  default     = "trace"
}

variable "ess_region" {
  type        = string
  description = "ess region"
  default     = "gcp-us-west2"
}

variable "ess_deployment_template" {
  type        = string
  description = "Elastic Cloud deployment template"
  default     = "gcp-compute-optimized"
}

variable "ess_version" {
  type        = string
  description = "ess version"
  default     = "8.[0-9]?([0-9]).[0-9]?([0-9])$"
}
