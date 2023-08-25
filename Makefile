SHELL = /bin/bash -eo pipefail

GORELEASER_VERSION = "v1.14.1"
GO_LICENSER_VERSION = "v0.4.0"
GOLANGCI_LINT_VERSION = "v1.54.2"
export DOCKER_IMAGE_NAME = observability/apm-lambda-extension
export DOCKER_REGISTRY = docker.elastic.co

clean:
	@rm -rf dist/
	@docker image ls "$(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME)*" -aq | xargs -I {} docker rmi --force {} || true

dist:
	@go run github.com/goreleaser/goreleaser@$(GORELEASER_VERSION) release --snapshot --rm-dist

.PHONY: zip
zip:
	@go run github.com/goreleaser/goreleaser@$(GORELEASER_VERSION) release --snapshot --rm-dist --skip-docker

build:
	@go run github.com/goreleaser/goreleaser@$(GORELEASER_VERSION) build --snapshot --rm-dist

.PHONY: release
release:
	go run github.com/goreleaser/goreleaser@$(GORELEASER_VERSION) release --rm-dist

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
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) version
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

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
SMOKETEST_DIRS = $$(find ./tf -mindepth 0 -maxdepth 0 -type d)

.PHONY: smoketest/discover
smoketest/discover:
	@echo "$(SMOKETEST_DIRS)"

.PHONY: smoketest/run
smoketest/run: zip
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
