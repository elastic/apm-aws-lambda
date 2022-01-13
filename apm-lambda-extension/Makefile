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
	aws lambda publish-layer-version --layer-name "${ELASTIC_LAYER_NAME}" --zip-file "fileb://./bin/extension.zip"
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
