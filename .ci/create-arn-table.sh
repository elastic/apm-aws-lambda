#!/usr/bin/env bash
set -exo pipefail

#
# Create the AWS ARN table given the below environment variables:
#
#   - AWS_FOLDER      - that's the location of the publish-layer-version output for each region
#	- ARCHITECTURE    - that's the supported architecture.
#	- PREFIX_ARN_FILE - that's the output file.
#

{
	echo "### ARCH: ${ARCHITECTURE}"
	echo ''
	echo '|Region|Arch|ARN|'
	echo '|------|----|---|'
	for f in $(ls "${AWS_FOLDER}"); do
		# TODO: identify what field to be used.
		echo "|${f}|${ARCHITECTURE}|$(cat $AWS_FOLDER/${f} | jq -r .LayerVersionArn)|"
	done
	echo ''
} > ${ARCHITECTURE}-${PREFIX_ARN_FILE}
