NAME 	?= ghcr.io/nibiruchain/nibiru-operator
VERSION ?= $(shell git describe --tags --exclude 'node-*/*' --abbrev=0)
IMG 	?= $(NAME):$(VERSION:v%=%)

NODE_UTILS_NAME    ?= ghcr.io/nibiruchain/node-utils
NODE_UTILS_VERSION ?= $(shell git describe --tags --match 'node-utils/*' --abbrev=0)
NODE_UTILS_IMG 	   ?= $(NODE_UTILS_NAME):$(NODE_UTILS_VERSION:node-utils/v%=%)

ENVTEST_K8S_VERSION = 1.26.1

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
	@$(CRD_TO_MARKDOWN) -f ./api/v1/chainnode_types.go -n ChainNode > ./docs/api/01-chainnode.md
	@$(CRD_TO_MARKDOWN) -f ./api/v1/chainnodeset_types.go -n ChainNodeSet > ./docs/api/02-chainnodeset.md
	@$(CRD_TO_MARKDOWN) -f ./api/v1/common_types.go -n Common > ./docs/api/03-common.md

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

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/manager

.PHONY: run
run: WORKER_NAME?=
run: WORKER_COUNT?=1
run: manifests generate ## Run a controller from your host.
	go run ./cmd/manager --nodeutils-image="$(NODE_UTILS_IMG)" --worker-name="$(WORKER_NAME)" -worker-count=$(WORKER_COUNT) -debug-mode -disable-webhooks

.PHONY: mirrord
mirrord: RELEASE_NAME?=nibiru-operator
mirrord: NAMESPACE?=nibiru-system
mirrord: WORKER_NAME?=
mirrord: WORKER_COUNT?=1
mirrord: manifests generate
	mirrord exec -t deployment/$(RELEASE_NAME) \
		-n $(NAMESPACE) \
		-a $(NAMESPACE) \
		-p --steal \
		go -- run ./cmd/manager \
			-nodeutils-image="$(NODE_UTILS_IMG)" \
			-worker-count=$(WORKER_COUNT) \
			-worker-name="$(WORKER_NAME)" \
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
deploy: manifests helm ## Deploy controller to the K8s cluster
	@$(HELM) upgrade $(RELEASE_NAME) \
		--install \
		--create-namespace \
		--namespace=$(NAMESPACE) \
		--set image=$(NAME) \
		--set imageTag=$(VERSION:v%=%) \
		--set nodeUtilsImage=$(NODE_UTILS_IMG) \
		--set workerName=$(WORKER_NAME) \
		--set workerCount=$(WORKER_COUNT) \
		--set imagePullSecrets=$(IMAGE_PULL_SECRETS) \
		--set serviceMonitorEnabled=$(SERVICE_MONITOR_ENABLED) \
		--set debugMode=$(DEBUG_MODE) \
		helm/nibiru-operator

.PHONY: undeploy
undeploy: RELEASE_NAME?=nibiru-operator
undeploy: NAMESPACE?=nibiru-system
undeploy: helm ## Undeploy controller from the K8s cluster
	@$(HELM) uninstall --namespace=$(NAMESPACE) $(RELEASE_NAME)

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	@mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= $(LOCALBIN)/kubectl
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
HELM ?= $(LOCALBIN)/helm
CRD_TO_MARKDOWN ?= $(LOCALBIN)/crd-to-markdown

## Tool Versions
KUBECTL_VERSION ?= v1.27.2
CONTROLLER_TOOLS_VERSION ?= v0.11.3
HELM_VERSION ?= v3.12.0
CRD_TO_MARKDOWN_VERSION ?= 0.0.3

.PHONY: kubectl
kubectl: $(KUBECTL) ## Download kubectl locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUBECTL): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kubectl && ! $(LOCALBIN)/kubectl version --output=json | grep -q $(KUBECTL_VERSION); then \
		echo "$(LOCALBIN)/kubectl version is not expected $(KUBECTL_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kubectl; \
	fi
	@test -s $(LOCALBIN)/kubectl || { \
  		curl -sfL https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/$(OS_NAME)/$(ARCH_NAME)/kubectl -o $(LOCALBIN)/kubectl; \
  		chmod a+x $(LOCALBIN)/kubectl; \
  	}

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	@test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	@test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: helm
helm: $(HELM) ## Download helm locally if necessary. If wrong version is installed, it will be removed before downloading.
$(HELM): $(LOCALBIN)
	@if test -x $(LOCALBIN)/helm && ! $(LOCALBIN)/helm version | grep -q $(HELM_VERSION); then \
		echo "$(LOCALBIN)/helm version is not expected $(HELM_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/helm; \
	fi
	@test -s $(LOCALBIN)/helm || { \
  		curl -sfL https://get.helm.sh/helm-$(HELM_VERSION)-$(OS_NAME)-$(ARCH_NAME).tar.gz -o helm.tar.gz; \
  		tar -zxf helm.tar.gz; \
  		mv $(OS_NAME)-$(ARCH_NAME)/helm $(LOCALBIN)/helm; \
  		rm -rf $(OS_NAME)-$(ARCH_NAME) helm.tar.gz; \
  		chmod a+x $(LOCALBIN)/helm; \
  	}

.PHONY: crd-to-markdown
crd-to-markdown: $(CRD_TO_MARKDOWN) ## Download crd-to-markdown locally if necessary.
$(CRD_TO_MARKDOWN): $(LOCALBIN)
	@test -s $(LOCALBIN)/crd-to-markdown || { \
  		GOBIN=$(LOCALBIN) go install github.com/clamoriniere/crd-to-markdown@v$(CRD_TO_MARKDOWN_VERSION); \
  	}