SHELL = /bin/bash -eo pipefail

AWS_FOLDER = .aws
DOCKER_IMAGE_NAME = observability/apm-lambda-extension
DOCKER_REGISTRY = docker.elastic.co
AGENT_VERSION = $(shell echo $${BRANCH_NAME} | cut -f 2 -d 'v')

# Add support for SOURCE_DATE_EPOCH and reproducble buils
# See https://reproducible-builds.org/specs/source-date-epoch/
SOURCE_DATE_EPOCH ?= $(shell git log -1 --pretty=%ct)
DATE_FMT = +%Y%m%d%H%M.%S
# Fallback mechanism to support other systems:
# 1. 'date -d': Busybox and GNU coreutils.
# 2. 'date -r': BSD date. It does not support '-d'.
BUILD_DATE = $(shell date -u -d "@${SOURCE_DATE_EPOCH}" "${DATE_FMT}" 2>/dev/null || date -u -r "${SOURCE_DATE_EPOCH}" "${DATE_FMT}")

ifndef GOARCH
	GOARCH=amd64
endif

# Transform GOARCH into the architecture of the extension layer
ifeq ($(GOARCH),amd64)
	ARCHITECTURE=x86_64
else
	ARCHITECTURE=arm64
endif

export AWS_FOLDER GOARCH ARCHITECTURE DOCKER_IMAGE_NAME DOCKER_REGISTRY

.PHONY: all
all: build

NOTICE.txt: go.mod
	@bash ./scripts/notice.sh

check-licenses:
	go run github.com/elastic/go-licenser@v0.4.0 -d -exclude tf .
	go run github.com/elastic/go-licenser@v0.4.0 -d -exclude tf -ext .java .
	go run github.com/elastic/go-licenser@v0.4.0 -d -exclude tf -ext .js .

update-licenses:
	go run github.com/elastic/go-licenser@v0.4.0 -exclude tf .
	go run github.com/elastic/go-licenser@v0.4.0 -exclude tf -ext .java .
	go run github.com/elastic/go-licenser@v0.4.0 -exclude tf -ext .js .

lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.48.0 version
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.48.0 run

build: check-licenses NOTICE.txt
	CGO_ENABLED=0 GOOS=linux go build -o bin/extensions/apm-lambda-extension main.go
	cp NOTICE.txt bin/NOTICE.txt
	cp dependencies.asciidoc bin/dependencies.asciidoc

build-and-publish: check-licenses validate-layer-name validate-aws-default-region
ifndef AWS_ACCESS_KEY_ID
	$(error AWS_ACCESS_KEY_ID is undefined)
endif
ifndef AWS_SECRET_ACCESS_KEY
	$(error AWS_SECRET_ACCESS_KEY is undefined)
endif
	GOARCH=${GOARCH} make build
	GOARCH=${GOARCH} make zip
	$(MAKE) publish

zip: build
	cd bin \
	&& rm -f extension.zip \
	&& find extensions NOTICE.txt dependencies.asciidoc | xargs touch -t ${BUILD_DATE} \
	&& zip -X -r extension.zip extensions NOTICE.txt dependencies.asciidoc \
	&& cp extension.zip ${GOARCH}.zip

