# Build file should build the function into a zip deployable in lambda
# and put the zip in ../build/${LAMBDA_RUNTIME_NO_PERIODS}.zip

#!/bin/bash

set -e

pip install -t ./package -r requirements.txt
zip -r ../../build/python3_8.zip .
zip -g ../../build/python3_8.zip main.py
