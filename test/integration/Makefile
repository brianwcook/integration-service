# Integration Test Makefile
# This Makefile provides targets for running integration tests with Kind

# Variables
KIND_CLUSTER_NAME ?= integration-test
KUBECONFIG ?= $(HOME)/.kube/config
REGISTRY_NAME ?= kind-registry
REGISTRY_PORT ?= 5001
TIMEOUT ?= 30m

# Container engine - use podman instead of docker
CONTAINER_ENGINE := /opt/podman/bin/podman

# Auto-detect if podman is running in a VM (Mac/Windows)
# This affects how we get images to the Kind cluster
UNAME_S := $(shell uname -s)
PODMAN_IN_VM := false
ifeq ($(UNAME_S),Darwin)
	PODMAN_IN_VM := true
endif
ifeq ($(UNAME_S),MINGW64_NT)
	PODMAN_IN_VM := true
endif
ifeq ($(UNAME_S),MSYS_NT)
	PODMAN_IN_VM := true
endif

# Check if podman machine is running (another indicator of VM usage)
ifneq ($(shell $(CONTAINER_ENGINE) machine list --format "{{.Running}}" 2>/dev/null | grep -c "true"),0)
	PODMAN_IN_VM := true
endif

# Default target
.PHONY: help
help: ## Show this help message
	@echo "Integration Test Makefile"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "Environment:"
	@echo "  OS: $(UNAME_S)"
	@echo "  Podman in VM: $(PODMAN_IN_VM)"
	@echo "  Container Engine: $(CONTAINER_ENGINE)"

.PHONY: test
test: ## Run integration tests (requires existing cluster)
	@echo "Running integration tests..."
	@export KUBEBUILDER_ASSETS="$$(cd ../.. && ./bin/setup-envtest use 1.25 --print path)" && \
	KIND_PROVIDER=podman go test -v --ginkgo.v --timeout=$(TIMEOUT) .

.PHONY: test-with-kind
test-with-kind: kind-up install-crds deploy-integration-service run-tests kind-down ## Run integration tests with Kind cluster

.PHONY: kind-up
kind-up: ## Create Kind cluster
	@echo "Creating Kind cluster with Podman..."
	@echo "Podman in VM: $(PODMAN_IN_VM)"
ifeq ($(PODMAN_IN_VM),true)
	@echo "Using VM-optimized setup (no registry needed)..."
	@if ! $(CONTAINER_ENGINE) network exists kind 2>/dev/null; then \
		$(CONTAINER_ENGINE) network create kind; \
	fi
else
	@echo "Using native setup with registry..."
	@if ! $(CONTAINER_ENGINE) network exists kind 2>/dev/null; then \
		$(CONTAINER_ENGINE) network create kind; \
	fi
	@if ! $(CONTAINER_ENGINE) ps --filter "name=$(REGISTRY_NAME)" --format "{{.Names}}" | grep -q $(REGISTRY_NAME); then \
		$(CONTAINER_ENGINE) run -d --restart=always -p "127.0.0.1:$(REGISTRY_PORT):5000" --name "$(REGISTRY_NAME)" --network kind registry:2; \
	fi
endif
	@if ! kind get clusters | grep -q $(KIND_CLUSTER_NAME); then \
		KIND_EXPERIMENTAL_PROVIDER=podman kind create cluster --name $(KIND_CLUSTER_NAME) --config kind-config.yaml; \
	fi
	@kubectl config use-context kind-$(KIND_CLUSTER_NAME)
ifneq ($(PODMAN_IN_VM),true)
	@echo "Connecting registry to cluster network..."
	@$(CONTAINER_ENGINE) network connect kind $(REGISTRY_NAME) || true
	@echo "Registry setup complete"
endif

.PHONY: kind-down
kind-down: ## Delete Kind cluster and registry
	@echo "Deleting Kind cluster..."
	@KIND_EXPERIMENTAL_PROVIDER=podman kind delete cluster --name $(KIND_CLUSTER_NAME) || true
