# ====================================================================================
# Setup Project

PROJECT_NAME ?= provider-anthropic
PROJECT_REPO ?= github.com/jonasz-lasut/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64 linux_arm64

# -include will silently skip missing files, which allows us
# to load those files with a target in the Makefile. If only
# "include" was used, the make command would fail and refuse
# to run a target until the include commands succeeded.
-include build/makelib/common.mk

# ====================================================================================
# Setup Output

-include build/makelib/output.mk

# ====================================================================================
# Setup Go

# Set a sane default so that the nprocs calculation below is less noisy on the initial
# loading of this file
NPROCS ?= 1

# each of our test suites starts a kube-apiserver and running many test suites in
# parallel can lead to high CPU utilization. by default we reduce the parallelism
# to half the number of CPU cores.
GO_TEST_PARALLEL := $(shell echo $$(( $(NPROCS) / 2 )))

GO_REQUIRED_VERSION ?= 1.26
GOLANGCILINT_VERSION ?= 2.13.2
GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/provider
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.Version=$(VERSION)
GO_SUBDIRS += cmd internal apis
-include build/makelib/golang.mk

# ====================================================================================
# Setup Kubernetes tools

KIND_VERSION = v0.31.0
UPTEST_VERSION = v2.2.0
# crddiff ships in the upbound/uptest module; crossplane/uptest does not carry it.
CRDDIFF_VERSION = v0.12.1
CROSSPLANE_CLI_VERSION = v2.2.1
# for e2e testing
CROSSPLANE_VERSION = 2.2.1
-include build/makelib/k8s_tools.mk

# ====================================================================================
# Setup Images

REGISTRY_ORGS ?= ghcr.io/jonasz-lasut
IMAGES = $(PROJECT_NAME)
-include build/makelib/imagelight.mk

# ====================================================================================
# Setup XPKG

XPKG_REG_ORGS ?= ghcr.io/jonasz-lasut
# NOTE(hasheddan): skip promoting on xpkg.crossplane.io as channel tags are
# inferred.
XPKG_REG_ORGS_NO_PROMOTE ?= ghcr.io/jonasz-lasut
XPKGS = $(PROJECT_NAME)
-include build/makelib/xpkg.mk

# ====================================================================================
# Fallthrough

# run `make help` to see the targets and options

# We want submodules to be set up the first time `make` is run.
# We manage the build/ folder and its Makefiles as a submodule.
# The first time `make` is run, the includes of build/*.mk files will
# all fail, and this target will be run. The next time, the default as defined
# by the includes will be run instead.
fallthrough: submodules
	@echo Initial setup complete. Running make again . . .
	@make

# NOTE(hasheddan): we force image building to happen prior to xpkg build so that
# we ensure image is present in daemon.
xpkg.build.provider-anthropic: do.build.images

# ====================================================================================
# Targets

# NOTE: the build submodule currently overrides XDG_CACHE_HOME in order to
# force the Helm 3 to use the .work/helm directory. This causes Go on Linux
# machines to use that directory as the build cache as well. We should adjust
# this behavior in the build submodule because it is also causing Linux users
# to duplicate their build cache, but for now we just make it easier to identify
# its location in CI so that we cache between builds.
go.cachedir:
	@go env GOCACHE

go.mod.cachedir:
	@go env GOMODCACHE

# Generate a coverage report for cobertura applying exclusions on
# - generated file
cobertura:
	@cat $(GO_TEST_OUTPUT)/coverage.txt | \
		grep -v zz_ | \
		$(GOCOVER_COBERTURA) > $(GO_TEST_OUTPUT)/cobertura-coverage.xml

# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

# This is for running out-of-cluster locally, and is for convenience. Running
# this make target will print out the command which was used. For more control,
# try running the binary directly with different arguments.
run: go.build
	@$(INFO) Running Crossplane locally out-of-cluster . . .
	@# To see other arguments that can be provided, run the command with --help instead
	$(GO_OUT_DIR)/provider --debug

# ====================================================================================
# End to End Testing
CROSSPLANE_NAMESPACE = crossplane-system
-include build/makelib/local.xpkg.mk
-include build/makelib/controlplane.mk

# This target requires the following environment variables to be set:
# - UPTEST_EXAMPLE_LIST, a comma-separated list of examples to test
#   To ensure the proper functioning of the end-to-end test resource pre-deletion hook, it is crucial to arrange your resources appropriately.
#   You can check the basic implementation here: https://github.com/crossplane/uptest/blob/main/internal/templates/03-delete.yaml.tmpl.
# - UPTEST_CLOUD_CREDENTIALS (optional), multiple sets of AWS IAM User credentials specified as key=value pairs.
#   The support keys are currently `DEFAULT` and `PEER`. So, an example for the value of this env. variable is:
#   DEFAULT='[default]
#   aws_access_key_id = REDACTED
#   aws_secret_access_key = REDACTED'
#   PEER='[default]
#   aws_access_key_id = REDACTED
#   aws_secret_access_key = REDACTED'
#   The associated `ProviderConfig`s will be named as `default` and `peer`.
# - UPTEST_DATASOURCE_PATH (optional), please see https://github.com/crossplane/uptest#injecting-dynamic-values-and-datasource
uptest: $(UPTEST) $(KUBECTL) $(CHAINSAW) $(CROSSPLANE_CLI)
	@$(INFO) running automated tests
	@KUBECTL=$(KUBECTL) CHAINSAW=$(CHAINSAW) CROSSPLANE_CLI=$(CROSSPLANE_CLI) CROSSPLANE_NAMESPACE=$(CROSSPLANE_NAMESPACE) $(UPTEST) e2e "${UPTEST_EXAMPLE_LIST}" --data-source="${UPTEST_DATASOURCE_PATH}" --setup-script=cluster/test/setup.sh --default-conditions="Ready,Synced" || $(FAIL)
	@$(OK) running automated tests

