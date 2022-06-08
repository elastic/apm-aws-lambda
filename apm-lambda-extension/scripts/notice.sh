#!/usr/bin/env bash

# Copyright 2022 Elasticsearch BV
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Script to generate a NOTICE file containing licence information from dependencies.

# cf. https://github.com/elastic/cloud-on-k8s/tree/main/hack/licence-detector

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR=${SCRIPT_DIR}/..
TEMP_DIR=$(mktemp -d)
LICENCE_DETECTOR="go.elastic.co/go-licence-detector@v0.3.0"

trap '[[ $TEMP_DIR ]] && rm -rf "$TEMP_DIR"' EXIT

get_licence_detector() {
    GOBIN="$TEMP_DIR" go install "$LICENCE_DETECTOR"
}

generate_notice() {
    (
        cd "$PROJECT_DIR"
        go mod download
        export LC_ALL=C
        pkgs=$(go list -deps -f '{{define "M"}}{{.Path}}@{{.Version}}{{end}}{{with .Module}}{{if not .Main}}{{if .Replace}}{{template "M" .Replace}}{{else}}{{template "M" .}}{{end}}{{end}}{{end}}' | sort -u)
        deps=""
        for pkg in $pkgs; do
          deps+="$(go list -m -json $pkg)\n"
        done
        echo -e $deps | "${TEMP_DIR}"/go-licence-detector \
            -depsTemplate="${SCRIPT_DIR}"/templates/dependencies.asciidoc.tmpl \
            -depsOut="${PROJECT_DIR}"/dependencies.asciidoc \
            -noticeTemplate="${SCRIPT_DIR}"/templates/NOTICE.txt.tmpl \
            -noticeOut="${PROJECT_DIR}"/NOTICE.txt \
            -overrides="${SCRIPT_DIR}"/overrides/overrides.json \
            -rules="${SCRIPT_DIR}"/rules.json \
            -includeIndirect
    )
}

echo "Generating notice file and dependency list"
get_licence_detector
generate_notice
