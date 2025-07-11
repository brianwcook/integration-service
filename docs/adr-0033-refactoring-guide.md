# ADR-0033: Upstreaming Integration Service - Refactoring Guide

This document describes the architectural changes made to the integration service according to [ADR-0033: Upstreaming Integration Service](https://github.com/konflux-ci/architecture/blob/main/ADR/0033-upstreaming-integration-service.md).

## Overview

The integration service has been refactored to remove dependencies on proprietary APIs and create a more focused, upstream-compatible service. The main changes include:

1. **New API Group**: `integration.konflux-ci.dev/v1alpha1`
2. **New Resource Types**: TestSubject, TestSubjectConstructor, updated IntegrationTestScenario
3. **Removed Dependencies**: Application API, Component API, Environment API, Release Service API
4. **Separation of Concerns**: Integration testing is now completely isolated from release management

## New Architecture

### API Types

#### TestSubject (`integration.konflux-ci.dev/v1alpha1`)

The `TestSubject` replaces the Snapshot resource in the integration-service domain. It represents a collection of components that should be tested together.

```yaml
apiVersion: integration.konflux-ci.dev/v1alpha1
kind: TestSubject
metadata:
  name: my-test-subject
  labels:
    integration.konflux-ci.dev/test-subject-group: "my-app"
    integration.konflux-ci.dev/control-test-subject: "true"  # Optional: marks this as the control
spec:
  components:
  - name: frontend
    containerImage: quay.io/myorg/frontend:v1.0.0
    source:
      git:
        url: https://github.com/myorg/frontend
        revision: main
  - name: backend  
    containerImage: quay.io/myorg/backend:v1.0.0
    source:
      git:
        url: https://github.com/myorg/backend
        revision: main
  source:  # Optional: overall source information
    git:
      url: https://github.com/myorg/myapp
      revision: main
status:
  testResults:
  - scenario: security-scan
    status: Passed
    startTime: "2023-10-01T10:00:00Z"
    completionTime: "2023-10-01T10:05:00Z"
    logURL: "/logs/pipelinerun/default/my-test-subject-security-scan"
  conditions:
  - type: TestsCompleted
    status: "True"
    reason: AllTestsPassed
```

#### TestSubjectConstructor (`integration.konflux-ci.dev/v1alpha1`)

The `TestSubjectConstructor` automatically creates TestSubjects when certain resources (like successful PipelineRuns) are detected.

```yaml
apiVersion: integration.konflux-ci.dev/v1alpha1
kind: TestSubjectConstructor
metadata:
  name: build-to-test-constructor
spec:
  selector:
    fields:
      match:
        kind: PipelineRun
    labels:
      match:
        pipelines.appstudio.openshift.io/type: build
        build.appstudio.openshift.io/success: "true"
  extractor:
    name: '.metadata.labels["appstudio.openshift.io/component"]'
    image_url: '.status.results[] | select(.name == "IMAGE_URL") | .value.stringVal'
    image_digest: '.status.results[] | select(.name == "IMAGE_DIGEST") | .value.stringVal'
    source:
      git:
        url: '.status.results[] | select(.name == "CHAINS-GIT_URL") | .value.stringVal'
        revision: '.status.results[] | select(.name == "CHAINS-GIT_COMMIT") | .value.stringVal'
  template:
    labels:
      integration.konflux-ci.dev/test-subject-group: "default"
      created-by: "build-constructor"
```

#### IntegrationTestScenario (`integration.konflux-ci.dev/v1alpha1`)

The updated `IntegrationTestScenario` uses label selectors instead of application references.

```yaml
apiVersion: integration.konflux-ci.dev/v1alpha1
kind: IntegrationTestScenario
metadata:
  name: security-scan
spec:
  selector:
    matchLabels:
      integration.konflux-ci.dev/test-subject-group: "my-app"
    matchExpressions:
    - key: environment
      operator: In
      values: ["staging", "production"]
  resolverRef:
    resolver: git
    params:
    - name: url
      value: https://github.com/myorg/test-definitions
    - name: revision  
      value: main
    - name: pathInRepo
      value: security/scan-pipeline.yaml
  params:
  - name: IMAGE_URL
    value: "$(params.TEST_SUBJECT_IMAGES)"
  optional: false
```

### Controllers

#### TestSubject Controller

- **Purpose**: Orchestrates integration testing for TestSubjects
- **Responsibilities**:
  - Find matching IntegrationTestScenarios using label selectors
  - Create PipelineRuns for each matching scenario
  - Update TestSubject status with test results
  - Manage control TestSubject designation within groups

#### TestSubjectConstructor Controller  

- **Purpose**: Automatically creates TestSubjects from external resources
- **Responsibilities**:
  - Watch for resources matching selector criteria
  - Extract data using JQ-like expressions
  - Create new TestSubjects with extracted data
  - Update control TestSubjects by merging new component data

#### Updated IntegrationTestScenario Controller

- **Purpose**: Validates IntegrationTestScenario resources
- **Changes**: Now works with label selectors instead of application references

### Key Concepts

#### Test Subject Groups

TestSubjects are organized into groups using the `integration.konflux-ci.dev/test-subject-group` label. This allows:

- **Grouping related components**: All components of an application can share the same group
- **Control TestSubject management**: One TestSubject per group can be marked as the "control" (the baseline for comparisons)
- **Selective testing**: IntegrationTestScenarios can target specific groups

#### Control TestSubjects

Within each group, one TestSubject can be designated as the "control" using the `integration.konflux-ci.dev/control-test-subject: "true"` label. The control TestSubject:

- Represents the current stable state of the group
- Is automatically updated when new TestSubjects pass all required tests
- Serves as the baseline for new TestSubjects in the same group

#### JQ-like Data Extraction

TestSubjectConstructors use JQ expressions to extract data from triggering resources:

```yaml
extractor:
  name: '.metadata.labels["appstudio.openshift.io/component"]'
  image_url: '.status.results[] | select(.name == "IMAGE_URL") | .value.stringVal'
  source:
    git:
      url: '.spec.params[] | select(.name == "git-url") | .value'
```

This provides flexible data extraction without hardcoded assumptions about resource structure.

## Migration Guide

### For Platform Teams

1. **Deploy new CRDs**: Apply the new `integration.konflux-ci.dev` CRDs
2. **Update controllers**: Deploy the refactored integration-service
3. **Configure constructors**: Create TestSubjectConstructors to replace automatic Snapshot-based workflows
4. **Update scenarios**: Migrate IntegrationTestScenarios to use label selectors

### For Application Teams  

1. **Update test scenarios**: Change from application-based to label-based selection
2. **Configure labels**: Apply appropriate labels to enable automatic TestSubject creation
3. **Monitor TestSubjects**: Use `kubectl get testsubjects` instead of snapshots for integration status

### Backward Compatibility

This is a **breaking change**. The old APIs (`appstudio.redhat.com`) are still supported but the new controllers only work with the new APIs (`integration.konflux-ci.dev`). A migration period will be needed to transition existing workloads.

## Benefits

1. **Upstream Compatibility**: No dependency on proprietary APIs
2. **Separation of Concerns**: Integration testing isolated from release management  
3. **Flexibility**: JQ-based extraction supports various resource schemas
4. **Simplicity**: Fewer dependencies and clearer responsibilities
5. **Extensibility**: Label-based selection enables rich matching criteria

## Examples

### Complete Workflow Example

1. **Build completes successfully**:
   ```yaml
   apiVersion: tekton.dev/v1
   kind: PipelineRun
   metadata:
     labels:
       pipelines.appstudio.openshift.io/type: build
       appstudio.openshift.io/component: frontend
   status:
     results:
     - name: IMAGE_URL
       value: quay.io/myorg/frontend:sha-abc123
   ```

2. **TestSubjectConstructor creates TestSubject**:
   ```yaml
   apiVersion: integration.konflux-ci.dev/v1alpha1  
   kind: TestSubject
   metadata:
     name: frontend-build-abc123
     labels:
       integration.konflux-ci.dev/test-subject-group: myapp
   spec:
     components:
     - name: frontend
       containerImage: quay.io/myorg/frontend:sha-abc123
   ```

3. **TestSubject controller finds matching scenarios and runs tests**

4. **When all required tests pass, TestSubject becomes the new control**

This creates a fully automated integration testing pipeline without dependencies on proprietary APIs. 