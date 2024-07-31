data "external" "username" {
  program = ["./get_username.sh"]
}

locals {
  user_name = data.external.username.result.user_name
}
