build:
	GOOS=linux GOARCH=amd64 go build -o bin/extensions/apm-lambda-extension main.go

build-GoExampleExtensionLayer:
	GOOS=linux GOARCH=amd64 go build -o $(ARTIFACTS_DIR)/extensions/apm-lambda-extension main.go
	chmod +x $(ARTIFACTS_DIR)/extensions/apm-lambda-extension

run-GoExampleExtensionLayer:
	go run apm-lambda-extension/main.go

test:
	go test extension/*.go -v
