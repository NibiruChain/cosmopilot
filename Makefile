NAME 	?= ghcr.io/voluzi/cosmopilot
VERSION ?= $(shell git describe --tags --exclude 'node-*/*' --exclude 'vault-*/*' --exclude 'chart/*' --exclude 'dataexporter/*' --abbrev=0)
IMG 	?= $(NAME):$(VERSION:v%=%)

NODE_UTILS_NAME    ?= ghcr.io/voluzi/node-utils
NODE_UTILS_VERSION ?= $(shell git describe --tags --match 'node-utils/*' --abbrev=0)
NODE_UTILS_IMG 	   ?= $(NODE_UTILS_NAME):$(NODE_UTILS_VERSION:node-utils/v%=%)

HELM_CHART_LATEST_TAG ?= $(shell git describe --tags --match 'chart/*' --abbrev=0)
HELM_CHART_VERSION = $(HELM_CHART_LATEST_TAG:chart/v%=%)

BUILDDIR ?= $(CURDIR)/build

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

OS_NAME := $(shell uname -s | tr A-Z a-z)
ifeq ($(shell uname -m),x86_64)
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
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
/^[a-zA-Z0-9_.-]+:.*?##/ { printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2 } \
/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: generate controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	@$(CONTROLLER_GEN) \
		rbac:roleName=manager-role crd webhook paths="./..." \
		output:crd:artifacts:config=helm/cosmopilot/crds \
		output:rbac:dir=helm/cosmopilot/templates/rbac
	@sed $(sedi) 's/name: manager-role/name: {{ .Release.Name }}/g' helm/cosmopilot/templates/rbac/role.yaml

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	@$(CONTROLLER_GEN) object paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: docs
docs: crd-to-markdown ## Generate markdown docs of CRD spec.
	@mkdir -p ./docs/03-reference/crds
	@$(CRD_TO_MARKDOWN) \
		-f ./api/v1/chainnode_types.go \
		-f ./api/v1/chainnodeset_types.go \
		-f ./api/v1/common_types.go \
		--header ./docs/03-reference/crds/header.md \
		-n ChainNode \
		-n ChainNodeSet > ./docs/03-reference/crds/crds.md
	@./contrib/scripts/generate-example-docs.sh

##@ Tests

.PHONY: test.unit
test.unit: manifests generate fmt vet ## Run unit tests.
	go test ./api/... ./cmd/... ./internal/... ./pkg/... -coverprofile cover.out

.PHONY: test.integration
test.integration: FOCUS?=
test.integration: SKIP?=
test.integration: TEST_TIMEOUT?=10m
test.integration: manifests generate fmt vet envtest ## Run integration tests (envtest-based, no cluster needed).
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
	go test -v ./test/integration/... -timeout $(TEST_TIMEOUT) \
		--ginkgo.v \
		--ginkgo.focus="$(FOCUS)" \
		--ginkgo.skip="$(SKIP)"

.PHONY: test.e2e
test.e2e: CLUSTER_NAME?=cosmopilot-e2e
test.e2e: REUSE_CLUSTER?=true
test.e2e: BUILD_NODE_UTILS?=true
test.e2e: FOCUS?=
test.e2e: SKIP?=
test.e2e: TEST_TIMEOUT?=30m
test.e2e: PROCS?=4
test.e2e: manifests generate fmt vet docker-build kind kubectl helm ginkgo ## Run e2e tests with locally built image.
	@if [ "$(BUILD_NODE_UTILS)" = "true" ]; then \
		$(MAKE) docker-build-nodeutils; \
	fi
	E2E_TEST=true \
	CLUSTER_NAME=$(CLUSTER_NAME) \
	CONTROLLER_IMAGE=$(IMG) \
	NODE_UTILS_IMAGE=$(NODE_UTILS_IMG) \
	BUILD_NODE_UTILS=$(BUILD_NODE_UTILS) \
	REUSE_CLUSTER=$(REUSE_CLUSTER) \
	$(GINKGO) -v -procs=$(PROCS) --timeout=$(TEST_TIMEOUT) \
		--focus="$(FOCUS)" \
		--skip="$(SKIP)" \
		./test/e2e/...

.PHONY: test.e2e.release
test.e2e.release: CLUSTER_NAME?=cosmopilot-e2e
test.e2e.release: CHART_VERSION?=$(HELM_CHART_VERSION)
test.e2e.release: REUSE_CLUSTER?=true
test.e2e.release: FOCUS?=
test.e2e.release: SKIP?=
test.e2e.release: TEST_TIMEOUT?=30m
test.e2e.release: PROCS?=4
test.e2e.release: kind kubectl helm ginkgo ## Run e2e tests with released chart version.
	E2E_TEST=true \
	CLUSTER_NAME=$(CLUSTER_NAME) \
	CHART_VERSION=$(CHART_VERSION) \
	NODE_UTILS_IMAGE=$(NODE_UTILS_IMG) \
	REUSE_CLUSTER=$(REUSE_CLUSTER) \
	$(GINKGO) -v -procs=$(PROCS) --timeout=$(TEST_TIMEOUT) \
		--focus="$(FOCUS)" \
		--skip="$(SKIP)" \
		./test/e2e/...


