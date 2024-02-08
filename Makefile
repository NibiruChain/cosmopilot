NAME 	?= ghcr.io/nibiruchain/nibiru-operator
VERSION ?= $(shell git describe --tags --exclude 'node-*/*' --exclude 'vault-*/*' --abbrev=0)
IMG 	?= $(NAME):$(VERSION:v%=%)

NODE_UTILS_NAME    ?= ghcr.io/nibiruchain/node-utils
NODE_UTILS_VERSION ?= $(shell git describe --tags --match 'node-utils/*' --abbrev=0)
NODE_UTILS_IMG 	   ?= $(NODE_UTILS_NAME):$(NODE_UTILS_VERSION:node-utils/v%=%)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

OS_NAME := $(shell uname -s | tr A-Z a-z)
ifeq ($(shell uname -p),x86_64)
	ARCH_NAME := amd64
else
	ARCH_NAME := arm64
endif

# Default case for Linux sed, just use "-i"
sedi := -i
ifeq ($(OS_NAME),darwin)
	sedi := -i ""
endif

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: docs
docs: crd-to-markdown
	@mkdir -p ./docs/api
	@$(CRD_TO_MARKDOWN) -f ./api/v1/chainnode_types.go -f ./api/v1/common_types.go -n ChainNode > ./docs/api/01-chainnode.md
	@$(CRD_TO_MARKDOWN) -f ./api/v1/chainnodeset_types.go -f ./api/v1/common_types.go -n ChainNodeSet > ./docs/api/02-chainnodeset.md

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	@$(CONTROLLER_GEN) \
		rbac:roleName=manager-role crd webhook paths="./..." \
		output:crd:artifacts:config=helm/nibiru-operator/crds \
		output:rbac:dir=helm/nibiru-operator/templates/rbac
	@sed $(sedi) 's/name: manager-role/name: {{ .Release.Name }}/g' helm/nibiru-operator/templates/rbac/role.yaml

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	@$(CONTROLLER_GEN) object paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test.unit
test.unit: manifests generate fmt vet ## Run unit tests.
	go test ./api/... ./cmd/... ./internal/... ./pkg/... -coverprofile cover.out

.PHONY: test.e2e
test.e2e: EVENTUALLY_TIMEOUT?=5m
test.e2e: CERTS_DIR?=/tmp/no-e2e
test.e2e: FOCUS?=
test.e2e: SKIP?=
test.e2e: WORKER_COUNT?=1
test.e2e: TEST_TIMEOUT?=20m
test.e2e: manifests generate fmt vet mirrord setup-test-env install ## Run integration tests.
	@# Create dummy operator service just to have all resources. mirrod will steal its traffic then.
	@$(HELM) get metadata nibiru-operator -n nibiru-system || $(MAKE) NAME=nginxinc/nginx-unprivileged VERSION=latest PROBES_ENABLED=false deploy
	@mkdir -p $(CERTS_DIR)
	@for f in tls.key ca.crt tls.crt; do \
		$(KUBECTL) -n nibiru-system get secret nibiru-operator-cert -o=go-template='{{index .data "'$$f'"|base64decode}}' > $(CERTS_DIR)/$$f; \
	done
	@$(MIRRORD) exec -t deployment/nibiru-operator \
		-n nibiru-system \
		-a nibiru-system \
		-p --steal \
		--fs-mode local \
		--no-telemetry \
		--disable-version-check \
		go -- test -v ./test/... -timeout $(TEST_TIMEOUT) \
			--certs-dir=$(CERTS_DIR) \
			--nodeutils-image=$(NODE_UTILS_IMG) \
			--eventually-timeout=$(EVENTUALLY_TIMEOUT) \
			--cert-issuer-name=no-e2e \
			--worker-count=$(WORKER_COUNT) \
			--ginkgo.focus=$(FOCUS) \
			--ginkgo.skip=$(SKIP)


##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/manager

.PHONY: run
run: WORKER_NAME?=
run: WORKER_COUNT?=1
run: manifests generate ## Run a controller from your host.
	go run ./cmd/manager --nodeutils-image="$(NODE_UTILS_IMG)" --worker-name="$(WORKER_NAME)" -worker-count=$(WORKER_COUNT) -debug-mode -disable-webhooks

