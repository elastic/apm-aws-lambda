## Terraform module to deploy Lambda functions on cloud

The module is used to deploy lambda functions for supported runtimes on cloud
for the purpose of benchmarking. The lambda function is configured to be
triggered with an no auth API gateway endpoint.


<!-- BEGIN_TF_DOCS -->
## Requirements

No requirements.

## Providers

| Name | Version |
|------|---------|
| <a name="provider_aws"></a> [aws](#provider\_aws) | n/a |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [aws_apigatewayv2_api.trigger](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/apigatewayv2_api) | resource |
| [aws_apigatewayv2_integration.trigger](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/apigatewayv2_integration) | resource |
| [aws_apigatewayv2_route.trigger](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/apigatewayv2_route) | resource |
| [aws_apigatewayv2_stage.trigger](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/apigatewayv2_stage) | resource |
| [aws_iam_role.iam_for_lambda](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_lambda_function.test_fn](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/lambda_function) | resource |
| [aws_lambda_layer_version.extn_layer](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/lambda_layer_version) | resource |
| [aws_lambda_permission.trigger](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/lambda_permission) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_apm_aws_extension_path"></a> [apm\_aws\_extension\_path](#input\_apm\_aws\_extension\_path) | Path to the zip file containing extension code | `string` | n/a | yes |
| <a name="input_apm_secret_token"></a> [apm\_secret\_token](#input\_apm\_secret\_token) | Secret token for auth against the given server URL | `string` | n/a | yes |
| <a name="input_apm_server_url"></a> [apm\_server\_url](#input\_apm\_server\_url) | APM Server URL for sending the generated load | `string` | n/a | yes |
| <a name="input_build_dir"></a> [build\_dir](#input\_build\_dir) | Prefix to add to all created resource | `string` | n/a | yes |
| <a name="input_custom_lambda_extension_arn"></a> [custom\_lambda\_extension\_arn](#input\_custom\_lambda\_extension\_arn) | Specific lambda extension to use, will use the latest build if not specified | `string` | `""` | no |
| <a name="input_lambda_handler"></a> [lambda\_handler](#input\_lambda\_handler) | Entrypoint for the lambda function | `string` | `"main.handler"` | no |
| <a name="input_lambda_invoke_path"></a> [lambda\_invoke\_path](#input\_lambda\_invoke\_path) | Request path to invoke the test lambda function | `string` | `"/test"` | no |
| <a name="input_lambda_memory_size"></a> [lambda\_memory\_size](#input\_lambda\_memory\_size) | Amount of memory (in MB) the lambda function can use | `number` | `128` | no |
| <a name="input_lambda_runtime"></a> [lambda\_runtime](#input\_lambda\_runtime) | The language-specific lambda runtime | `string` | `"python3.9"` | no |
| <a name="input_lambda_timeout"></a> [lambda\_timeout](#input\_lambda\_timeout) | Timeout of the lambda function in seconds | `number` | `15` | no |
| <a name="input_resource_prefix"></a> [resource\_prefix](#input\_resource\_prefix) | Prefix to add to all created resource | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_base_url"></a> [base\_url](#output\_base\_url) | Base URL for triggering the test lambda function |
| <a name="output_trigger_url"></a> [trigger\_url](#output\_trigger\_url) | URL for triggering the test lambda function |
<!-- END_TF_DOCS -->
