#!/bin/bash

set -e

export TF_IN_AUTOMATION=1
export TF_CLI_ARGS=-no-color

cleanup() {
  [ "$SKIP_DESTROY" != "1" ]; terraform destroy -auto-approve >> tf.log
}

trap "cleanup" EXIT

echo "-> Creating the underlying infrastructure..."
terraform init | tee tf.log
terraform apply -auto-approve | tee -a tf.log

# https://github.com/hashicorp/setup-terraform/issues/167#issuecomment-1090760365
if [[ -z "${GITHUB_WORKFLOW}" ]]; then
  AWS_REGION=$(terraform output -raw aws_region)
else
  AWS_REGION=$(terraform-bin output -raw aws_region)
fi


echo "-> Calling the lambda function..."
aws lambda invoke --region="${AWS_REGION}" --function-name smoke-testing-test response.json
aws lambda invoke --region="${AWS_REGION}" --function-name smoke-testing-test response.json

echo "-> Waiting for the agent documents to be indexed in Elasticsearch..."

# https://github.com/hashicorp/setup-terraform/issues/167#issuecomment-1090760365
if [[ -z "${GITHUB_WORKFLOW}" ]]; then
  ES_HOST=$(terraform output -raw elasticsearch_url)
  ES_USER=$(terraform output -raw elasticsearch_username)
  ES_PASS=$(terraform output -raw elasticsearch_password)
else
  ES_HOST=$(terraform-bin output -raw elasticsearch_url)
  ES_USER=$(terraform-bin output -raw elasticsearch_username)
  ES_PASS=$(terraform-bin output -raw elasticsearch_password)
fi



hits=0
attempts=0
while [ ${hits} -eq 0 ]; do
    # Check that ES has transaction documents on the GET endpoint.
    resp=$(curl -s -H 'Content-Type: application/json' -u ${ES_USER}:${ES_PASS} "${ES_HOST}/traces-apm-*/_search" -d '{
    "query": {
        "match": {
        "agent.name": "nodejs"
        }
    }
    }')
    hits=$(echo ${resp} | jq '.hits.total.value')
    if [[ ${attempts} -ge 30 ]]; then
        echo "-> Didn't find any traces for the NodeJS agents after ${attempts} attempts"
        exit 1
    fi
    let "attempts+=1"
    sleep 1
done

echo "-> Smoke tests passed!"
