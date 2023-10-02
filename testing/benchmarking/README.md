<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.1.8, < 2.0.0 |
| <a name="requirement_aws"></a> [aws](#requirement\_aws) | ~> 4.0 |
| <a name="requirement_ec"></a> [ec](#requirement\_ec) | >=0.4.0 |
| <a name="requirement_null"></a> [null](#requirement\_null) | >=3.1.1 |

## Providers

No providers.

## Modules

| Name | Source | Version |
|------|--------|---------|
| <a name="module_artillery_deployment"></a> [artillery\_deployment](#module\_artillery\_deployment) | ../tf-modules/artillery_deployment | n/a |
| <a name="module_ec_deployment"></a> [ec\_deployment](#module\_ec\_deployment) | github.com/elastic/apm-server/testing/infra/terraform/modules/ec_deployment | n/a |
| <a name="module_lambda_deployment"></a> [lambda\_deployment](#module\_lambda\_deployment) | ../tf-modules/lambda_deployment | n/a |

## Resources

No resources.

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_aws_region"></a> [aws\_region](#input\_aws\_region) | AWS region to deploy lambda function | `string` | `"us-west-2"` | no |
| <a name="input_custom_lambda_extension_arn"></a> [custom\_lambda\_extension\_arn](#input\_custom\_lambda\_extension\_arn) | Specific lambda extension to use, will use the latest build if not specified | `string` | `""` | no |
| <a name="input_deployment_template"></a> [deployment\_template](#input\_deployment\_template) | Optional deployment template. Defaults to the CPU optimized template for GCP | `string` | `"gcp-compute-optimized-v2"` | no |
| <a name="input_elasticsearch_size"></a> [elasticsearch\_size](#input\_elasticsearch\_size) | Optional Elasticsearch instance size | `string` | `"8g"` | no |
| <a name="input_elasticsearch_zone_count"></a> [elasticsearch\_zone\_count](#input\_elasticsearch\_zone\_count) | Optional Elasticsearch zone count | `number` | `2` | no |
| <a name="input_ess_region"></a> [ess\_region](#input\_ess\_region) | Optional ESS region where the deployment will be created. Defaults to gcp-us-west2 | `string` | `"gcp-us-west2"` | no |
| <a name="input_lambda_memory_size"></a> [lambda\_memory\_size](#input\_lambda\_memory\_size) | Amount of memory (in MB) the lambda function can use | `number` | `128` | no |
| <a name="input_lambda_runtime"></a> [lambda\_runtime](#input\_lambda\_runtime) | The language-specific lambda runtime | `string` | `"python3.9"` | no |
| <a name="input_lambda_timeout"></a> [lambda\_timeout](#input\_lambda\_timeout) | Timeout of the lambda function in seconds | `number` | `15` | no |
| <a name="input_load_arrival_rate"></a> [load\_arrival\_rate](#input\_load\_arrival\_rate) | Rate(per second) at which the virtual users are generated | `number` | `50` | no |
| <a name="input_load_duration"></a> [load\_duration](#input\_load\_duration) | Duration over which to generate new virtual users | `number` | `10` | no |
| <a name="input_machine_type"></a> [machine\_type](#input\_machine\_type) | Machine type for artillery nodes | `string` | `"t2.medium"` | no |
| <a name="input_resource_prefix"></a> [resource\_prefix](#input\_resource\_prefix) | Prefix to add to all created resource | `string` | n/a | yes |
| <a name="input_stack_version"></a> [stack\_version](#input\_stack\_version) | Optional stack version | `string` | `"latest"` | no |

## Outputs

No outputs.
<!-- END_TF_DOCS -->
