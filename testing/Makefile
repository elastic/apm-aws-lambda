USER_NAME?=$(USER)
AWS_REGION?=us-west-2
MACHINE_TYPE?=t2.medium
STACK_VERSION?=latest
LOAD_DURATION?=10
LOAD_ARRIVAL_RATE?=50
LAMBDA_RUNTIME?=python3.8
LAMBDA_TIMEOUT?=15
LAMBDA_MEMORY_SIZE?=128
LAMBDA_RUNTIME_NORM=$(subst '.','_',$(LAMBDA_RUNTIME))

# TODO: @lahsivjar Add automation for terraform fmt and docs

build/$(LAMBDA_RUNTIME_NORM).zip:
	@mkdir -p build
	@cd functions/$(LAMBDA_RUNTIME_NORM) && ./build.sh

.PHONY: clean
clean:
	@rm -rf build/
	@rm -rf functions/python3.8/package/
	@rm -rf functions/go1.x/main

.PHONY: bench
bench: TF_CMD=apply
bench: build/$(LAMBDA_RUNTIME_NORM).zip --run-tf

.PHONY: destroy
destroy: TF_CMD=destroy
destroy: --run-tf

.PHONY: terraform-init
terraform-init:
	@cd benchmarking && terraform init

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
		-var 'stack_version=$(STACK_VERSION)'
