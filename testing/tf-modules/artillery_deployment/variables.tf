variable "resource_prefix" {
  type        = string
  description = "Prefix to add to all created resource"
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

variable "load_base_url" {
  type        = string
  description = "Base URL for load generation"
}

variable "load_req_path" {
  type        = string
  description = "Request path for load generation"
}
