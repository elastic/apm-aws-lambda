#!/usr/bin/env bash
set -euo pipefail

#
# Publishes the created artifacts from GoReleaser to AWS as AWS Lambda Layers in every region.
# Finalized by generating an ARN table which will be used in the release notes.
#

export AWS_FOLDER=${AWS_FOLDER:-.aws}
export SUFFIX_ARN_FILE=${SUFFIX_ARN_FILE:-arn-file.md}

# This needs to be set in GH actions
# https://dotjoeblog.wordpress.com/2021/03/14/github-actions-aws-error-exit-code-255/
# eu-west-1 is just a random region
export AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION:-eu-west-1}

printenv
GOOS=${GOOS:?Please provide GOOS environment variable.}
GOARCH=${GOARCH:?Please provide GOARCH environment variable.}
ELASTIC_LAYER_NAME=${ELASTIC_LAYER_NAME:?Please provide ELASTIC_LAYER_NAME environment variable.}
ARCHITECTURE=${ARCHITECTURE:?Please provide ARCHITECTURE environment variable.}
VERSION=${VERSION:?Please provide VERSION environment variable. e.g. current git tag}

FULL_LAYER_NAME="${ELASTIC_LAYER_NAME}-${ARCHITECTURE}"

ALL_AWS_REGIONS=$(aws ec2 describe-regions --output json --no-cli-pager | jq -r '.Regions[].RegionName')

rm -rf "${AWS_FOLDER}"

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

zip_file="./dist/${VERSION}-${GOOS}-${GOARCH}.zip"

for region in $ALL_AWS_REGIONS; do
  echo "Publish ${FULL_LAYER_NAME} in ${region}"
  aws lambda \
    --output json \
    publish-layer-version \
    --region="${region}" \
    --layer-name="${FULL_LAYER_NAME}" \
    --compatible-architectures="${ARCHITECTURE}" \
    --description="AWS Lambda Extension Layer for Elastic APM ${ARCHITECTURE}" \
    --license="Apache-2.0" \
    --zip-file="fileb://${zip_file}" > "${AWS_FOLDER}/${region}"
  publish_output=$(cat "${AWS_FOLDER}/${region}")
  layer_version=$(echo "${publish_output}" | jq '.Version')

  echo "Grant public layer access ${FULL_LAYER_NAME}:${layer_version} in ${region}"
  aws lambda \
  		--output json \
  		add-layer-version-permission \
  		--region="${region}" \
  		--layer-name="${FULL_LAYER_NAME}" \
  		--action="lambda:GetLayerVersion" \
  		--principal='*' \
  		--statement-id="${FULL_LAYER_NAME}" \
  		--version-number="${layer_version}" > "${AWS_FOLDER}/.${region}-public"
done

sh -c "./.ci/create-arn-table.sh"