test:
	go test extension/*.go -v

env:
	env

dist: validate-branch-name test zip
	@cp ./bin/$(GOARCH).zip bin/$(BRANCH_NAME)-linux-$(GOARCH).zip

build-docker: validate-version
	docker build -t $(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME)-$(ARCHITECTURE):$(AGENT_VERSION) \
  --build-arg EXTENSION_FILE=bin/extensions/apm-lambda-extension .

push-docker: build-docker
	@./.ci/push_docker.sh $(DOCKER_REGISTRY) "$(DOCKER_IMAGE_NAME)-$(ARCHITECTURE)" $(AGENT_VERSION)

# List all the AWS regions
get-all-aws-regions:
	@aws \
		ec2 \
		describe-regions \
		--region us-east-1 \
		--output json \
		--no-cli-pager \
		| jq -r '.Regions[].RegionName' > .regions

# Publish the given LAYER in all the AWS regions
publish-in-all-aws-regions: validate-layer-name get-all-aws-regions
	@mkdir -p $(AWS_FOLDER)
	@while read AWS_DEFAULT_REGION; do \
		echo "publish '$(ELASTIC_LAYER_NAME)-$(ARCHITECTURE)' in $${AWS_DEFAULT_REGION}"; \
		AWS_DEFAULT_REGION="$${AWS_DEFAULT_REGION}" ELASTIC_LAYER_NAME=$(ELASTIC_LAYER_NAME) $(MAKE) publish > $(AWS_FOLDER)/$${AWS_DEFAULT_REGION}; \
		AWS_DEFAULT_REGION="$${AWS_DEFAULT_REGION}" ELASTIC_LAYER_NAME=$(ELASTIC_LAYER_NAME) $(MAKE) grant-public-layer-access; \
	done <.regions

# Publish the given LAYER in the given AWS region
publish: validate-layer-name validate-aws-default-region
	@aws lambda \
		--output json \
		publish-layer-version \
		--layer-name "$(ELASTIC_LAYER_NAME)-$(ARCHITECTURE)" \
		--description "AWS Lambda Extension Layer for Elastic APM $(ARCHITECTURE)" \
		--license "Apache-2.0" \
		--zip-file "fileb://./bin/extension.zip"

# Grant public access to the given LAYER in the given AWS region
grant-public-layer-access: validate-layer-name validate-aws-default-region
	@echo "[debug] $(ELASTIC_LAYER_NAME)-$(ARCHITECTURE) with version: $$($(MAKE) -s --no-print-directory get-version)"
	@aws lambda \
		--output json \
		add-layer-version-permission \
		--layer-name "$(ELASTIC_LAYER_NAME)-$(ARCHITECTURE)" \
		--action lambda:GetLayerVersion \
		--principal '*' \
		--statement-id "$(ELASTIC_LAYER_NAME)-$(ARCHITECTURE)" \
		--version-number $$($(MAKE) -s --no-print-directory get-version) > $(AWS_FOLDER)/.$(AWS_DEFAULT_REGION)-public

# Get the ARN Version for the AWS_REGIONS
# NOTE: jq -r .Version "$(AWS_FOLDER)/$(AWS_DEFAULT_REGION)" fails in the CI
#       with 'parse error: Invalid numeric literal at line 1, column 5'
get-version: validate-aws-default-region
	@grep '"Version"' "$(AWS_FOLDER)/$(AWS_DEFAULT_REGION)" | cut -d":" -f2 | sed 's/ //g' | cut -d"," -f1

# Generate the file with the ARN entries
create-arn-file: validate-suffix-arn-file
	@./.ci/create-arn-table.sh

release-notes: validate-branch-name validate-suffix-arn-file
	@gh release list
	cat *-$(SUFFIX_ARN_FILE) > $(SUFFIX_ARN_FILE)
	@gh \
		release \
		create $(BRANCH_NAME) \
		--title '$(BRANCH_NAME)' \
		--generate-notes \
		--notes-file $(SUFFIX_ARN_FILE) \
		./bin/$(BRANCH_NAME)*.zip

validate-version:
ifndef AGENT_VERSION
	$(error AGENT_VERSION is undefined)
endif

validate-branch-name:
ifndef BRANCH_NAME
	$(error BRANCH_NAME is undefined)
endif

validate-suffix-arn-file:
ifndef SUFFIX_ARN_FILE
	$(error SUFFIX_ARN_FILE is undefined)
endif

validate-layer-name:
ifndef ELASTIC_LAYER_NAME
	$(error ELASTIC_LAYER_NAME is undefined)
endif

validate-aws-default-region:
ifndef AWS_DEFAULT_REGION
	$(error AWS_DEFAULT_REGION is undefined)
endif

##############################################################################
# Smoke tests -- Basic smoke tests for the APM Lambda extension
##############################################################################

SMOKETEST_VERSIONS ?= latest
SMOKETEST_DIRS = $$(find ./tf -mindepth 0 -maxdepth 0 -type d)

.PHONY: smoketest/discover
smoketest/discover:
	@echo "$(SMOKETEST_DIRS)"

.PHONY: smoketest/run
smoketest/run: build
	@ for version in $(shell echo $(SMOKETEST_VERSIONS) | tr ',' ' '); do \
		echo "-> Running $(TEST_DIR) smoke tests for version $${version}..."; \
		cd $(TEST_DIR) && ./test.sh $${version}; \
	done

.PHONY: smoketest/cleanup
smoketest/cleanup:
	@ cd $(TEST_DIR); \
	if [ -f "./cleanup.sh" ]; then \
		./cleanup.sh; \
	fi

.PHONY: smoketest/all
smoketest/all/cleanup:
	@ for test_dir in $(SMOKETEST_DIRS); do \
		echo "-> Cleanup $${test_dir} smoke tests..."; \
		$(MAKE) smoketest/cleanup TEST_DIR=$${test_dir}; \
	done
