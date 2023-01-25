#!/usr/bin/env bash
set -euo pipefail

export AWS_FOLDER=${AWS_FOLDER:-.aws}
export SUFFIX_ARN_FILE=${SUFFIX_ARN_FILE:-arn-file.md}

GOOS=${GOOS:?Please provide GOOS environment variable.}
GOARCH=${GOARCH:?Please provide GOARCH environment variable.}
ELASTIC_LAYER_NAME=${ELASTIC_LAYER_NAME:?Please provide ELASTIC_LAYER_NAME environment variable.}
ARCHITECTURE=${ARCHITECTURE:?Please provide ARCHITECTURE environment variable.}
VERSION=${VERSION:?Please provide VERSION environment variable. e.g. current git tag}

FULL_LAYER_NAME="${ELASTIC_LAYER_NAME}-${ARCHITECTURE}"

ALL_AWS_REGIONS=$(aws ec2 describe-regions --output json --no-cli-pager | jq -r '.Regions[].RegionName')

rm -rf ${AWS_FOLDER}

# Delete previous layers
for region in $ALL_AWS_REGIONS; do
  layer_versions=$(aws lambda list-layer-versions --region="${region}" --layer-name="${FULL_LAYER_NAME}" | jq '.LayerVersions[].Version')
  echo "Found layer versions for ${FULL_LAYER_NAME} in ${region}: ${layer_versions:-none}"
  for version_number in $layer_versions; do
    echo "- Deleting ${FULL_LAYER_NAME}:${version_number} in ${region}"
    aws lambda delete-layer-version \
        --region="${region}" \
        --layer-name="${FULL_LAYER_NAME}" \
        --version-number="${version_number}"
  done
done

mkdir -p "${AWS_FOLDER}"

for region in $ALL_AWS_REGIONS; do
  echo "Publish ${FULL_LAYER_NAME} in ${region}"
  publish_output=$(aws lambda \
    --output json \
    publish-layer-version \
    --region="${region}" \
    --layer-name="${FULL_LAYER_NAME}" \
    --description="AWS Lambda Extension Layer for Elastic APM ${ARCHITECTURE}" \
    --license="Apache-2.0" \
    --zip-file="fileb://./dist/${VERSION}-${GOOS}-${GOARCH}.zip")
  echo "${publish_output}" > "${AWS_FOLDER}/${region}"
  layer_version=$(echo "${publish_output}" | jq '.Version')
  echo "Grant public layer access ${FULL_LAYER_NAME}:${layer_version} in ${region}"
  grant_access_output=$(aws lambda \
  		--output json \
  		add-layer-version-permission \
  		--region="${region}" \
  		--layer-name="${FULL_LAYER_NAME}" \
  		--action="lambda:GetLayerVersion" \
  		--principal='*' \
  		--statement-id="${FULL_LAYER_NAME}" \
  		--version-number="${layer_version}")
  echo "${grant_access_output}" > "${AWS_FOLDER}/.${region}-public"
done

sh -c "./.ci/create-arn-table.sh"
