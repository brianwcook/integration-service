# Integration Tests - Kind-based Real Kubernetes Testing

This directory contains **real** integration tests for the Integration Service that run against a live Kubernetes cluster using Kind and Podman.

## Overview

These tests replace the previous envtest-based approach with real Kubernetes clusters to provide more accurate integration testing. The tests validate:

- **ADR-0033 TestSubjectConstructor workflow** - Complete build-to-test automation
- **Multi-component TestSubject management** - Cumulative component builds
- **TestSubject validation webhooks** - API validation and security
- **Error handling and resilience** - Graceful degradation
- **Controller health monitoring** - Service availability checks

## Prerequisites

- **Podman Desktop** (Do NOT install Docker - uses Podman as container runtime)
- **Kind** (Kubernetes in Docker/Podman)
- **kubectl** (Kubernetes CLI)
- **Go 1.21+** (for running tests)

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────────┐
│  Kind Cluster   │    │  Test Namespace  │    │  Integration Tests  │
│                 │    │                  │    │                     │
│  ┌───────────┐  │    │  ┌─────────────┐ │    │  ┌───────────────┐  │
│  │ Tekton    │  │    │  │ TestSubject │ │    │  │ Ginkgo Specs  │  │
│  │ Pipelines │  │    │  │ Constructor │ │    │  │               │  │
│  └───────────┘  │    │  └─────────────┘ │    │  └───────────────┘  │
│                 │    │                  │    │                     │
│  ┌───────────┐  │    │  ┌─────────────┐ │    │  ┌───────────────┐  │
│  │Integration│  │    │  │ Integration │ │    │  │ Test Helpers  │  │
│  │Service    │  │    │  │ Test        │ │    │  │               │  │
│  │Controllers│  │    │  │ Scenarios   │ │    │  └───────────────┘  │
│  └───────────┘  │    │  └─────────────┘ │    │                     │
└─────────────────┘    └──────────────────┘    └─────────────────────┘
```

## Running Tests

### Quick Start

```bash
# Run complete test suite with Kind cluster
make test-with-kind

# Run tests against existing cluster
make test
```

### Manual Setup

```bash
# 1. Create Kind cluster with registry
make kind-up

# 2. Install dependencies (Tekton, CRDs)
make install-tekton
make install-crds

# 3. Deploy integration service
make deploy-integration-service

# 4. Run tests
make run-tests

# 5. Clean up
make kind-down
```

### Available Make Targets

| Target | Description |
|--------|-------------|
| `test-with-kind` | Full test suite with cluster setup/teardown |
| `test` | Run tests against existing cluster |
| `kind-up` | Create Kind cluster with registry |
| `kind-down` | Delete Kind cluster and registry |
| `install-tekton` | Install Tekton Pipelines |
| `install-crds` | Install Integration Service CRDs |
| `deploy-integration-service` | Deploy controllers |
| `setup-test-env` | Setup complete test environment |
| `logs` | Show controller logs |
| `debug` | Show cluster debug information |
| `help` | Show all available targets |

## Test Structure

### Core Test Scenarios

#### 1. ADR-0033 TestSubjectConstructor Workflow
Tests the complete build-to-test automation:
- Creates TestSubjectConstructor to watch build PipelineRuns
- Simulates successful component builds
- Verifies TestSubject creation with correct component metadata
- Validates integration test PipelineRun creation and parameters

#### 2. Multi-Component TestSubject Management
Tests cumulative component builds:
- Creates multiple component builds over time
- Verifies TestSubjects are updated with new components
- Validates component metadata aggregation

#### 3. TestSubject Validation Webhook
Tests API validation:
- Attempts to create invalid TestSubjects
- Verifies webhook rejection of malformed resources
- Confirms valid TestSubjects are accepted

#### 4. Error Handling and Resilience
Tests graceful degradation:
- Creates TestSubjectConstructors with invalid JQ expressions
- Verifies no TestSubjects are created from invalid configurations
- Tests controller recovery from errors

#### 5. Integration Service Controller Health
Tests service availability:
- Verifies all controllers are running and healthy
- Confirms Tekton is operational
- Validates cluster connectivity

### Test Helper Functions

The test suite includes comprehensive helper functions:

```go
// Namespace management
CreateTestNamespace() string
DeleteTestNamespace(name string)

// TestSubject operations
WaitForTestSubject(namespace, name string)
WaitForTestSubjectWithLabel(namespace, labelKey, labelValue string)

// PipelineRun operations
WaitForPipelineRun(namespace, labelKey, labelValue string)
CreateSuccessfulBuildPipelineRun(namespace, componentName, imageURL string)

// Resource creation
CreateIntegrationTestScenario(namespace, name, groupLabel string, optional bool)
CreateTestSubjectConstructor(namespace, name, groupLabel string)
```

## Container Engine Configuration

The tests use **Podman** as the container engine instead of Docker:

```makefile
# Uses Podman path
CONTAINER_ENGINE := /opt/podman/bin/podman

# Kind uses Podman provider
KIND_EXPERIMENTAL_PROVIDER=podman kind create cluster
```

## Debugging

### View Controller Logs
```bash
make logs
```

### Debug Cluster State
```bash
make debug
```

### Manual Inspection
```bash
# Connect to cluster
kubectl config use-context kind-integration-test

# View resources
kubectl get testsubjects -A
kubectl get integrationtestscenarios -A
kubectl get pipelineruns -A

# View controller status
kubectl get pods -n integration-service-system
kubectl get pods -n tekton-pipelines
```

## Configuration

### Kind Cluster Configuration
See `kind-config.yaml` for cluster setup including:
- Container registry configuration
- Networking settings
- Feature gates
- Port mappings

### Test Environment Variables
- `KUBECONFIG`: Path to kubeconfig file
- `KIND_PROVIDER`: Set to `podman` for Podman runtime
- `TIMEOUT`: Test timeout (default: 30m)

## Differences from envtest

| Aspect | envtest (Old) | Kind (New) |
|--------|---------------|------------|
| **Environment** | Simulated API server | Real Kubernetes cluster |
| **Controllers** | In-process | Deployed in cluster |
| **Networking** | Mocked | Real cluster networking |
| **Webhooks** | Local webhook server | Real admission webhooks |
| **Tekton** | Not available | Full Tekton Pipelines |
| **Realism** | Limited | Production-like |
| **Performance** | Fast | Slower but more accurate |
| **Debugging** | Difficult | Standard kubectl debugging |

## Contributing

When adding new tests:

1. **Use test helpers** for common operations
2. **Create isolated namespaces** for each test
3. **Wait for resources** rather than sleep
4. **Clean up resources** in AfterEach blocks
5. **Test real workflows** end-to-end
6. **Verify both success and failure** cases

## Troubleshooting

### Common Issues

**Kind cluster creation fails:**
```bash
# Ensure Podman is running
podman system info

# Check Kind provider
KIND_EXPERIMENTAL_PROVIDER=podman kind get clusters
```

**Tests timeout:**
```bash
# Check controller logs
make logs

# Verify cluster health
make debug
```

**Registry issues:**
```bash
# Check registry connectivity
podman ps | grep registry
```

**Resource not found:**
```bash
# Verify CRDs are installed
kubectl get crd | grep integration
```

For more help, see the main project documentation or open an issue. 