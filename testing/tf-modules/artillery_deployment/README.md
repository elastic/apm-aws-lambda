<!-- BEGIN_TF_DOCS -->
## Terraform module for deploying artillery

The module is used to deploy and run [artillery](https://www.artillery.io/) in
cloud for benchmarking lambda functions.

## Providers

| Name | Version |
|------|---------|
| <a name="provider_aws"></a> [aws](#provider\_aws) | n/a |
| <a name="provider_null"></a> [null](#provider\_null) | n/a |
| <a name="provider_tls"></a> [tls](#provider\_tls) | n/a |

## Resources

| Name | Type |
|------|------|
| [aws_instance.artillery](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/instance) | resource |
| [aws_internet_gateway.artillery](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/internet_gateway) | resource |
| [aws_key_pair.artillery](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/key_pair) | resource |
| [aws_route.artillery](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route) | resource |
| [aws_security_group.artillery](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group) | resource |
| [aws_subnet.artillery](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/subnet) | resource |
| [aws_vpc.artillery](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc) | resource |
| [null_resource.run_artillery](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [tls_private_key.artillery](https://registry.terraform.io/providers/hashicorp/tls/latest/docs/resources/private_key) | resource |
| [aws_ami.ubuntu](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/ami) | data source |
| [tls_public_key.artillery](https://registry.terraform.io/providers/hashicorp/tls/latest/docs/data-sources/public_key) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_load_arrival_rate"></a> [load\_arrival\_rate](#input\_load\_arrival\_rate) | Rate(per second) at which the virtual users are generated | `number` | `50` | no |
| <a name="input_load_base_url"></a> [load\_base\_url](#input\_load\_base\_url) | Base URL for load generation | `string` | n/a | yes |
| <a name="input_load_duration"></a> [load\_duration](#input\_load\_duration) | Duration over which to generate new virtual users | `number` | `10` | no |
| <a name="input_load_req_path"></a> [load\_req\_path](#input\_load\_req\_path) | Request path for load generation | `string` | n/a | yes |
| <a name="input_machine_type"></a> [machine\_type](#input\_machine\_type) | Machine type for artillery nodes | `string` | `"t2.medium"` | no |
| <a name="input_resource_prefix"></a> [resource\_prefix](#input\_resource\_prefix) | Prefix to add to all created resource | `string` | n/a | yes |
<!-- END_TF_DOCS -->