# The Kind cluster signs ServiceAccount tokens with a keypair kept outside the
# cluster so its OIDC issuer and JWKS survive recreation: an Anthropic
# federation issuer registered once (see README, workload identity federation)
# keeps trusting the provider after `make controlplane.down` and in CI, which
# writes the same key from a repository secret. The private key can mint tokens
# the organization trusts, so it is gitignored.
KIND_SA_KEY_DIR ?= cluster/local/pki
KIND_CONFIG_TEMPLATE ?= cluster/local/kind.yaml

$(KIND_SA_KEY_DIR)/sa.key:
	@$(INFO) generating the Kind ServiceAccount signing key in $(KIND_SA_KEY_DIR)
	@mkdir -p $(KIND_SA_KEY_DIR)
	@openssl genrsa -out $@ 2048 2>/dev/null
	@$(OK) generating the Kind ServiceAccount signing key in $(KIND_SA_KEY_DIR)

# The public half is derived, so CI only needs the private key (written from
# the KIND_SA_KEY repository secret before make e2e).
$(KIND_SA_KEY_DIR)/sa.pub: $(KIND_SA_KEY_DIR)/sa.key
	@openssl rsa -in $< -pubout -out $@ 2>/dev/null

# Creates the Kind cluster with the persistent signing keypair before the
# makelib's controlplane.up runs, which then only installs Crossplane.
kind.up: $(KIND) $(KIND_SA_KEY_DIR)/sa.pub
	@$(KIND) get kubeconfig --name $(KIND_CLUSTER_NAME) >/dev/null 2>&1 || { \
		$(INFO) creating Kind cluster $(KIND_CLUSTER_NAME) with the persistent signing keypair; \
		mkdir -p $(WORK_DIR); \
		sed "s|__PKI_DIR__|$(abspath $(KIND_SA_KEY_DIR))|g" $(KIND_CONFIG_TEMPLATE) > $(WORK_DIR)/kind.yaml; \
		$(KIND) create cluster --name=$(KIND_CLUSTER_NAME) --config $(WORK_DIR)/kind.yaml; \
	}

controlplane.up: kind.up

local-deploy: build controlplane.up local.xpkg.deploy.provider.$(PROJECT_NAME)
	@$(INFO) running locally built provider
	@$(KUBECTL) wait provider.pkg $(PROJECT_NAME) --for condition=Healthy --timeout 5m
	@$(KUBECTL) -n crossplane-system wait --for=condition=Available deployment --all --timeout=5m
	@$(OK) running locally built provider

e2e: local-deploy uptest

# Base ref that crddiff compares changed CRDs against. GITHUB_BASE_REF is set
# on pull_request events; locally the default is origin/main.
CRDDIFF_BASE_REF ?= origin/$(or $(GITHUB_BASE_REF),main)
# Space-separated CRD paths to check. CI passes the files changed by the pull
# request; locally every CRD that differs from the base ref is checked.
MODIFIED_CRD_LIST ?= $(shell git diff --name-only $(CRDDIFF_BASE_REF) -- package/crds/)

# Advisory: reports breaking OpenAPI v3 schema changes per CRD but never fails.
# No --enable-upjet-extensions: it presumes Upjet's spec.forProvider CEL rules
# and refuses to load the hand-written ProviderConfig CRDs.
crddiff:
	@$(INFO) Checking breaking CRD schema changes against $(CRDDIFF_BASE_REF)
	@for crd in $(MODIFIED_CRD_LIST); do \
		if ! git cat-file -e "$(CRDDIFF_BASE_REF):$${crd}" 2>/dev/null; then \
			echo "CRD $${crd} does not exist at $(CRDDIFF_BASE_REF). Skipping..." ; \
			continue ; \
		fi ; \
		echo "Checking $${crd} for breaking API changes..." ; \
		if ! changes_detected=$$(set -o pipefail; go run github.com/upbound/uptest/cmd/crddiff@$(CRDDIFF_VERSION) revision <(git cat-file -p "$(CRDDIFF_BASE_REF):$${crd}") "$${crd}" 2>&1 | sed '/^exit status [0-9]*$$/d') ; then \
			printf "\033[31m"; echo "Breaking change detected in $${crd}!"; printf "\033[0m" ; \
			echo "$${changes_detected}" ; \
			echo ; \
			if [ -n "$${GITHUB_ACTIONS}" ]; then \
				echo "::warning file=$${crd}::Breaking CRD schema change detected, see the job summary" ; \
				{ echo "### Breaking CRD schema change: $${crd}" ; echo '```' ; echo "$${changes_detected}" ; echo '```' ; echo ; } >> "$${GITHUB_STEP_SUMMARY}" ; \
			fi ; \
		fi ; \
	done
	@$(OK) Checking breaking CRD schema changes

.PHONY: cobertura submodules fallthrough run crds.clean crddiff kind.up

# ====================================================================================
# Special Targets

define CROSSPLANE_MAKE_HELP
Crossplane Targets:
    cobertura             Generate a coverage report for cobertura applying exclusions on generated files.
    submodules            Update the submodules, such as the common build scripts.
    run                   Run crossplane locally, out-of-cluster. Useful for development.

endef
# The reason CROSSPLANE_MAKE_HELP is used instead of CROSSPLANE_HELP is because the crossplane
# binary will try to use CROSSPLANE_HELP if it is set, and this is for something different.
export CROSSPLANE_MAKE_HELP

crossplane.help:
	@echo "$$CROSSPLANE_MAKE_HELP"

help-special: crossplane.help

.PHONY: crossplane.help help-special

# TODO(negz): Update CI to use these targets.
vendor: modules.download
vendor.check: modules.check
