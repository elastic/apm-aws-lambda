SHELL = /bin/bash -eo pipefail

GORELEASER_VERSION = "v2.13.3"
GOLANGCI_LINT_VERSION = "v1.64.4"
export DOCKER_IMAGE_NAME = observability/apm-lambda-extension
export DOCKER_REGISTRY = docker.elastic.co

clean:
	@rm -rf dist/
	@docker image ls "$(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME)*" -aq | xargs -I {} docker rmi --force {} || true

.PHONY: dist
dist:
	@go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION) release --snapshot --clean

.PHONY: zip
zip:
	@go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION) release --snapshot --clean --skip=docker

build:
	@go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION) build --snapshot --clean

.PHONY: release
release:
	go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION) release --clean

.PHONY: release-skip-docker
release-skip-docker:
	go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION) release --clean --skip=docker

.PHONY: release-notes
release-notes:
	@./.ci/release-github.sh

.PHONY: test
test:
	@go test -v ./...

.PHONY: lint-prep
lint-prep:
	@go mod tidy && git diff --exit-code

.PHONY: lint
lint:
	@if [ "$(CI)" != "" ]; then go run github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) version; fi
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --build-tags tools

MODULE_DEPS=$(sort $(shell go list -deps -f "{{with .Module}}{{if not .Main}}{{.Path}}{{end}}{{end}}" .))

NOTICE.txt: go.mod
	go list -m -json $(MODULE_DEPS) | go tool go.elastic.co/go-licence-detector \
		-includeIndirect \
		-rules scripts/rules.json \
		-noticeTemplate scripts/templates/NOTICE.txt.tmpl \
		-noticeOut NOTICE.txt \
		-depsTemplate scripts/templates/dependencies.asciidoc.tmpl \
		-depsOut dependencies.asciidoc

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
