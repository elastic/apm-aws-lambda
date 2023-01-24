#!/bin/bash
set -euo pipefail

export SUFFIX_ARN_FILE=${SUFFIX_ARN_FILE:-arn-file.md}
VERSION=${VERSION:?Please provide VERSION environment variable. e.g. current git tag}

rm -rf "${SUFFIX_ARN_FILE}"

cat ./*"-${SUFFIX_ARN_FILE}" >> "$SUFFIX_ARN_FILE"

gh release \
  create "${VERSION}" \
  --title="${VERSION}" \
  --generate-notes \
  --notes-file="${SUFFIX_ARN_FILE}" \
  ./dist/${VERSION}*.zip