.PHONY: run.mirrord
run.mirrord: RELEASE_NAME?=nibiru-operator
run.mirrord: NAMESPACE?=nibiru-system
run.mirrord: WORKER_NAME?=
run.mirrord: WORKER_COUNT?=1
run.mirrord: CERTS_DIR?=/tmp
run.mirrord: manifests generate mirrord
	@# Create dummy operator service just to have all resources. mirrod will steal its traffic then.
	@$(HELM) get metadata $(RELEASE_NAME) -n $(NAMESPACE) || \
		$(MAKE) RELEASE_NAME=$(RELEASE_NAME) NAMESPACE=$(NAMESPACE) NAME=nginxinc/nginx-unprivileged VERSION=latest PROBES_ENABLED=false deploy
	@mkdir -p $(CERTS_DIR)
	@for f in tls.key ca.crt tls.crt; do \
		$(KUBECTL) -n $(NAMESPACE) get secret $(RELEASE_NAME)-cert -o=go-template='{{index .data "'$$f'"|base64decode}}' > $(CERTS_DIR)/$$f; \
	done
	$(MIRRORD) exec -t deployment/$(RELEASE_NAME) \
		-n $(NAMESPACE) \
		-a $(NAMESPACE) \
		-p --steal \
		--no-telemetry \
		go -- run ./cmd/manager \
			-nodeutils-image="$(NODE_UTILS_IMG)" \
			-worker-count=$(WORKER_COUNT) \
			-worker-name="$(WORKER_NAME)" \
			-certs-dir="$(CERTS_DIR)" \
			-debug-mode

.PHONY: attach.mirrord
attach.mirrord: RELEASE_NAME?=nibiru-operator
attach.mirrord: NAMESPACE?=nibiru-system
attach.mirrord: WORKER_NAME?=
attach.mirrord: WORKER_COUNT?=1
attach.mirrord: CERTS_DIR?=/tmp
attach.mirrord: manifests generate mirrord
	@mkdir -p $(CERTS_DIR)
	@for f in tls.key ca.crt tls.crt; do \
		$(KUBECTL) -n $(NAMESPACE) get secret $(RELEASE_NAME)-cert -o=go-template='{{index .data "'$$f'"|base64decode}}' > $(CERTS_DIR)/$$f; \
	done
	$(MIRRORD) exec -t deployment/$(RELEASE_NAME) \
		-n $(NAMESPACE) \
		-a $(NAMESPACE) \
		-p --steal \
		--no-telemetry \
		go -- run ./cmd/manager \
			-nodeutils-image="$(NODE_UTILS_IMG)" \
			-worker-count=$(WORKER_COUNT) \
			-worker-name="$(WORKER_NAME)" \
			-certs-dir="$(CERTS_DIR)" \
			-debug-mode

.PHONY: docker-build
docker-build: test ## Build docker image with the manager.
	docker build --platform linux/amd64 -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}

.PHONY: docker-build-utils
docker-build-utils: test ## Build docker image with the manager.
	docker build --platform linux/amd64 -f Dockerfile.utils -t ${NODE_UTILS_IMG} .

.PHONY: docker-push-utils
docker-push-utils: ## Push docker image with the manager.
	docker push ${NODE_UTILS_IMG}

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kubectl ## Install CRDs into the K8s cluster
	@$(KUBECTL) apply -f helm/nibiru-operator/crds

.PHONY: uninstall
uninstall: manifests kubectl ## Uninstall CRDs from the K8s cluster
	@$(KUBECTL) delete -f helm/nibiru-operator/crds

.PHONY: deploy
deploy: RELEASE_NAME?=nibiru-operator
deploy: NAMESPACE?=nibiru-system
deploy: SERVICE_MONITOR_ENABLED?=false
deploy: IMAGE_PULL_SECRETS?=
deploy: WORKER_NAME?=
deploy: WORKER_COUNT?=1
deploy: DEBUG_MODE?=false
deploy: PROBES_ENABLED?=true
deploy: manifests helm ## Deploy controller to the K8s cluster
	@$(HELM) upgrade $(RELEASE_NAME) \
		--install \
		--create-namespace \
		--namespace=$(NAMESPACE) \
		--set image=$(NAME) \
		--set imageTag=$(VERSION:v%=%) \
		--set nodeUtilsImage=$(NODE_UTILS_IMG) \
		--set probesEnabled=$(PROBES_ENABLED) \
		--set workerName=$(WORKER_NAME) \
		--set workerCount=$(WORKER_COUNT) \
		--set imagePullSecrets=$(IMAGE_PULL_SECRETS) \
		--set serviceMonitorEnabled=$(SERVICE_MONITOR_ENABLED) \
		--set debugMode=$(DEBUG_MODE) \
		--wait \
		helm/nibiru-operator

.PHONY: undeploy
undeploy: RELEASE_NAME?=nibiru-operator
undeploy: NAMESPACE?=nibiru-system
undeploy: helm ## Undeploy controller from the K8s cluster
	@$(HELM) uninstall --namespace=$(NAMESPACE) $(RELEASE_NAME)

##@ Test Environment

.PHONY: setup-test-env
setup-test-env: CLUSTER_NAME?=no-e2e
setup-test-env: kind kubectl helm
	@./contrib/scripts/test-env/env.sh up --cluster-name $(CLUSTER_NAME) --issuer-name no-e2e --kind-bin $(KIND) --kubectl-bin $(KUBECTL) --helm-bin $(HELM)
	@sleep 5 #Wait for cert-manager to be available to respond to webhook requests

