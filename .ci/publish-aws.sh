#!/usr/bin/env bash
set -euo pipefail


env | sort

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

GOOS=${GOOS:?Please provide GOOS environment variable.}
GOARCH=${GOARCH:?Please provide GOARCH environment variable.}
ELASTIC_LAYER_NAME=${ELASTIC_LAYER_NAME:?Please provide ELASTIC_LAYER_NAME environment variable.}
ARCHITECTURE=${ARCHITECTURE:?Please provide ARCHITECTURE environment variable.}
VERSION=${VERSION:?Please provide VERSION environment variable. e.g. current git tag}
