#!/bin/sh

# build the go extension, and then zip up
make all && cd bin && zip -r extension.zip extensions

# then run this command with amazon stuff exported

# AWS_ACCESS_KEY_ID='' AWS_SECRET_ACCESS_KEY='' \
# aws lambda publish-layer-version \
#  --layer-name "apm-lambda-extension" \
#  --region us-west-2 \
#  --zip-file  "fileb://extension.zip"
