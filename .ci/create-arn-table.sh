#!/usr/bin/env bash
set -o pipefail

#
# Create the AWS ARN table given the below environment variables:
#
#   - AWS_FOLDER      - that's the location of the publish-layer-version output for each region
#	- ARCHITECTURE    - that's the supported architecture.
#	- SUFFIX_ARN_FILE - that's the output file.
#

ARN_FILE=${ARCHITECTURE}-${SUFFIX_ARN_FILE}

{
	echo "### ARCH: ${ARCHITECTURE}"
	echo ''
	echo '|Region|Arch|ARN|'
	echo '|------|----|---|'
} > "${ARN_FILE}"

for f in $(ls "${AWS_FOLDER}"); do
	LAYER_VERSION_ARN=$(grep '"LayerVersionArn"' "$AWS_FOLDER/${f}" | cut -d":" -f2- | sed 's/ //g' | sed 's/"//g' | cut -d"," -f1)
	echo "INFO: create-arn-table ARN(${LAYER_VERSION_ARN}):region(${f}):arch(${ARCHITECTURE})"
	echo "|${f}|${ARCHITECTURE}|${LAYER_VERSION_ARN}|" >> "${ARN_FILE}"
done

echo '' >> "${ARN_FILE}"