ifneq ($(PODMAN_IN_VM),true)
	@$(CONTAINER_ENGINE) stop $(REGISTRY_NAME) || true
	@$(CONTAINER_ENGINE) rm $(REGISTRY_NAME) || true
endif
	@$(CONTAINER_ENGINE) network rm kind || true

.PHONY: install-crds
install-crds: ## Install CRDs into the cluster
	@echo "Installing CRDs..."
	@kubectl apply -f ../../config/crd/bases/

.PHONY: install-tekton
install-tekton: ## Install Tekton Pipelines
	@echo "Installing Tekton Pipelines..."
	@kubectl apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml
	@kubectl wait --for=condition=ready pod -l app=tekton-pipelines-controller -n tekton-pipelines --timeout=300s

.PHONY: deploy-integration-service
deploy-integration-service: ## Deploy integration service controllers to the cluster
	@echo "Building integration service image..."
	@cd ../.. && $(CONTAINER_ENGINE) build -t integration-service:test .
ifeq ($(PODMAN_IN_VM),true)
	@echo "Using save/load method for VM environment..."
	@$(CONTAINER_ENGINE) save -o /tmp/integration-service-test.tar integration-service:test
	@KIND_EXPERIMENTAL_PROVIDER=podman kind load image-archive /tmp/integration-service-test.tar --name $(KIND_CLUSTER_NAME)
	@rm -f /tmp/integration-service-test.tar
	@echo "Deploying integration service..."
	@cd ../.. && make deploy IMG=integration-service:test
else
	@echo "Using registry push method for native environment..."
	@$(CONTAINER_ENGINE) tag integration-service:test localhost:$(REGISTRY_PORT)/integration-service:test
	@$(CONTAINER_ENGINE) push localhost:$(REGISTRY_PORT)/integration-service:test
	@echo "Deploying integration service..."
	@cd ../.. && make deploy IMG=localhost:$(REGISTRY_PORT)/integration-service:test
endif
	@kubectl wait --for=condition=available deployment/integration-service-controller-manager -n integration-service-system --timeout=300s

.PHONY: run-tests
run-tests: ## Run the integration tests
	@echo "Running integration tests..."
	@export KUBEBUILDER_ASSETS="$$(cd ../.. && ./bin/setup-envtest use 1.25 --print path)" && \
	go test -v --ginkgo.v --timeout=$(TIMEOUT) .

.PHONY: setup-test-env
setup-test-env: kind-up install-tekton install-crds deploy-integration-service ## Setup complete test environment

.PHONY: clean
clean: kind-down ## Clean up all resources

.PHONY: logs
logs: ## Show logs from integration service controllers
	@echo "Showing controller logs..."
	@kubectl logs -l app=integration-service -n integration-service-system --tail=100

.PHONY: debug
debug: ## Debug integration test environment
	@echo "Cluster info:"
	@kubectl cluster-info
	@echo ""
	@echo "Nodes:"
	@kubectl get nodes
	@echo ""
	@echo "Pods:"
	@kubectl get pods --all-namespaces
	@echo ""
	@echo "CRDs:"
	@kubectl get crd | grep integration 

.PHONY: build-and-load-image
build-and-load-image: ## Build and load integration service image (VM-optimized)
	@echo "Building integration service image..."
	@cd ../.. && $(CONTAINER_ENGINE) build -t integration-service:test .
	@echo "Loading image into Kind cluster..."
	@$(CONTAINER_ENGINE) save -o /tmp/integration-service-test.tar integration-service:test
	@KIND_EXPERIMENTAL_PROVIDER=podman kind load image-archive /tmp/integration-service-test.tar --name $(KIND_CLUSTER_NAME)
	@rm -f /tmp/integration-service-test.tar
	@echo "Image loaded successfully"

.PHONY: deploy-only
deploy-only: ## Deploy integration service (assumes image is already loaded)
	@echo "Deploying integration service..."
	@cd ../.. && make deploy IMG=integration-service:test
	@kubectl wait --for=condition=available deployment/integration-service-controller-manager -n integration-service-system --timeout=300s 