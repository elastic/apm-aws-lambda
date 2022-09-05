variable "aws_region" {
  type        = string
  description = "aws region"
  default     = "eu-central-1"
}

variable "log_level" {
  type        = string
  description = "lambda extension log level"
  default     = "trace"
}

variable "ec_region" {
  type        = string
  description = "ec region"
  default     = "aws-eu-central-1"
}

variable "ec_deployment_template" {
  type        = string
  description = "ec deployment template"
  default     = "aws-storage-optimized-v2"
}
