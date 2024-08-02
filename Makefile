SHELL = /bin/bash -eo pipefail

GORELEASER_VERSION = "v1.19.2"
GO_LICENSER_VERSION = "v0.4.0"
GOLANGCI_LINT_VERSION = "v1.59.1"
export DOCKER_IMAGE_NAME = observability/apm-lambda-extension
export DOCKER_REGISTRY = docker.elastic.co

clean:
	@rm -rf dist/
	@docker image ls "$(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME)*" -aq | xargs -I {} docker rmi --force {} || true

.PHONY: dist
dist:
	@go run github.com/goreleaser/goreleaser@$(GORELEASER_VERSION) release --snapshot --clean

.PHONY: zip
zip:
	@go run github.com/goreleaser/goreleaser@$(GORELEASER_VERSION) release --snapshot --clean --skip-docker

build:
	@go run github.com/goreleaser/goreleaser@$(GORELEASER_VERSION) build --snapshot --clean

.PHONY: release
release:
	go run github.com/goreleaser/goreleaser@$(GORELEASER_VERSION) release --clean

.PHONY: release-notes
release-notes:
	@./.ci/release-github.sh

.PHONY: test
test:
	@go run gotest.tools/gotestsum@v1.9.0 --format testname --junitfile $(junitfile)

.PHONY: lint-prep
lint-prep:
	@go mod tidy && git diff --exit-code

.PHONY: lint
lint:
	@if [ "$(CI)" != "" ]; then go run github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) version; fi
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --build-tags tools

NOTICE.txt: go.mod
	@bash ./scripts/notice.sh

.PHONY: check-linceses
check-licenses:
	@go run github.com/elastic/go-licenser@$(GO_LICENSER_VERSION) -d -exclude tf -exclude testing .
	@go run github.com/elastic/go-licenser@$(GO_LICENSER_VERSION) -d -exclude tf -exclude testing -ext .java .
	@go run github.com/elastic/go-licenser@$(GO_LICENSER_VERSION) -d -exclude tf -exclude testing -ext .js .


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
