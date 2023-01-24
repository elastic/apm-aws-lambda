SHELL = /bin/bash -eo pipefail
DOCKER_IMAGE_NAME = observability/apm-lambda-extension
DOCKER_REGISTRY = docker.elastic.co

clean:
	@rm -rf dist/
	@docker image ls "$(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME)*" -aq | xargs docker rmi --force

dist:
	@goreleaser release --snapshot --rm-dist

release:
	@goreleaser release --rm-dist

release-notes:
	@./.ci/release-github.sh

lint:
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.48.0 version
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.48.0 run

NOTICE.txt:
	@bash ./scripts/notice.sh
