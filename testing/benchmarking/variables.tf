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
  default     = "python3.9"
}

variable "lambda_timeout" {
  type        = number
  description = "Timeout of the lambda function in seconds"
  default     = 15
}

variable "lambda_memory_size" {
  type        = number
  description = "Amount of memory (in MB) the lambda function can use"
  default     = 128
}

variable "lambda_apm_aws_extension_path" {
  type        = string
  description = "Extension path where apm-aws-lambda extension zip is created"
}

variable "custom_lambda_extension_arn" {
  type        = string
  description = "Specific lambda extension to use, will use the latest build if not specified"
  default     = ""
}

variable "ess_region" {
  type        = string
  description = "Optional ESS region where the deployment will be created. Defaults to gcp-us-west2"
  default     = "gcp-us-west2"
}

variable "deployment_template" {
  type        = string
  description = "Optional deployment template. Defaults to the CPU optimized template for GCP"
  default     = "gcp-compute-optimized-v3"
}

variable "stack_version" {
  type        = string
  description = "Optional stack version"
  default     = "latest"
}

variable "elasticsearch_size" {
  type        = string
  description = "Optional Elasticsearch instance size"
  default     = "8g"
}

variable "elasticsearch_zone_count" {
  type        = number
  description = "Optional Elasticsearch zone count"
  default     = 2
}

variable "github_workflow_id" {
  type        = string
  description = "The GitHub Workflow ID"
  default     = "1"
}
