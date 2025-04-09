<!-- BEGIN_TF_DOCS -->
## Terraform module to tag cloud resources

The module is used to enforce consistent tags on cloud resources

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_build"></a> [build](#input\_build) | The value to use for the build tag/label | `string` | `"unknown"` | no |
| <a name="input_project"></a> [project](#input\_project) | The value to use for the project tag/label | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_labels"></a> [labels](#output\_labels) | Labels for CSP resources |
| <a name="output_tags"></a> [tags](#output\_tags) | Tags for CSP resources |
<!-- END_TF_DOCS -->