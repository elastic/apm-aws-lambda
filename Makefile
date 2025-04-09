SHELL = /bin/bash -eo pipefail

export DOCKER_IMAGE_NAME = observability/apm-lambda-extension
export DOCKER_REGISTRY = docker.elastic.co

clean:
	@rm -rf dist/
	@docker image ls "$(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME)*" -aq | xargs -I {} docker rmi --force {} || true

.PHONY: dist
dist:
	@go tool github.com/goreleaser/goreleaser/v2 release --snapshot --clean

.PHONY: zip
zip:
	@go tool github.com/goreleaser/goreleaser/v2 release --snapshot --clean --skip docker

build:
	@go tool github.com/goreleaser/goreleaser/v2 build --snapshot --clean

.PHONY: release
release:
	go tool github.com/goreleaser/goreleaser/v2 release --clean

.PHONY: release-notes
release-notes:
	@./.ci/release-github.sh

.PHONY: test
test:
	@go tool gotest.tools/gotestsum --format testname --junitfile $(junitfile)

.PHONY: lint-prep
lint-prep:
	@go mod tidy && git diff --exit-code

.PHONY: lint
lint:
	@if [ "$(CI)" != "" ]; then go tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint version; fi
	@go tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint run

NOTICE.txt: go.mod
	@bash ./scripts/notice.sh

.PHONY: check-licenses
check-licenses:
	@go tool github.com/elastic/go-licenser -d -exclude tf -exclude testing -exclude e2e-testing .
	@go tool github.com/elastic/go-licenser -d -exclude tf -exclude testing -exclude e2e-testing -ext .java .
	@go tool github.com/elastic/go-licenser -d -exclude tf -exclude testing -exclude e2e-testing -ext .js .


.PHONY: check-notice
check-notice:
	$(MAKE) NOTICE.txt
	@git diff --exit-code --quiet NOTICE.txt && exit 0 || echo "regenerate NOTICE.txt" && exit 1

##############################################################################
# Smoke tests -- Basic smoke tests for the APM Lambda extension
##############################################################################

SMOKETEST_VERSIONS ?= latest

.PHONY: smoketest/run
smoketest/run: zip
	@echo "-> Running smoke tests for version $${version}..."
	cd testing/smoketest && ./test.sh $${version}

.PHONY: smoketest/cleanup
smoketest/cleanup: zip
	@echo "-> Running cleanup"
	cd testing/smoketest && ./cleanup.sh
