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
NORMAL := \033[0m
INFO := $(CYAN)
WARN := $(MAGENTA)

XDG_CONFIG_HOME ?= $(CURDIR)/.config
envrc::
	@echo 'export XDG_CONFIG_HOME="$(XDG_CONFIG_HOME)"'
KUBECONFIG_DIR = $(XDG_CONFIG_HOME)/kubectl
KUBECONFIG ?= $(KUBECONFIG_DIR)/config
$(KUBECONFIG_DIR):
	mkdir -p $(@)
envrc::
	@echo 'export KUBECONFIG="$(KUBECONFIG)"'

LOCAL_BIN := $(CURDIR)/bin
PATH := $(LOCAL_BIN):$(PATH)
export PATH
$(LOCAL_BIN):
	mkdir -p $@
envrc::
	@echo 'export PATH="$(PATH)"'



### DEPS #
#
ifeq ($(PLATFORM),Darwin)
OPEN := open
else
OPEN := xdg-open
endif

GCLOUD_SDK_VERSION := 367.0.0
GCLOUD_BIN := gcloud-$(GCLOUD_SDK_VERSION)-$(PLATFORM)-x86_64
GCLOUD := $(LOCAL_BIN)/$(GCLOUD_BIN)
GCLOUD_SDK_FILE := google-cloud-sdk-$(GCLOUD_SDK_VERSION)-$(PLATFORM)-x86_64.tar.gz
GCLOUD_SDK_URL := https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/$(GCLOUD_SDK_FILE)
GOOGLE_CLOUD_SDK_BIN := $(LOCAL_BIN)/google-cloud-sdk/bin
PATH := $(GOOGLE_CLOUD_SDK_BIN):$(PATH)
export PATH
$(GCLOUD): | $(CURL) $(LOCAL_BIN)
	$(CURL) --progress-bar --fail --location --output $(LOCAL_BIN)/$(GCLOUD_SDK_FILE) "$(GCLOUD_SDK_URL)"
	cd $(LOCAL_BIN) && \
	tar -xzf $(GCLOUD_SDK_FILE) && \
	rm -rf $(GCLOUD_SDK_FILE) && \
	cd google-cloud-sdk && \
	./install.sh --quiet --usage-reporting=false
	ln -sf $(LOCAL_BIN)/google-cloud-sdk/bin/gcloud $(GCLOUD)
	ln -sf $(GCLOUD) $(LOCAL_BIN)/gcloud
	@printf "$(INFO)ko requires $(BOLD)docker-credential-gcloud$(NORMAL)\n"
	$(GCLOUD) auth configure-docker
	@printf "$(RED)Remember to run: $(BOLD)make .envrc -B && . .envrc$(NORMAL)\n"

.PHONY: gcloud
gcloud: $(GCLOUD)
.PHONY: releases-gcloud
releases-gcloud:
	$(OPEN) https://cloud.google.com/sdk/docs/quickstart

KO_RELEASES := https://github.com/google/ko/releases
KO_VERSION := 0.9.3
KO_BIN_DIR := $(LOCAL_BIN)/ko_$(KO_VERSION)_$(PLATFORM)_x86_64
KO_URL := $(KO_RELEASES)/download/v$(KO_VERSION)/$(notdir $(KO_BIN_DIR)).tar.gz
KO := $(KO_BIN_DIR)/ko
$(KO): | $(CURL) $(LOCAL_BIN)
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
KIND_VERSION := 0.11.1
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
# Keep this in sync with KIND_K8S_VERSION
KUBECTL_VERSION := 1.21.9
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
K9S_VERSION := 0.25.12
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
k9s: | $(KUBECONFIG) $(K9S) ## Terminal ncurses UI for K8S
	$(K9S) $(K9S_ARGS)

GH_RELEASES := https://github.com/cli/cli/releases
GH_VERSION := 2.3.0
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

KN_RELEASES := https://github.com/knative/client/releases
KN_VERSION := 1.1.0
KN_BIN := kn-$(KN_VERSION)-$(platform)-amd64
KN_URL := $(KN_RELEASES)/download/knative-v$(KN_VERSION)/kn-$(platform)-amd64
KN := $(LOCAL_BIN)/$(KN_BIN)
$(KN): | $(CURL) $(LOCAL_BIN)
	$(CURL) --progress-bar --fail --location --output $(KN) "$(KN_URL)"
	touch $(KN)
	chmod +x $(KN)
	$(KN) version | grep $(KN_VERSION)
	ln -sf $(KN) $(LOCAL_BIN)/kn
.PHONY: kn
kn: $(KN)
.PHONY: releases-kn
releases-kn:
	$(OPEN) $(KN_RELEASES)



### TARGETS #
#
.DEFAULT_GOAL := help

.PHONY: envrc
envrc:: ## Configure shell envrc - eval "$(make envrc)" OR rm .envrc && make .envrc && source .envrc
	@echo 'unalias m 2>/dev/null || true ; alias m=make'
.envrc:
	$(MAKE) --file $(lastword $(MAKEFILE_LIST)) --no-print-directory envrc SILENT="1>/dev/null 2>&1" > .envrc

CURL ?= /usr/bin/curl
$(CURL):
	@which $(CURL) \
	|| ( printf "$(RED)$(BOLD)$(CURL)$(NORMAL)$(RED) is missing, install $(BOLD)curl$(NORMAL)\n" ; exit 1)
.PHONY: curl
curl: $(CURL)

HELP_TARGET_DEPTH ?= \#\#
.PHONY: help
help:
	@printf "\nIf this is your first time running this, remember to run: $(BOLD)make .envrc && source .envrc$(NORMAL)\n"
	@printf "Now just type $(BOLD)make <TAB>$(NORMAL) to enjoy shell autocompletion\n"
	@printf "By the way, $(BOLD)m$(NORMAL) is an alias for $(BOLD)make$(NORMAL)\n\n"
	@printf "Here is a list of all the make targets that you can run, e.g. $(BOLD)make test-e2e$(NORMAL) or $(BOLD)m test-e2e$(NORMAL)\n\n"
	@awk -F':+ |$(HELP_TARGET_DEPTH)' '/^[^.][0-9a-zA-Z._%-]+:+.+$(HELP_TARGET_DEPTH).+$$/ { printf "\033[36m%-34s\033[0m %s\n", $$1, $$3 }' $(MAKEFILE_LIST) \
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
envrc:: bash-autocomplete

.PHONY: go-dep-update
go-dep-update: ## Update any Go dependency
	@printf "Update dep in go.mod by running e.g. $(BOLD)go get -d github.com/rabbitmq/messaging-topology-operator@v1.2.1$(NORMAL)\n" \
	; read -rp " (press any key when done)" -n 1
	$(CURDIR)/hack/update-deps.sh
	$(CURDIR)/hack/update-codegen.sh

.PHONY: test-unit
test-unit: ## Run unit tests
	go test -race $(GOTEST) ./...

.PHONY: test-unit-uncached
test-unit-uncached: GOTEST = -count=1
test-unit-uncached: test-unit ## Run unit tests with no cache

SERVING_NAMESPACE = knative-serving
EVENTING_NAMESPACE = knative-eventing
SYSTEM_NAMESPACE = $(EVENTING_NAMESPACE)
export SYSTEM_NAMESPACE
RABBITMQ_SYSTEM_NAMESPACE = rabbitmq-system
export RABBITMQ_SYSTEM_NAMESPACE
RABBITMQ_SOURCE_NAMESPACE = knative-sources
export RABBITMQ_SOURCE_NAMESPACE
CERT_MANAGER_NAMESPACE = cert-manager
export CERT_MANAGER_NAMESPACE
KIND_CLUSTER_NAME ?= eventing-rabbitmq-e2e
export KIND_CLUSTER_NAME
envrc::
	@echo 'export KIND_CLUSTER_NAME="$(KIND_CLUSTER_NAME)"'
KO_DOCKER_REPO := kind.local
envrc::
	@echo 'export KO_DOCKER_REPO="$(KO_DOCKER_REPO)"'
export KO_DOCKER_REPO
MIN_SUPPORTED_K8S_VERSION := 1.21.2
KIND_K8S_VERSION ?= $(MIN_SUPPORTED_K8S_VERSION)
export KIND_K8S_VERSION
# Find the corresponding version digest in https://github.com/kubernetes-sigs/kind/releases
KIND_K8S_DIGEST ?= sha256:9d07ff05e4afefbba983fac311807b3c17a5f36e7061f6cb7e2ba756255b2be4
export KIND_K8S_DIGEST

.PHONY: kind-cluster
kind-cluster: | $(KIND) $(ENVSUBST)
	( $(KIND) get clusters | grep $(KIND_CLUSTER_NAME) ) \
	|| ( cat $(CURDIR)/test/e2e/kind.yaml \
	     | $(ENVSUBST_SAFE) \
	     | $(KIND) create cluster --name $(KIND_CLUSTER_NAME) --config  - )

$(KUBECONFIG): | $(KUBECONFIG_DIR)
	$(MAKE) --no-print-directory kind-cluster
	$(KIND) get kubeconfig --name $(KIND_CLUSTER_NAME) > $(KUBECONFIG)

.PHONY: kubeconfig
kubeconfig: $(KUBECONFIG)

# https://github.com/rabbitmq/cluster-operator/releases
RABBITMQ_CLUSTER_OPERATOR_VERSION ?= 1.11.1
.PHONY: install-rabbitmq-cluster-operator
install-rabbitmq-cluster-operator: | $(KUBECONFIG) $(KUBECTL) ## Install RabbitMQ Cluster Operator
	$(KUBECTL) $(K_CMD) --filename \
		https://github.com/rabbitmq/cluster-operator/releases/download/v$(RABBITMQ_CLUSTER_OPERATOR_VERSION)/cluster-operator.yml

# https://github.com/jetstack/cert-manager/releases
# ⚠️  You may want to keep this in sync with RABBITMQ_TOPOLOGY_OPERATOR_VERSION
# In other words, don't upgrade cert-manager to a version that was released AFTER RABBITMQ_TOPOLOGY_OPERATOR_VERSION
CERT_MANAGER_VERSION ?= 1.7.0
.PHONY: install-cert-manager
install-cert-manager: | $(KUBECONFIG) $(KUBECTL) ## Install Cert Manager - dependency of RabbitMQ Topology Operator
	$(KUBECTL) $(K_CMD) --filename \
		https://github.com/jetstack/cert-manager/releases/download/v$(CERT_MANAGER_VERSION)/cert-manager.yaml
	$(KUBECTL) wait --for=condition=available deploy/cert-manager-webhook --timeout=60s --namespace $(CERT_MANAGER_NAMESPACE)

# https://github.com/rabbitmq/messaging-topology-operator/releases
RABBITMQ_TOPOLOGY_OPERATOR_VERSION ?= 1.3.0
.PHONY: install-rabbitmq-topology-operator
install-rabbitmq-topology-operator: | install-cert-manager $(KUBECTL) ## Install RabbitMQ Topology Operator
	$(KUBECTL) $(K_CMD) --filename \
		https://github.com/rabbitmq/messaging-topology-operator/releases/download/v$(RABBITMQ_TOPOLOGY_OPERATOR_VERSION)/messaging-topology-operator-with-certmanager.yaml

KNATIVE_VERSION ?= 1.1.0

# https://github.com/knative/serving/releases
.PHONY: install-knative-serving
install-knative-serving: | $(KUBECONFIG) $(KUBECTL) ## Install Knative Serving
	$(KUBECTL) $(K_CMD) --filename \
		https://github.com/knative/serving/releases/download/knative-v$(KNATIVE_VERSION)/serving-crds.yaml
	$(KUBECTL) $(K_CMD) --filename \
		https://github.com/knative/serving/releases/download/knative-v$(KNATIVE_VERSION)/serving-core.yaml
	$(KUBECTL) wait --for=condition=available deploy/controller --timeout=60s --namespace $(SERVING_NAMESPACE)
	$(KUBECTL) wait --for=condition=available deploy/webhook --timeout=60s --namespace $(SERVING_NAMESPACE)
	$(KUBECTL) apply --filename https://github.com/knative/net-kourier/releases/download/knative-v$(KNATIVE_VERSION)/kourier.yaml
	$(KUBECTL) patch configmap/config-network --namespace $(SERVING_NAMESPACE) --type merge \
		--patch '{"data":{"ingress.class":"kourier.ingress.networking.knative.dev"}}'

# https://github.com/knative/eventing/releases
.PHONY: install-knative-eventing
install-knative-eventing: | $(KUBECONFIG) $(KUBECTL) ## Install Knative Eventing
	$(KUBECTL) $(K_CMD) --filename \
		https://github.com/knative/eventing/releases/download/knative-v$(KNATIVE_VERSION)/eventing-crds.yaml
	$(KUBECTL) $(K_CMD) --filename \
		https://github.com/knative/eventing/releases/download/knative-v$(KNATIVE_VERSION)/eventing-core.yaml
	$(KUBECTL) wait --for=condition=available deploy/eventing-controller --timeout=60s --namespace $(EVENTING_NAMESPACE)
	$(KUBECTL) wait --for=condition=available deploy/eventing-webhook --timeout=60s --namespace $(EVENTING_NAMESPACE)

.PHONY: install
install: | $(KUBECTL) $(KO) install-knative-eventing install-rabbitmq-cluster-operator install-rabbitmq-topology-operator ## Install local dev Knative Eventing RabbitMQ - manages all dependencies, including K8S components
	$(KO) apply --filename config/broker
	$(KUBECTL) wait --for=condition=available deploy/rabbitmq-broker-controller --timeout=60s --namespace $(EVENTING_NAMESPACE)
	$(KUBECTL) wait --for=condition=available deploy/rabbitmq-broker-webhook --timeout=60s --namespace $(EVENTING_NAMESPACE)
	$(KO) apply --filename config/source
	$(KUBECTL) wait --for=condition=available deploy/pingsource-mt-adapter --timeout=60s --namespace knative-eventing

.PHONY: install-standalone
install-standalone: | $(KUBECTL) $(KO) install-knative-eventing install-rabbitmq-cluster-operator ## Install local dev Knative Eventing RabbitMQ Standalone - manages all dependencies, including K8S components
	$(KO) apply --filename config/brokerstandalone
	$(KUBECTL) wait --for=condition=available deploy/rabbitmq-standalone-broker-controller --timeout=60s --namespace $(EVENTING_NAMESPACE)
	$(KUBECTL) wait --for=condition=available deploy/rabbitmq-broker-webhook --timeout=60s --namespace $(EVENTING_NAMESPACE)
	$(KO) apply --filename config/source
	$(KUBECTL) wait --for=condition=available deploy/pingsource-mt-adapter --timeout=60s --namespace knative-eventing

.PHONY: test-e2e-publish
test-e2e-publish: | $(KUBECONFIG) ## Run TestKoPublish end-to-end tests  - assumes a K8S with all necessary components installed (Knative & RabbitMQ)
	go test -v -race -count=1 -timeout=15m -tags=e2e ./test/e2e/... -run 'TestKoPublish'

.PHONY: test-e2e-broker
test-e2e-broker: | $(KUBECONFIG) ## Run Broker end-to-end tests - assumes a K8S with all necessary components installed (Knative & RabbitMQ)
	@printf "$(WARN)$(BOLD)rabbitmqcluster$(NORMAL)$(WARN) has large resource requirements 🤔$(NORMAL)\n"
	go test -v -race -count=1 -timeout=20m -tags=e2e ./test/e2e/... -run 'Test.*Broker.*'

.PHONY: test-e2e-source
test-e2e-source: | $(KUBECONFIG) ## Run Source end-to-end tests - assumes a K8S with all necessary components installed (Knative & RabbitMQ)
	go test -v -race -count=1 -timeout=15m -tags=e2e ./test/e2e/... -run 'Test.*Source.*'

.PHONY: test-e2e
test-e2e: install test-e2e-publish test-e2e-broker test-e2e-source ## Run all end-to-end tests - manages all dependencies, including K8S components

.PHONY: _test-conformance
_test-conformance:
	BROKER_TEMPLATES=$(BROKER_TEMPLATES) \
	BROKER_CLASS=RabbitMQBroker \
	go test -v -tags=e2e \
		-count=1 -parallel=12 -timeout=2h \
		-run TestBroker.*Conformance.* $(CURDIR)/test/conformance/...

.PHONY: test-conformance
test-conformance: BROKER_TEMPLATES = $(CURDIR)/test/conformance/testdata/with-operator
test-conformance: | _test-conformance ## Run conformance tests

.PHONY: test-conformance-standalone
test-conformance-standalone: BROKER_TEMPLATES = $(CURDIR)/test/conformance/testdata/with-secret
test-conformance-standalone: _test-conformance ## Run conformance tests for standalone broker

TEST_COMPILATION_TAGS=e2e
.PHONY: test-compilation
test-compilation: ## Build test binaries with e2e tags
	go test -vet=off -tags "$(BUILD_TAGS)" -exec echo  ./...

.PHONE: reset
reset:
	kind delete cluster; rm -f $(CURDIR)/.envrc; rm -rf $(CURDIR)/bin; rm -rf $(CURDIR)/.config
