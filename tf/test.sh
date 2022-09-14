#!/bin/bash

set -e

export TF_IN_AUTOMATION=1
export TF_CLI_ARGS=-no-color

cleanup() {
  terraform destroy -auto-approve >> tf.log
}

trap "cleanup" EXIT

echo "-> Creating the underlying infrastructure..."
terraform init | tee tf.log
terraform apply -auto-approve | tee -a tf.log

echo "-> Calling the lambda function..."
aws lambda invoke --function-name smoke-testing-test response.json
aws lambda invoke --function-name smoke-testing-test response.json

echo "-> Waiting for the agent documents to be indexed in Elasticsearch..."

ES_HOST=$(terraform output -raw elasticsearch_url)
ES_USER=$(terraform output -raw elasticsearch_username)
ES_PASS=$(terraform output -raw elasticsearch_password)

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
