build:
	GOOS=linux GOARCH=amd64 go build -o bin/extensions/apm-lambda-extension main.go

build-GoExampleExtensionLayer:
	GOOS=linux GOARCH=amd64 go build -o $(ARTIFACTS_DIR)/extensions/apm-lambda-extension main.go
	chmod +x $(ARTIFACTS_DIR)/extensions/apm-lambda-extension

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
	make build
	cd bin && rm extension.zip || true && zip -r extension.zip extensions
	aws lambda publish-layer-version --layer-name "${ELASTIC_LAYER_NAME}" --zip-file "fileb://./bin/extension.zip"
run-GoExampleExtensionLayer:
	go run apm-lambda-extension/main.go

test:
	go test extension/*.go -v
