### CONFIG #
#
SHELL := bash # we want bash behaviour in all shell invocations
PLATFORM := $(shell uname)
platform := $(shell echo $(PLATFORM) | tr A-Z a-z)
ifeq ($(PLATFORM),Darwin)
platform_alt = macOS
else
platform_alt = $(platform)
endif
ARCH := $(shell uname -m)

# https://stackoverflow.com/questions/4842424/list-of-ansi-color-escape-sequences
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
MAGENTA := \033[0;35m
CYAN := \033[0;36m
BOLD := \033[1m
RESET := \033[0m
INFO := $(CYAN)
WARN := $(MAGENTA)

XDG_CONFIG_HOME ?= $(CURDIR)/.config
env::
	@echo 'export XDG_CONFIG_HOME="$(XDG_CONFIG_HOME)"'
KUBECONFIG_DIR = $(XDG_CONFIG_HOME)/kubectl
KUBECONFIG = $(KUBECONFIG_DIR)/config
$(KUBECONFIG_DIR):
	mkdir -p $(@)
env::
	@echo 'export KUBECONFIG="$(KUBECONFIG)"'

LOCAL_BIN := $(CURDIR)/bin
$(LOCAL_BIN):
	mkdir -p $@
env::
	@echo 'export PATH="$(LOCAL_BIN):$$PATH"'



### DEPS #
#

GCLOUD_SDK_VERSION := 365.0.0
GCLOUD_BIN := gcloud-$(GCLOUD_SDK_VERSION)-$(PLATFORM)-x86_64
GCLOUD := $(LOCAL_BIN)/$(GCLOUD_BIN)
GCLOUD_SDK_FILE := google-cloud-sdk-$(GCLOUD_SDK_VERSION)-$(PLATFORM)-x86_64.tar.gz
GCLOUD_SDK_URL := https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/$(GCLOUD_SDK_FILE)
$(GCLOUD): | $(CURL) $(LOCAL_BIN)
	$(CURL) --progress-bar --fail --location --output $(LOCAL_BIN)/$(GCLOUD_SDK_FILE) "$(GCLOUD_SDK_URL)"
	cd $(LOCAL_BIN) && \
	tar -xzf $(GCLOUD_SDK_FILE) && \
	rm -rf $(GCLOUD_SDK_FILE) && \
	cd google-cloud-sdk && \
	./install.sh --quiet --usage-reporting=false
	ln -sf $(LOCAL_BIN)/google-cloud-sdk/bin/gcloud $(GCLOUD)
	ln -sf $(GCLOUD) $(LOCAL_BIN)/gcloud
	@printf "$(INFO)ko requires $(BOLD)docker-credential-gcloud$(RESET)\n"
	PATH=$(GOOGLE_CLOUD_SDK_BIN):$$PATH $(GCLOUD) auth configure-docker
	@printf "$(RED)Remember to run: $(BOLD)make .env -B && . .env$(RESET)\n"

GOOGLE_CLOUD_SDK_BIN := $(CURDIR)/bin/google-cloud-sdk/bin
env::
	@echo 'export PATH="$(GOOGLE_CLOUD_SDK_BIN):$$PATH"'
.PHONY: gcloud
gcloud: $(GCLOUD)
.PHONY: releases-gcloud
releases-gcloud:
	$(OPEN) https://cloud.google.com/sdk/docs/quickstart

KO_RELEASES := https://github.com/google/ko/releases
KO_VERSION = 0.9.3
KO_URL := $(KO_RELEASES)/download/v$(KO_VERSION)/$(notdir $(KO_BIN_DIR)).tar.gz
KO_BIN_DIR := $(LOCAL_BIN)/ko_$(KO_VERSION)_$(PLATFORM)_x86_64
KO := $(KO_BIN_DIR)/ko
$(KO): | $(CURL) $(LOCAL_BIN) $(GCLOUD)
	$(CURL) --progress-bar --fail --location --output $(KO_BIN_DIR).tar.gz "$(KO_URL)"
	mkdir -p $(KO_BIN_DIR) && tar zxvf $(KO_BIN_DIR).tar.gz -C $(KO_BIN_DIR)
	touch $(KO)
	chmod +x $(KO)
	$(KO) version | grep $(KO_VERSION)
	ln -sf $(KO) $(LOCAL_BIN)/ko
.PHONY: ko
ko: $(KO)
.PHONY: releases-ko
releases-ko:
	$(OPEN) $(KO_RELEASES)

KIND_RELEASES := https://github.com/kubernetes-sigs/kind/releases
KIND_VERSION = 0.11.1
KIND_URL := $(KIND_RELEASES)/download/v$(KIND_VERSION)/kind-$(platform)-amd64
KIND := $(LOCAL_BIN)/kind_$(KIND_VERSION)_$(platform)_amd64
$(KIND): | $(CURL) $(LOCAL_BIN)
	$(CURL) --progress-bar --fail --location --output $(KIND) "$(KIND_URL)"
	touch $(KIND)
	chmod +x $(KIND)
	$(KIND) version | grep $(KIND_VERSION)
	ln -sf $(KIND) $(LOCAL_BIN)/kind
.PHONY: kind
kind: $(KIND)
.PHONY: releases-kind
releases-kind:
	$(OPEN) $(KIND_RELEASES)

# The envsubst that comes with gettext does not support this,
# using this Go version instead: https://github.com/a8m/envsubst#docs
ENVSUBST_RELEASES := https://github.com/a8m/envsubst/releases
ENVSUBST_VERSION := 1.2.0
ENVSUBST_URL := $(ENVSUBST_RELEASES)/download/v$(ENVSUBST_VERSION)/envsubst-$(PLATFORM)-x86_64
ENVSUBST := $(LOCAL_BIN)/envsubst-$(ENVSUBST_VERSION)-$(PLATFORM)-x86_64
# We want to fail if variables are not set or empty.
ENVSUBST_SAFE := $(ENVSUBST) -no-unset -no-empty
$(ENVSUBST): | $(CURL) $(LOCAL_BIN)
	$(CURL) --progress-bar --fail --location --output $(ENVSUBST) "$(ENVSUBST_URL)"
	touch $(ENVSUBST)
	chmod +x $(ENVSUBST)
	ln -sf $(ENVSUBST) $(LOCAL_BIN)/envsubst
.PHONY: envsubst
envsubst: $(ENVSUBST)
.PHONY: releases-envsubst
releases-envsubst:
	$(OPEN) $(ENVSUBST_RELEASES)

KUBECTL_RELEASES := https://github.com/kubernetes/kubernetes/tags
# Keep this in sync with KIND_K8s_VERSION
KUBECTL_VERSION = 1.20.7
KUBECTL_BIN := kubectl-$(KUBECTL_VERSION)-$(platform)-amd64
KUBECTL_URL := https://storage.googleapis.com/kubernetes-release/release/v$(KUBECTL_VERSION)/bin/$(platform)/amd64/kubectl
KUBECTL := $(LOCAL_BIN)/$(KUBECTL_BIN)
$(KUBECTL): | $(CURL) $(LOCAL_BIN)
	$(CURL) --progress-bar --fail --location --output $(KUBECTL) "$(KUBECTL_URL)"
	touch $(KUBECTL)
	chmod +x $(KUBECTL)
	$(KUBECTL) version | grep $(KUBECTL_VERSION)
	ln -sf $(KUBECTL) $(LOCAL_BIN)/kubectl
.PHONY: kubectl
kubectl: $(KUBECTL)
.PHONY: releases-kubectl
releases-kubectl:
	$(OPEN) $(KUBECTL_RELEASES)
K_CMD ?= apply
# Dump all objects (do not apply) if DEBUG variable is set
ifneq (,$(DEBUG))
K_CMD = create --dry-run=client --output=yaml
endif

K9S_RELEASES := https://github.com/derailed/k9s/releases
K9S_VERSION := 0.25.6
K9S_BIN_DIR := $(LOCAL_BIN)/k9s-$(K9S_VERSION)-$(platform)-x86_64
K9S_URL := $(K9S_RELEASES)/download/v$(K9S_VERSION)/k9s_$(platform)_x86_64.tar.gz
K9S := $(K9S_BIN_DIR)/k9s
$(K9S): | $(CURL) $(LOCAL_BIN) $(KUBECTL)
	$(CURL) --progress-bar --fail --location --output $(K9S_BIN_DIR).tar.gz "$(K9S_URL)"
	mkdir -p $(K9S_BIN_DIR) && tar zxf $(K9S_BIN_DIR).tar.gz -C $(K9S_BIN_DIR)
	touch $(K9S)
	chmod +x $(K9S)
	$(K9S) version | grep $(K9S_VERSION)
	ln -sf $(K9S) $(LOCAL_BIN)/k9s
.PHONY: releases-k9s
releases-k9s:
	$(OPEN) $(K9S_RELEASES)
.PHONY: k9s
K9S_ARGS ?= --all-namespaces
k9s: | $(KUBECONFIG) $(K9S) ## Terminal ncurses UI for K8s
	$(K9S) $(K9S_ARGS)

GH_RELEASES := https://github.com/cli/cli/releases
GH_VERSION := 2.2.0
GH_DIR := $(LOCAL_BIN)/gh_$(GH_VERSION)_$(platform_alt)_amd64
GH_URL := $(GH_RELEASES)/download/v$(GH_VERSION)/$(notdir $(GH_DIR)).tar.gz
GH := $(GH_DIR)/bin/gh
$(GH): | $(CURL) $(LOCAL_BIN)
	$(CURL) --progress-bar --fail --location --output $(GH_DIR).tar.gz $(GH_URL)
	tar zxf $(GH_DIR).tar.gz -C $(LOCAL_BIN)
	touch $(GH)
	chmod +x $(GH)
	$(GH) version | grep $(GH_VERSION)
	ln -sf $(GH) $(LOCAL_BIN)/gh
.PHONY: gh
gh: $(GH)
.PHONY: releases-gh
releases-gh:
	$(OPEN) $(GH_RELEASES)



### TARGETS #
#
.DEFAULT_GOAL := help

.PHONY: env
env:: ## Configure shell env - eval "$(make env)" OR rm .env && make .env && source .env
	@echo 'unalias m 2>/dev/null || true ; alias m=make'
.env:
	$(MAKE) --file $(lastword $(MAKEFILE_LIST)) --no-print-directory env SILENT="1>/dev/null 2>&1" > .env

CURL ?= /usr/bin/curl
$(CURL):
	@which $(CURL) \
	|| ( printf "$(RED)$(BOLD)$(CURL)$(RESET)$(RED) is missing, install $(BOLD)curl$(RESET)\n" ; exit 1)
.PHONY: curl
curl: $(CURL)

HELP_TARGET_DEPTH ?= \#\#
.PHONY: help
help:
	@awk -F':+ |$(HELP_TARGET_DEPTH)' '/^[^.][0-9a-zA-Z._%-]+:+.+$(HELP_TARGET_DEPTH).+$$/ { printf "\033[36m%-26s\033[0m %s\n", $$1, $$3 }' $(MAKEFILE_LIST) \
	| sort

define MAKE_TARGETS
  awk -F':+' '/^[^.%\t_][0-9a-zA-Z._%-]*:+.*$$/ { printf "%s\n", $$1 }' $(MAKEFILE_LIST)
endef

define BASH_AUTOCOMPLETE
  complete -W \"$$($(MAKE_TARGETS) | sort | uniq)\" make gmake m
endef

.PHONY: bash-autocomplete
bash-autocomplete:
	@echo "$(BASH_AUTOCOMPLETE)"
env:: bash-autocomplete

.PHONY: dep-update
dep-update: ## Update any dependency
	@printf "Update dep in go.mod by running e.g. $(BOLD)go get -d github.com/rabbitmq/messaging-topology-operator@v1.2.1$(RESET)\n" \
	; read -rp " (press any key when done)" -n 1
	$(CURDIR)/hack/update-deps.sh
	$(CURDIR)/hack/update-codegen.sh

.PHONY: test-unit
test-unit: ## Run unit tests
	@printf "$(INFO)Starting point: $(BOLD).github/workflows/knative-go-test.yaml$(RESET)\n"
	go test -race $(GOTEST) ./... \
	| grep -v "no test files"

.PHONY: test-unit-uncached
test-unit-uncached: GOTEST = -count=1
test-unit-uncached: test-unit

SYSTEM_NAMESPACE = knative-eventing
export SYSTEM_NAMESPACE
RABBITMQ_SYSTEM_NAMESPACE = rabbitmq-system
export RABBITMQ_SYSTEM_NAMESPACE
RABBITMQ_SOURCE_NAMESPACE = knative-sources
export RABBITMQ_SOURCE_NAMESPACE
CERT_MANAGER_NAMESPACE = cert-manager
export CERT_MANAGER_NAMESPACE
KIND_CLUSTER_NAME = eventing-rabbitmq-e2e
export KIND_CLUSTER_NAME
env::
	@echo 'export KIND_CLUSTER_NAME="$(KIND_CLUSTER_NAME)"'
KO_DOCKER_REPO = kind.local
env::
	@echo 'export KO_DOCKER_REPO="$(KO_DOCKER_REPO)"'
export KO_DOCKER_REPO
MIN_SUPPORTED_K8s_VERSION = 1.20
KIND_K8s_VERSION = $(MIN_SUPPORTED_K8s_VERSION)
export KIND_K8s_VERSION
# Find the corresponding version digest in https://github.com/kubernetes-sigs/kind/releases
KIND_K8s_DIGEST = sha256:cbeaf907fc78ac97ce7b625e4bf0de16e3ea725daf6b04f930bd14c67c671ff9
export KIND_K8s_DIGEST

.PHONY: kind-cluster
kind-cluster: | $(KIND) $(ENVSUBST)
	( $(KIND) get clusters | grep $(KIND_CLUSTER_NAME) ) \
	|| ( cat $(CURDIR)/test/e2e/kind.yaml \
	     | $(ENVSUBST_SAFE) \
	     | $(KIND) create cluster --name $(KIND_CLUSTER_NAME) --config - )

$(KUBECONFIG): | kind-cluster $(KUBECONFIG_DIR)
	$(KIND) get kubeconfig --name $(KIND_CLUSTER_NAME) > $(KUBECONFIG)

# https://github.com/rabbitmq/cluster-operator/releases
RABBITMQ_CLUSTER_OPERATOR_VERSION := 1.10.0
install-rabbitmq-cluster-operator: | $(KUBECONFIG) $(KUBECTL)
	$(KUBECTL) $(K_CMD) --filename \
		https://github.com/rabbitmq/cluster-operator/releases/download/v$(RABBITMQ_CLUSTER_OPERATOR_VERSION)/cluster-operator.yml

# https://github.com/jetstack/cert-manager/releases
CERT_MANAGER_VERSION := 1.5.3
install-cert-manager: | $(KUBECONFIG) $(KUBECTL)
	$(KUBECTL) $(K_CMD) --filename \
		https://github.com/jetstack/cert-manager/releases/download/v$(CERT_MANAGER_VERSION)/cert-manager.yaml
	$(KUBECTL) wait --for=condition=available deploy/cert-manager-webhook --timeout=60s --namespace $(CERT_MANAGER_NAMESPACE)

# https://github.com/rabbitmq/messaging-topology-operator/releases
RABBITMQ_TOPOLOGY_OPERATOR_VERSION := 1.2.1
install-rabbitmq-topology-operator: | install-cert-manager $(KUBECTL)
	$(KUBECTL) $(K_CMD) --filename \
		https://github.com/rabbitmq/messaging-topology-operator/releases/download/v$(RABBITMQ_TOPOLOGY_OPERATOR_VERSION)/messaging-topology-operator-with-certmanager.yaml

# https://github.com/knative/eventing/releases
KNATIVE_EVENTING_VERSION := 1.0.0
install-knative-eventing: | $(KUBECONFIG) $(KUBECTL)
	$(KUBECTL) $(K_CMD) --filename \
		https://github.com/knative/eventing/releases/download/knative-v$(KNATIVE_EVENTING_VERSION)/eventing-crds.yaml
	$(KUBECTL) $(K_CMD) --filename \
		https://github.com/knative/eventing/releases/download/knative-v$(KNATIVE_EVENTING_VERSION)/eventing-core.yaml
	$(KUBECTL) wait --for=condition=available deploy/eventing-controller --timeout=30s --namespace $(SYSTEM_NAMESPACE)
	$(KUBECTL) wait --for=condition=available deploy/eventing-webhook --timeout=30s --namespace $(SYSTEM_NAMESPACE)

install-knative-eventing-rabbitmq: | $(KUBECONFIG) $(KO) install-knative-eventing
	$(KO) apply --filename config/broker
	$(KUBECTL) wait --for=condition=available deploy/rabbitmq-broker-controller --timeout=30s --namespace $(SYSTEM_NAMESPACE)
	$(KUBECTL) wait --for=condition=available deploy/rabbitmq-broker-webhook --timeout=30s --namespace $(SYSTEM_NAMESPACE)
	$(KO) apply --filename config/source
	$(KUBECTL) wait --for=condition=available deploy/pingsource-mt-adapter --timeout=30s --namespace knative-eventing

.PHONY: e2e-setup
e2e-setup: install-rabbitmq-cluster-operator install-rabbitmq-topology-operator install-knative-eventing-rabbitmq ## Setup environment for e2e testing
	@printf "$(INFO)Starting point: $(BOLD).github/workflows/kind-e2e.yaml$(RESET)\n"
	@printf "$(INFO)This is slow - $(BOLD)33s cached$(RESET)$(INFO) - so it is kept as a non-explicit dependency for any e2e test$(RESET)\n"

.PHONY: e2e-setup-rm
e2e-setup-rm: | $(KIND)
	$(KIND) delete cluster --name $(KIND_CLUSTER_NAME)

.PHONY: test-e2e-publish
test-e2e-publish: ## Run TestKoPublish end-to-end tests
	@printf "$(WARN)Remember to run $(BOLD)make e2e-setup$(RESET)$(WARN) if needed$(RESET)\n"
	go test -v -race -count=1 -timeout=15m -tags=e2e ./test/e2e/... -run 'TestKoPublish' \
	| grep -v "no test files"

.PHONY: test-e2e-broker
test-e2e-broker: ## Run Broker end-to-end tests
	@printf "$(WARN)Remember to run $(BOLD)make e2e-setup$(RESET)$(WARN) if needed$(RESET)\n"
	@printf "$(WARN)$(BOLD)rabbitmqcluster$(RESET)$(WARN) has large resource requirements 🤔$(RESET)\n"
	go test -v -race -count=1 -timeout=15m -tags=e2e ./test/e2e/... -run 'Test.*Broker.*' \
	| grep -v "no test files"

.PHONY: test-e2e-source
test-e2e-source: ## Run Source end-to-end tests
	@printf "$(WARN)Remember to run $(BOLD)make e2e-setup$(RESET)$(WARN) if needed$(RESET)\n"
	go test -v -race -count=1 -timeout=15m -tags=e2e ./test/e2e/... -run 'Test.*Source.*' \
	| grep -v "no test files"

.PHONY: test-e2e
test-e2e: test-e2e-publish test-e2e-broker test-e2e-source

BUILD_TAGS=e2e
.PHONY: build
build: ## Build binaries with e2e tags
	@printf "$(INFO)Starting point: $(BOLD).github/workflows/knative-go-build.yaml$(RESET)\n"
	go test -vet=off -tags "$(BUILD_TAGS)" -exec echo  ./... \
	| grep -v "no test files"
