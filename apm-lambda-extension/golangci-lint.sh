#!/bin/sh
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)"/bin v1.45.0
golangci-lint --version
golangci-lint run | read && echo "Code differs from golangci-lint style. Run 'golangci-lint run'" 1>&2 && exit 1 || true