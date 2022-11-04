USER_NAME?=$(USER)
AWS_REGION?=us-west-2
MACHINE_TYPE?=t2.medium
LOAD_DURATION?=10
LAMBDA_RUNTIME?=python3.8
LAMBDA_TIMEOUT?=15

# TODO: @lahsivjar Add automation for terraform fmt and docs

build/$(LAMBDA_RUNTIME).zip:
	@mkdir -p build
	@cd functions/$(LAMBDA_RUNTIME) && ./build.sh

.PHONY: clean
clean:
	@rm -rf build/
	@rm -rf functions/python3.8/package/

.PHONY: bench
bench: TF_CMD=apply
bench: build/$(LAMBDA_RUNTIME).zip --validate-input --run-tf

.PHONY: destroy
destroy: TF_CMD=destroy
destroy: --validate-input --run-tf

.PHONY: terraform-init
terraform-init:
	@cd benchmarking && terraform init

check_defined = \
    $(strip $(foreach 1,$1, \
        $(call __check_defined,$1,$(strip $(value 2)))))
__check_defined = \
    $(if $(value $1),, \
      $(error Undefined $1$(if $2, ($2))))

.PHONY: --validate-input
--validate-input:
	$(call check_defined, APM_SERVER_URL, is required)
	$(call check_defined, APM_SECRET_TOKEN, is required)

.PHONY: --run-tf
--run-tf:
	@cd benchmarking && terraform $(TF_CMD) -auto-approve \
		-var 'resource_prefix=$(USER_NAME)' \
		-var 'apm_server_url=$(APM_SERVER_URL)' \
		-var 'apm_secret_token=$(APM_SECRET_TOKEN)' \
		-var 'aws_region=$(AWS_REGION)' \
		-var 'machine_type=$(MACHINE_TYPE)' \
		-var 'load_duration=$(LOAD_DURATION)' \
		-var 'lambda_runtime=$(LAMBDA_RUNTIME)' \
		-var 'lambda_timeout=$(LAMBDA_TIMEOUT)'
