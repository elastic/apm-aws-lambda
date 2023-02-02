SHELL = /bin/bash -eo pipefail
export DOCKER_IMAGE_NAME = observability/apm-lambda-extension
export DOCKER_REGISTRY = docker.elastic.co

# Add support for SOURCE_DATE_EPOCH and reproducble buils
# See https://reproducible-builds.org/specs/source-date-epoch/
SOURCE_DATE_EPOCH ?= $(shell git log -1 --pretty=%ct)
DATE_FMT = +%Y%m%d%H%M.%S
DATE_FMT_RFC_3339 = +%Y-%m-%dT%H:%M:%SZ
# Fallback mechanism to support other systems:
# 1. 'date -d': Busybox and GNU coreutils.
# 2. 'date -r': BSD date. It does not support '-d'.
export BUILD_DATE = $(shell date -u -d "@${SOURCE_DATE_EPOCH}" "${DATE_FMT}" 2>/dev/null || date -u -r "${SOURCE_DATE_EPOCH}" "${DATE_FMT}")
export BUILD_DATE_RFC_3339 = $(shell date -u -d "@${SOURCE_DATE_EPOCH}" "${DATE_FMT_RFC_3339}" 2>/dev/null || date -u -r "${SOURCE_DATE_EPOCH}" "${DATE_FMT_RFC_3339}")

clean:
	@rm -rf dist/
	@docker image ls "$(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME)*" -aq | xargs docker rmi --force

dist:
	@go run github.com/goreleaser/goreleaser@v1.14.1 release --snapshot --rm-dist

.PHONY: release
release:
	go run github.com/goreleaser/goreleaser@v1.14.1 release --rm-dist

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
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.48.0 version
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.48.0 run

NOTICE.txt: go.mod
	@bash ./scripts/notice.sh

.PHONY: check-linceses
check-licenses:
	@go run github.com/elastic/go-licenser@v0.4.0 -d -exclude tf -exclude testing .
	@go run github.com/elastic/go-licenser@v0.4.0 -d -exclude tf -exclude testing -ext .java .
	@go run github.com/elastic/go-licenser@v0.4.0 -d -exclude tf -exclude testing -ext .js .


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
smoketest/run: build
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
