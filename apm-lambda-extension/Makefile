SHELL = /bin/bash -eo pipefail

AWS_FOLDER = .aws

ifndef GOARCH
	GOARCH=amd64
endif

# Transform GOARCH into the architecture of the extension layer
ifeq ($(GOARCH),amd64)
	ARCHITECTURE=x86_64
else
	ARCHITECTURE=arm64
endif

export AWS_FOLDER GOARCH ARCHITECTURE

build:
	GOOS=linux go build -o bin/extensions/apm-lambda-extension main.go
	chmod +x bin/extensions/apm-lambda-extension
build-and-publish: validate-layer-name validate-aws-default-region
ifndef AWS_ACCESS_KEY_ID
	$(error AWS_ACCESS_KEY_ID is undefined)
endif
ifndef AWS_SECRET_ACCESS_KEY
	$(error AWS_SECRET_ACCESS_KEY is undefined)
endif
	GOARCH=${GOARCH} make build
	GOARCH=${GOARCH} make zip
	$(MAKE) publish
zip:
	cd bin && rm -f extension.zip || true && zip -r extension.zip extensions && cp extension.zip ${GOARCH}.zip
test:
	go test extension/*.go -v
env:
	env
dist: validate-branch-name build test zip
	@cp ./bin/$(GOARCH).zip bin/$(BRANCH_NAME)-linux-$(GOARCH).zip

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
	done <.regions
	$(MAKE) create-arn-file

# Publish the given LAYER in the given AWS region
publish: validate-layer-name validate-aws-default-region
	@aws lambda \
		--output json \
		publish-layer-version \
		--layer-name "$(ELASTIC_LAYER_NAME)-$(ARCHITECTURE)" \
		--description "AWS Lambda Extension Layer for Elastic APM $(ARCHITECTURE)" \
		--license "Apache-2.0" \
		--zip-file "fileb://./bin/extension.zip"

# Generate the file with the ARN entries
create-arn-file: validate-suffix-arn-file
	@../.ci/create-arn-table.sh

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
