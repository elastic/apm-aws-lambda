#!/usr/bin/env bash
set -euo pipefail

if [ ! -e dist/artifacts.json ] ; then
    exit 1
fi

echo "Gather the container images generated and published with goreleaser"
# NOTE: we are assuming that the first two images in the artifacts.json file are the ones we just built and published, 
#       which is true for now but may not be in the future if we add more images or other types of artifacts.
#       We should consider adding some metadata to the goreleaser config to be able to identify our images more reliably.
images=$(jq -r '[.[] | select (.type=="Docker Image") | select(.name|contains("latest")|not)]' dist/artifacts.json)
image_1=$(echo "$images" | jq -r '.[0].name')
image_2=$(echo "$images" | jq -r '.[1].name')
digest_1=$(echo "$images" | jq -r '.[0].extra.Digest')
digest_2=$(echo "$images" | jq -r '.[1].extra.Digest')

echo "Export github actions outputs"
echo "name_1=$image_1"    >> "$GITHUB_OUTPUT"
echo "name_2=$image_2"    >> "$GITHUB_OUTPUT"
echo "digest_2=$digest_2" >> "$GITHUB_OUTPUT"
echo "digest_1=$digest_1" >> "$GITHUB_OUTPUT"
