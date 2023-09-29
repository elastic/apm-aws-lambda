USER_NAME?=$(USER)
AWS_REGION?=us-west-2
MACHINE_TYPE?=t2.medium
STACK_VERSION?=latest
LOAD_DURATION?=10
LOAD_ARRIVAL_RATE?=50
LAMBDA_RUNTIME?=python3.9
LAMBDA_TIMEOUT?=15
LAMBDA_MEMORY_SIZE?=128
CUSTOM_LAMBDA_EXTENSION_ARN?=
LAMBDA_RUNTIME_NORM=$(subst '.','_',$(LAMBDA_RUNTIME))
LAMBDA_APM_AWS_EXTENSION_PATH=$(shell echo $(wildcard ../dist/*-linux-amd64.zip) | cut -d' ' -f1)
TERRAFORM_DOCS=../build/terraform-docs
TERRAFORM_DIRS=$(shell find tf-modules -type d -mindepth 1 -maxdepth 1)

build/$(LAMBDA_RUNTIME_NORM).zip:
	@mkdir -p build
	@cd functions/$(LAMBDA_RUNTIME_NORM) && ./build.sh

.PHONY: clean
clean:
	@rm -rf build/
	@rm -rf functions/python3.9/package/
	@rm -rf functions/go1.x/main

.PHONY: bench
bench: TF_CMD=apply
bench: build/$(LAMBDA_RUNTIME_NORM).zip --run-tf

.PHONY: destroy
destroy: TF_CMD=destroy
destroy: --run-tf

.PHONY: fmt
fmt:
	@terraform fmt -write $(TERRAFORM_DIRS)

.PHONY: lint
lint:
	@terraform fmt -write=false $(TERRAFORM_DIRS)

.PHONY: terraform-init
terraform-init:
	@cd benchmarking && terraform init

$(TERRAFORM_DOCS): ../tools/go.mod
	@go build -o $@ -modfile=$< github.com/terraform-docs/terraform-docs

.PHONY: docs
docs: $(TERRAFORM_DOCS) $(addsuffix /README.md,$(TERRAFORM_DIRS))

%/README.md: force
	$(TERRAFORM_DOCS) markdown --hide-empty --header-from header.md --output-file=README.md --output-mode replace $(subst README.md,,$@)

force: ;

.PHONY: --run-tf
--run-tf:
	@cd benchmarking && terraform $(TF_CMD) -auto-approve \
		-var 'resource_prefix=$(USER_NAME)' \
		-var 'aws_region=$(AWS_REGION)' \
		-var 'machine_type=$(MACHINE_TYPE)' \
		-var 'load_duration=$(LOAD_DURATION)' \
		-var 'load_arrival_rate=$(LOAD_ARRIVAL_RATE)' \
		-var 'lambda_runtime=$(LAMBDA_RUNTIME)' \
		-var 'lambda_timeout=$(LAMBDA_TIMEOUT)' \
		-var 'lambda_memory_size=$(LAMBDA_MEMORY_SIZE)' \
		-var 'stack_version=$(STACK_VERSION)' \
		-var 'custom_lambda_extension_arn=$(CUSTOM_LAMBDA_EXTENSION_ARN)' \
		-var 'lambda_apm_aws_extension_path=../$(LAMBDA_APM_AWS_EXTENSION_PATH)'

