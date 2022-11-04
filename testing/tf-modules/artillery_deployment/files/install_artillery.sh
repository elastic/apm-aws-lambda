#!/bin/bash

set -e

INSTALL_NVM_VER=v0.34.0

# Install nvm
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/$INSTALL_NVM_VER/install.sh | bash
source ~/.nvm/nvm.sh

# Install npm lts
nvm install --lts
nvm use default

# Install artillery
npm install --quiet --no-progress -g artillery@latest

# Check version
artillery --version
