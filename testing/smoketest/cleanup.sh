#!/bin/bash

set -e

export TF_IN_AUTOMATION=1
export TF_CLI_ARGS=-no-color

echo "-> Tearing down the underlying infrastructure..."
terraform destroy -auto-approve | tee -a tf.log
