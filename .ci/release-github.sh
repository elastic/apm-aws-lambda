#!/usr/bin/env bash
set -euo pipefail

#
# Collects all created ARN tables created by .ci/publish-aws.sh script
# and publishes a GitHub Release Note with the given information.
#

export SUFFIX_ARN_FILE=${SUFFIX_ARN_FILE:-arn-file.md}
VERSION=${VERSION:?Please provide VERSION environment variable. e.g. current git tag}

rm -rf "${SUFFIX_ARN_FILE}"

cat ./*"-${SUFFIX_ARN_FILE}" >> "$SUFFIX_ARN_FILE"

gh release \
  create "${VERSION}" \
  --title="${VERSION}" \
  --generate-notes \
  --notes-file="${SUFFIX_ARN_FILE}" \
  ./dist/${VERSION}*.json \
  ./dist/${VERSION}*.zip \
  ./dist/*checksums.txt
