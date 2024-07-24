#!/bin/bash

set -e

export TF_IN_AUTOMATION=1
export TF_CLI_ARGS=-no-color

cleanup() {
  if [ "$SKIP_DESTROY" != "1" ]; then
    echo "-> Tearing down the underlying infrastructure..."
    terraform destroy -auto-approve | tee -a tf.log
  fi
}

trap "cleanup" EXIT



echo "-> Creating the underlying infrastructure..."
terraform init | tee tf.log
terraform apply -auto-approve -var "function_runtime=${FUNCTION_RUNTIME:-nodejs}" | tee -a tf.log

# https://github.com/hashicorp/setup-terraform/issues/167#issuecomment-1090760365
TERRAFORM_WRAPPER=terraform-bin
if [[ -z "${GITHUB_WORKFLOW}" ]]; then
  TERRAFORM_WRAPPER=terraform
fi

AWS_REGION=$($TERRAFORM_WRAPPER output -raw aws_region)
FUNCTION_NAME=$($TERRAFORM_WRAPPER output -raw user_name)-smoke-testing-test

echo "-> Calling the lambda function..."
aws lambda invoke --region="${AWS_REGION}" --function-name "${FUNCTION_NAME}" response.json
aws lambda invoke --region="${AWS_REGION}" --function-name "${FUNCTION_NAME}" response.json

echo "-> Waiting for the agent documents to be indexed in Elasticsearch..."

ES_HOST=$($TERRAFORM_WRAPPER output -raw elasticsearch_url)
ES_USER=$($TERRAFORM_WRAPPER output -raw elasticsearch_username)
ES_PASS=$($TERRAFORM_WRAPPER output -raw elasticsearch_password)
AGENT_NAME=$($TERRAFORM_WRAPPER output -raw agent_name)

hits=0
attempts=0
while [ ${hits} -eq 0 ]; do
    # Check that ES has transaction documents on the GET endpoint.
    resp=$(curl -s -H 'Content-Type: application/json' -u ${ES_USER}:${ES_PASS} "${ES_HOST}/traces-apm-*/_search" -d "{
    \"query\": {
        \"match\": {
        \"agent.name\": \"${AGENT_NAME}\"
        }
    }
    }")
    hits=$(echo ${resp} | jq '.hits.total.value')
    if [[ ${attempts} -ge 30 ]]; then
        echo "-> Didn't find any traces for the ${AGENT_NAME} agents after ${attempts} attempts"
        exit 1
    fi
    let "attempts+=1"
    sleep 1
done

echo "-> Smoke tests passed!"
