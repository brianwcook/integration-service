# Integration Test Suite

This directory contains a comprehensive integration test suite for the integration-service, designed to replace the old e2e tests with a modern, reliable testing framework using Kind.

## Overview

The integration test suite provides:

- **Real Kubernetes Environment**: Uses Kind to create actual Kubernetes clusters for testing
- **Complete Workflow Testing**: Tests the entire build-to-test workflow end-to-end
- **Controller Integration**: Tests all controllers working together
- **Scenario Coverage**: Covers various integration scenarios including error cases
- **Fast Execution**: Optimized for quick feedback in CI/CD pipelines

## Architecture

The test suite consists of:

- **`integration_test_suite.go`**: Main test suite setup with envtest framework
- **`end_to_end_test.go`**: Comprehensive end-to-end test scenarios
- **`Makefile`**: Automation for running tests with Kind
- **`kind-config.yaml`**: Kind cluster configuration
- **Helper functions**: Utilities for creating test resources

## Test Scenarios

### 1. Complete Build-to-Test Workflow
Tests the full pipeline from build completion to integration test execution:
- TestSubjectConstructor watches for successful builds
- Automatically creates TestSubject with extracted component data
- Finds matching IntegrationTestScenarios
- Creates and executes integration test PipelineRuns

### 2. TestSubject Group Management
Tests the control TestSubject pattern:
- First component build creates initial TestSubject
- Second component build creates new TestSubject with both components
- Proper label management and component aggregation

### 3. Label Selector Matching
Tests the flexibility of label-based selection:
- Multiple applications with different labels
- Scenarios only run for matching TestSubjects
- No cross-contamination between applications

### 4. Optional Test Handling
Tests required vs optional integration tests:
- Both required and optional scenarios are executed
- Proper labeling of optional tests
- Correct failure handling for optional tests

### 5. Error Handling
Tests robustness and error recovery:
- Invalid JQ expressions in TestSubjectConstructor
- Missing or malformed resources
- Controller resilience to failures

## Running Tests

### Prerequisites

- Docker
- Kind
- kubectl
- Go 1.21+
- Ginkgo v2

### Quick Start

```bash
# Run tests with automatic Kind cluster management
make test-with-kind

# Or run individual steps
make kind-up
make install-crds
make install-tekton
make run-tests
make kind-down
```

### Development Workflow

```bash
# Setup test environment once
make setup-test-env

# Run tests multiple times during development
make test

# Debug issues
make debug
make logs

# Clean up when done
make clean
```

### Environment Variables

- `KIND_CLUSTER_NAME`: Name of the Kind cluster (default: integration-test)
- `REGISTRY_PORT`: Port for the local registry (default: 5001)
- `TIMEOUT`: Test timeout (default: 30m)

## Test Structure

### Suite Setup
The test suite uses the envtest framework to:
1. Start a real Kubernetes API server
2. Install all required CRDs
3. Start the integration-service controllers
4. Provide a clean environment for each test

### Test Helpers
Common helper functions provide:
- Namespace management
- Resource creation (TestSubject, IntegrationTestScenario, etc.)
- Waiting for resources to be created/updated
- Assertions for complex scenarios

### Test Isolation
Each test runs in its own namespace to ensure:
- No interference between tests
- Clean state for each scenario
- Parallel test execution capability

## CI/CD Integration

The test suite is designed for CI/CD pipelines:

```bash
# In CI pipeline
make test-with-kind
```

This command:
1. Creates a Kind cluster
2. Installs dependencies
3. Runs all tests
4. Cleans up resources
5. Reports results

## Extending Tests

### Adding New Test Scenarios

1. Create test functions in `end_to_end_test.go`
2. Use existing helper functions for resource creation
3. Follow the pattern of setup → action → verification
4. Use descriptive test names and `By()` statements

### Adding New Helper Functions

1. Add helpers to `integration_test_suite.go`
2. Follow naming convention: `CreateXXX`, `WaitForXXX`, `VerifyXXX`
3. Use Eventually/Consistently for async operations
4. Include proper error handling

### Testing New Controllers

1. Update `SetupControllers()` to include new controllers
2. Add required CRDs to the test environment
3. Create helper functions for new resource types
4. Write focused tests for new functionality

## Troubleshooting

### Common Issues

1. **Kind cluster creation fails**
   - Check Docker is running
   - Verify Kind is installed
   - Check port availability

2. **Tests timeout**
   - Increase timeout with `TIMEOUT=60m`
   - Check resource quotas
   - Verify controller logs

3. **CRD installation fails**
   - Ensure CRDs are up to date
   - Check RBAC permissions
   - Verify cluster connectivity

### Debug Commands

```bash
# Check cluster status
make debug

# View controller logs
make logs

# Manual cluster inspection
kubectl get pods --all-namespaces
kubectl describe crd integrationtestscenarios.integration.konflux-ci.dev
```

## Performance Considerations

The test suite is optimized for:
- Fast startup (< 2 minutes)
- Efficient resource usage
- Parallel test execution
- Quick cleanup

Typical execution times:
- Full suite: 10-15 minutes
- Individual test: 30-60 seconds
- Setup/teardown: 2-3 minutes

## Migration from Old E2E Tests

This new integration test suite replaces the old e2e tests with:
- **Better Reliability**: Real Kubernetes environment vs mocked
- **Faster Execution**: Optimized setup and parallel execution
- **Better Coverage**: More comprehensive scenarios
- **Easier Maintenance**: Clear structure and helper functions
- **CI/CD Friendly**: Designed for automation

The old e2e tests can be safely removed once this suite is fully validated. 