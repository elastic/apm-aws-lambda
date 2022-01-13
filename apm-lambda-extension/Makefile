build:
ifndef GOARCH
	GOARCH=amd64
endif
	GOOS=linux go build -o bin/extensions/apm-lambda-extension main.go
	chmod +x bin/extensions/apm-lambda-extension
build-and-publish:
ifndef AWS_DEFAULT_REGION
	$(error AWS_DEFAULT_REGION is undefined)
endif
ifndef AWS_ACCESS_KEY_ID
	$(error AWS_ACCESS_KEY_ID is undefined)
endif
ifndef AWS_SECRET_ACCESS_KEY
	$(error AWS_SECRET_ACCESS_KEY is undefined)
endif
ifndef ELASTIC_LAYER_NAME
	$(error ELASTIC_LAYER_NAME is undefined)
endif
	GOARCH=${GOARCH} make build
	GOARCH=${GOARCH} make zip
	$(MAKE) publish
zip:
	cd bin && rm -f extension.zip || true && zip -r extension.zip extensions && cp extension.zip ${GOARCH}.zip
test:
	go test extension/*.go -v
env:
ifndef GOARCH
	export GOARCH=amd64
endif
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
publish-in-all-aws-regions: get-all-aws-regions
ifndef ELASTIC_LAYER_NAME
	$(error ELASTIC_LAYER_NAME is undefined)
endif
	@while read AWS_DEFAULT_REGION; do \
		AWS_DEFAULT_REGION="$${AWS_DEFAULT_REGION}" ELASTIC_LAYER_NAME=$(ELASTIC_LAYER_NAME) $(MAKE) publish; \
	done <.regions

# Publish the given LAYER in the given AWS region
publish:
ifndef AWS_DEFAULT_REGION
	$(error AWS_DEFAULT_REGION is undefined)
endif
ifndef ELASTIC_LAYER_NAME
	$(error ELASTIC_LAYER_NAME is undefined)
endif
	@echo 'publish "${ELASTIC_LAYER_NAME}" in ${AWS_DEFAULT_REGION}'
	@echo 'aws lambda publish-layer-version --layer-name "${ELASTIC_LAYER_NAME}" --zip-file "fileb://./bin/extension.zip"'

release-notes:
ifndef BRANCH_NAME
	$(error BRANCH_NAME is undefined)
endif
ifndef CHANGELOG
	$(error CHANGELOG is undefined)
endif
	@gh release list
	@echo "gh release create $(BRANCH_NAME) -F $(CHANGELOG)"
