# Build file should build the function into a zip deployable in lambda
# and put the zip in ../build/${LAMBDA_RUNTIME_NO_PERIODS}.zip

#!/bin/bash

set -e

CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build main.go
zip ../../build/go1_x.zip main
