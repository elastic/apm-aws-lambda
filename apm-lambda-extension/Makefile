SHELL = /bin/bash -eo pipefail

AWS_FOLDER = .aws

ifndef GOARCH
	GOARCH=amd64
endif

export AWS_FOLDER GOARCH

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
dist: build test zip

# List all the AWS regions
get-all-aws-regions:
	@aws \
		ec2 \
		describe-regions \
		--region us-east-1 \
		--all-regions \
		--output json \
		--no-cli-pager \
		| jq -r '.Regions[].RegionName' > .regions

# Publish the given LAYER in all the AWS regions
publish-in-all-aws-regions: validate-layer-name get-all-aws-regions
	@while read AWS_DEFAULT_REGION; do \
		AWS_DEFAULT_REGION="$${AWS_DEFAULT_REGION}" ELASTIC_LAYER_NAME=$(ELASTIC_LAYER_NAME) $(MAKE) publish; \
	done <.regions
	$(MAKE) create-arn-file

# Publish the given LAYER in the given AWS region
publish: validate-branch-name validate-layer-name validate-aws-default-region
	@echo 'publish "${ELASTIC_LAYER_NAME}" in ${AWS_DEFAULT_REGION}'
	@mkdir -p $(AWS_FOLDER)
	@mv bin/$(GOARCH).zip bin/$(BRANCH_NAME)-linux-$(GOARCH).zip
	@aws lambda \
		--output json \
		publish-layer-version \
		--layer-name "${ELASTIC_LAYER_NAME}" \
		--zip-file "fileb://./bin/extension.zip" \
		--generate-cli-skeleton > $(AWS_FOLDER)/${AWS_DEFAULT_REGION}

# Generate the file with the ARN entries
create-arn-file: validate-branch-name validate-prefix-arn-file
	@../.ci/create-arn-table.sh

release-notes: validate-branch-name validate-prefix-arn-file
	@gh release list
	@cat *-$(PREFIX_ARN_FILE) > $(PREFIX_ARN_FILE)
	@gh \
		release \
		create $(BRANCH_NAME) \
		--title '$(BRANCH_NAME)' \
		--draft \
		--generate-notes \
		--notes-file $(PREFIX_ARN_FILE) \
		./bin/$(BRANCH_NAME)*.zip

validate-branch-name:
ifndef BRANCH_NAME
	$(error BRANCH_NAME is undefined)
endif

validate-prefix-arn-file:
ifndef PREFIX_ARN_FILE
	$(error PREFIX_ARN_FILE is undefined)
endif

validate-layer-name:
ifndef ELASTIC_LAYER_NAME
	$(error ELASTIC_LAYER_NAME is undefined)
endif

validate-aws-default-region:
ifndef AWS_DEFAULT_REGION
	$(error AWS_DEFAULT_REGION is undefined)
endif
