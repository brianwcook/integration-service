# Integration Test Makefile
# This Makefile provides targets for running integration tests with Kind

# Variables
KIND_CLUSTER_NAME ?= integration-test
KUBECONFIG ?= $(HOME)/.kube/config
REGISTRY_NAME ?= kind-registry
REGISTRY_PORT ?= 5001
TIMEOUT ?= 30m

# Default target
.PHONY: help
help: ## Show this help message
	@echo "Integration Test Makefile"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: test
test: ## Run integration tests (requires existing cluster)
	@echo "Running integration tests..."
	ginkgo -v --timeout=$(TIMEOUT) .

.PHONY: test-with-kind
test-with-kind: kind-up install-crds run-tests kind-down ## Run integration tests with Kind cluster

.PHONY: kind-up
kind-up: ## Create Kind cluster with registry
	@echo "Creating Kind cluster..."
	@if ! docker network ls | grep -q kind; then \
		docker network create kind; \
	fi
	@if ! docker ps | grep -q $(REGISTRY_NAME); then \
		docker run -d --restart=always -p "127.0.0.1:$(REGISTRY_PORT):5000" --name "$(REGISTRY_NAME)" --net kind registry:2; \
	fi
	@if ! kind get clusters | grep -q $(KIND_CLUSTER_NAME); then \
		kind create cluster --name $(KIND_CLUSTER_NAME) --config kind-config.yaml; \
	fi
	@kubectl config use-context kind-$(KIND_CLUSTER_NAME)
	@echo "Connecting registry to cluster network..."
	@if ! docker network ls | grep -q kind; then \
		docker network create kind; \
	fi
	@docker network connect kind $(REGISTRY_NAME) || true
	@kubectl apply -f - <<EOF || true
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:$(REGISTRY_PORT)"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

.PHONY: kind-down
kind-down: ## Delete Kind cluster and registry
	@echo "Deleting Kind cluster..."
	@kind delete cluster --name $(KIND_CLUSTER_NAME) || true
	@docker stop $(REGISTRY_NAME) || true
	@docker rm $(REGISTRY_NAME) || true

.PHONY: install-crds
install-crds: ## Install CRDs into the cluster
	@echo "Installing CRDs..."
	@kubectl apply -f ../../config/crd/bases/

.PHONY: install-tekton
install-tekton: ## Install Tekton Pipelines
	@echo "Installing Tekton Pipelines..."
	@kubectl apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml
	@kubectl wait --for=condition=ready pod -l app=tekton-pipelines-controller -n tekton-pipelines --timeout=300s

.PHONY: run-tests
run-tests: ## Run the integration tests
	@echo "Running integration tests..."
	@ginkgo -v --timeout=$(TIMEOUT) .

.PHONY: setup-test-env
setup-test-env: kind-up install-tekton install-crds ## Setup complete test environment

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