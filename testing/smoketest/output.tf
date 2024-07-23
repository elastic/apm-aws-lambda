output "elasticsearch_url" {
  value       = module.ec_deployment.elasticsearch_url
  description = "The secure Elasticsearch URL"
}

output "elasticsearch_username" {
  value       = module.ec_deployment.elasticsearch_username
  description = "The Elasticsearch username"
  sensitive   = true
}

output "elasticsearch_password" {
  value       = module.ec_deployment.elasticsearch_password
  description = "The Elasticsearch password"
  sensitive   = true
}

output "aws_region" {
  value       = var.aws_region
  description = "The AWS region"
}

output "user_name" {
  value = var.user_name
}
