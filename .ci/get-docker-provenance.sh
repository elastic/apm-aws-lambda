#!/usr/bin/env bash
set -euo pipefail

if [ ! -e dist/artifacts.json ] ; then
    exit 1
fi

echo "Gather the container images generated and published with goreleaser"
images=$(jq -r '[.[] | select (.type=="Published Docker Image") | select(.name|endswith("latest")|not)]' dist/artifacts.json)
image_1=$(echo "$images" | jq -r '.[0].name')
image_2=$(echo "$images" | jq -r '.[1].name')
digest_1=$(echo "$images" | jq -r '.[0].extra.Digest')
digest_2=$(echo "$images" | jq -r '.[1].extra.Digest')

echo "Export github actions outputs"
echo "name_1=$image_1"    >> "$GITHUB_OUTPUT"
echo "name_2=$image_2"    >> "$GITHUB_OUTPUT"
echo "digest_1=$digest_1" >> "$GITHUB_OUTPUT"
echo "digest_2=$digest_2" >> "$GITHUB_OUTPUT"
