# Build file should build the function into a zip deployable in lambda
# and put the zip in ../build/${LAMBDA_RUNTIME_NO_PERIODS}.zip

#!/bin/bash

set -e

zip -g ../../build/python3_9.zip main.py