##@ Build

$(BUILDDIR)/:
	mkdir -p $(BUILDDIR)/

.PHONY: build
build: manifests generate fmt vet $(BUILDDIR)/ ## Build manager binary.
	go build -o $(BUILDDIR)/cosmopilot ./cmd/manager

.PHONY: docker-build
docker-build: ## Build docker image.
	docker build -t $(IMG) .

.PHONY: docker-build-nodeutils
docker-build-nodeutils: ## Build node-utils docker image.
	docker build -t $(NODE_UTILS_IMG) -f Dockerfile.utils .

.PHONY: helm.package
helm.package: manifests helm $(BUILDDIR)/ ## Package helm chart. Final package name is cosmopilot-<<VERSION>>.tgz
	@$(HELM) package helm/cosmopilot --version $(HELM_CHART_VERSION:v%=%) --app-version $(VERSION:v%=%) -d $(BUILDDIR)

##@ Run

.PHONY: run
run: WORKER_NAME?=
run: WORKER_COUNT?=1
run: manifests generate ## Run the controller locally on your host.
	go run ./cmd/manager --nodeutils-image="$(NODE_UTILS_IMG)" --worker-name="$(WORKER_NAME)" -worker-count=$(WORKER_COUNT) -debug-mode -disable-webhooks

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kubectl ## Install CRDs into the K8s cluster
	@$(KUBECTL) apply -f helm/cosmopilot/crds --server-side --force-conflicts

.PHONY: uninstall
uninstall: manifests kubectl ## Uninstall CRDs from the K8s cluster
	@$(KUBECTL) delete -f helm/cosmopilot/crds

.PHONY: deploy
deploy: RELEASE_NAME?=cosmopilot
deploy: NAMESPACE?=cosmopilot-system
deploy: SERVICE_MONITOR_ENABLED?=false
deploy: IMAGE_PULL_SECRETS?=
deploy: WORKER_NAME?=
deploy: WORKER_COUNT?=1
deploy: DEBUG_MODE?=false
deploy: PROBES_ENABLED?=true
deploy: APP_VERSION?=$(VERSION:v%=%)
deploy: manifests helm ## Deploy controller to the K8s cluster
	@$(HELM) package helm/cosmopilot --version $(VERSION:v%=%) --app-version $(VERSION:v%=%) -d testbin/
	@$(HELM) upgrade $(RELEASE_NAME) \
		--install \
		--create-namespace \
		--namespace=$(NAMESPACE) \
		--set image=$(NAME) \
		--set imageTag=$(APP_VERSION:v%=%) \
		--set nodeUtilsImage=$(NODE_UTILS_IMG) \
		--set probesEnabled=$(PROBES_ENABLED) \
		--set workerName=$(WORKER_NAME) \
		--set workerCount=$(WORKER_COUNT) \
		--set imagePullSecrets=$(IMAGE_PULL_SECRETS) \
		--set serviceMonitorEnabled=$(SERVICE_MONITOR_ENABLED) \
		--set debugMode=$(DEBUG_MODE) \
		--wait \
		./testbin/cosmopilot-$(VERSION:v%=%).tgz

.PHONY: undeploy
undeploy: RELEASE_NAME?=cosmopilot
undeploy: NAMESPACE?=cosmopilot-system
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
HELM ?= $(LOCALBIN)/helm
CRD_TO_MARKDOWN ?= $(LOCALBIN)/crd-to-markdown
KIND ?= $(LOCALBIN)/kind
ENVTEST ?= $(LOCALBIN)/setup-envtest
GINKGO ?= $(LOCALBIN)/ginkgo

## Tool Versions
KUBECTL_VERSION ?= v1.34.3
CONTROLLER_TOOLS_VERSION ?= v0.17.3
HELM_VERSION ?= v3.17.3
CRD_TO_MARKDOWN_VERSION ?= 0.0.3
KIND_VERSION ?= 0.25.0
ENVTEST_K8S_VERSION ?= 1.32.0
GINKGO_VERSION ?= v2.27.3

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
kind: $(KIND) ## Download kind locally if necessary. If wrong version is installed, it will be overwritten.
$(KIND): $(LOCALBIN)
	@if test -x $(KIND) && ! $(KIND) version -q | grep -q $(KIND_VERSION); then \
		echo "$(KIND) version is not expected $(KIND_VERSION). Removing it before installing."; \
		rm -rf $(KIND); \
	fi
	@test -s $(KIND) || { \
		GOBIN=$(LOCALBIN) go install sigs.k8s.io/kind@v$(KIND_VERSION) ;\
	}

# find or download setup-envtest
.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	@test -s $(ENVTEST) || { \
		GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest ;\
	}

# find or download ginkgo
.PHONY: ginkgo
ginkgo: $(GINKGO) ## Download ginkgo locally if necessary.
$(GINKGO): $(LOCALBIN)
	@test -s $(GINKGO) && $(GINKGO) version | grep -q $(GINKGO_VERSION) || \
	GOBIN=$(LOCALBIN) go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)
