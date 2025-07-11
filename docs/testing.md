# Testing Guide

This document explains how to run tests for the integration-service project, including unit tests for controllers and webhooks.

## Quick Start

**For daily development (recommended):**
```bash
make test-unit          # Fast local unit tests
make test-integration   # Local integration tests with Kind
make test-all          # All tests locally
```

**For CI/CD consistency:**
```bash
make test-unit-containerized    # Unit tests in container
make test-all-containerized     # All tests in container
```

## Test Structure

The project contains several types of tests:

### Unit Tests (Fast ⚡)
- **Controller Tests**: Test the business logic of controllers using fake Kubernetes clients
  - `internal/controller/testsubject/` - TestSubject controller tests
  - `internal/controller/testsubjectconstructor/` - TestSubjectConstructor controller tests
- **Webhook Tests**: Test validation webhook logic
  - `internal/webhook/v1alpha1/` - TestSubject webhook validation tests

### Integration Tests (Comprehensive 🔧)
- **End-to-End Tests**: Test complete workflows using Kind clusters
  - `test/integration/` - Full integration test suite

## Running Tests

### Local Testing (Recommended for Development)

**Install dependencies once:**
```bash
make install-test-tools
```

**Run tests:**
```bash
# Unit tests only (fastest for development)
make test-unit

# Integration tests (requires Kind)
make test-integration

# All tests
make test-all
```

**Prerequisites:**
- Go 1.23+
- Kind (for integration tests)
- Docker or Podman (for integration tests)

### Containerized Testing (For Consistency)

**Build test container:**
```bash
make test-container-build
```

**Run tests in container:**
```bash
# Unit tests in container
make test-unit-containerized

# All tests in container
make test-all-containerized
```

**Debug container environment:**
```bash
make test-container-debug
```

## When to Use Each Approach

| Scenario | Recommended Approach | Why |
|----------|---------------------|-----|
| 💻 **Daily development** | `make test-unit` | Fastest feedback loop |
| 🔧 **Pre-commit validation** | `make test-all` | Comprehensive local testing |
| 🏗️ **CI/CD pipelines** | `make test-unit-containerized` | Consistent environment |
| 🐛 **Debugging test failures** | Local approach | Better IDE integration |
| 🌍 **Cross-platform consistency** | Container approach | Eliminates "works on my machine" |

## Test Development

### Writing Unit Tests

Unit tests use the [Ginkgo](https://onsi.github.io/ginkgo/) testing framework with [Gomega](https://onsi.github.io/gomega/) matchers.

**Example test structure:**
```go
var _ = Describe("MyController", func() {
    BeforeEach(func() {
        // Setup test environment
    })
    
    Context("when processing valid resources", func() {
        It("should create expected resources", func() {
            // Test implementation
        })
    })
})
```

### Running Specific Tests

```bash
# Run specific test package
ginkgo run -v internal/controller/testsubject/

# Run tests with focus
ginkgo run -v --focus="TestSubject Controller" internal/controller/testsubject/

# Run tests with custom flags
ginkgo run -v --dry-run internal/controller/testsubject/
```

## Troubleshooting

### Common Issues

**"ginkgo: command not found"**
```bash
make install-test-tools
```

**Integration tests fail to start Kind**
```bash
# Check Kind installation
kind version

# Check Docker/Podman
docker version  # or podman version
```

**Container tests fail to build**
```bash
# Check container engine
make test-container-debug

# Clean and rebuild
make test-container-clean
make test-container-build
```

### Performance Tips

- Use `make test-unit` for rapid development iteration
- Use `make test-integration` only when testing full workflows
- Use containers for final validation before PR submission

## CI/CD Integration

For CI/CD pipelines, we recommend:

1. **Local approach** for most cases (faster)
2. **Container approach** for release validation
3. **Parallel execution** where possible

**Example CI configuration:**
```yaml
jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      - run: make test-unit
      
  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      - run: make test-integration
``` 