.PHONY: teardown-test-env
teardown-test-env: CLUSTER_NAME?=no-e2e
teardown-test-env: kind
	@./contrib/scripts/test-env/env.sh down --cluster-name $(CLUSTER_NAME) --kind-bin $(KIND)

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	@mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= $(LOCALBIN)/kubectl
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
HELM ?= $(LOCALBIN)/helm
CRD_TO_MARKDOWN ?= $(LOCALBIN)/crd-to-markdown
KIND ?= $(LOCALBIN)/kind
MIRRORD ?= $(LOCALBIN)/mirrord

## Tool Versions
KUBECTL_VERSION ?= v1.29.1
CONTROLLER_TOOLS_VERSION ?= v0.11.3
HELM_VERSION ?= v3.14.0
CRD_TO_MARKDOWN_VERSION ?= 0.0.3
KIND_VERSION ?= 0.21.0
MIRRORD_VERSION ?= 3.86.1

.PHONY: kubectl
kubectl: $(KUBECTL) ## Download kubectl locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUBECTL): $(LOCALBIN)
	@if test -x $(KUBECTL) && ! $(KUBECTL) version --output=json | grep -q $(KUBECTL_VERSION); then \
		echo "$(KUBECTL) version is not expected $(KUBECTL_VERSION). Removing it before installing."; \
		rm -rf $(KUBECTL); \
	fi
	@test -s $(KUBECTL) || { \
		curl -sfL https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/$(OS_NAME)/$(ARCH_NAME)/kubectl -o $(KUBECTL); \
        chmod a+x $(KUBECTL); \
  	}

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	@test -s $(CONTROLLER_GEN) && $(CONTROLLER_GEN) --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: helm
helm: $(HELM) ## Download helm locally if necessary. If wrong version is installed, it will be removed before downloading.
$(HELM): $(LOCALBIN)
	@if test -x $(HELM) && ! $(HELM) version | grep -q $(HELM_VERSION); then \
		echo "$(HELM) version is not expected $(HELM_VERSION). Removing it before installing."; \
		rm -rf $(HELM); \
	fi
	@test -s $(HELM) || { \
  		curl -sfL https://get.helm.sh/helm-$(HELM_VERSION)-$(OS_NAME)-$(ARCH_NAME).tar.gz -o helm.tar.gz; \
  		tar -zxf helm.tar.gz; \
        mv $(OS_NAME)-$(ARCH_NAME)/helm $(HELM); \
  		rm -rf $(OS_NAME)-$(ARCH_NAME) helm.tar.gz; \
        chmod a+x $(HELM); \
  	}

.PHONY: crd-to-markdown
crd-to-markdown: $(CRD_TO_MARKDOWN) ## Download crd-to-markdown locally if necessary.
$(CRD_TO_MARKDOWN): $(LOCALBIN)
	@test -s $(CRD_TO_MARKDOWN) || { \
  		GOBIN=$(LOCALBIN) go install github.com/clamoriniere/crd-to-markdown@v$(CRD_TO_MARKDOWN_VERSION); \
    }

# find or download kind
.PHONY: kind
kind: $(KIND)
$(KIND): $(LOCALBIN)
	@if test -x $(KIND) && ! $(KIND) version -q | grep -q $(KIND_VERSION); then \
		echo "$(KIND) version is not expected $(KIND_VERSION). Removing it before installing."; \
		rm -rf $(KIND); \
	fi
	@test -s $(KIND) || { \
		GOBIN=$(LOCALBIN) go install sigs.k8s.io/kind@v$(KIND_VERSION) ;\
	}

# find or download mirrord
.PHONY: mirrord
mirrord: $(MIRRORD)
$(MIRRORD): $(LOCALBIN)
	@if test -x $(MIRRORD) && ! $(MIRRORD) -V | grep -q $(MIRRORD_VERSION); then \
		echo "$(MIRRORD) version is not expected $(MIRRORD_VERSION). Removing it before installing."; \
		rm -rf $(MIRRORD); \
	fi
	@test -s $(MIRRORD) || { \
		if [ "$(OS_NAME)" = "darwin" ]; then \
			curl -sfL https://github.com/metalbear-co/mirrord/releases/download/$(MIRRORD_VERSION)/mirrord_mac_universal -o $(MIRRORD); \
		else \
			if [ "$(ARCH_NAME)" = "amd64" ]; then \
			  curl -sfL https://github.com/metalbear-co/mirrord/releases/download/$(MIRRORD_VERSION)/mirrord_linux_x86_64 -o $(MIRRORD); \
			else \
			  curl -sfL https://github.com/metalbear-co/mirrord/releases/download/$(MIRRORD_VERSION)/mirrord_linux_aarch64 -o $(MIRRORD); \
			fi; \
		fi; \
		chmod a+x $(MIRRORD); \
	}
