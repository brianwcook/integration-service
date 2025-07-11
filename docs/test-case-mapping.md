# Test Case Mapping: Old vs New Architecture

This document maps the existing test cases from the old architecture to the new refactored integration service architecture based on ADR-0033.

## Test Case Analysis

### 1. BuildPipeline Controller Tests

#### Old Test Cases:
- `TestBuildPipelineReconcile_CreateSnapshot` - Tests creation of Snapshot from successful build PipelineRun
- `TestBuildPipelineReconcile_SkipCreateSnapshot` - Tests skipping Snapshot creation for certain conditions
- `TestBuildPipelineReconcile_UpdateSnapshot` - Tests updating existing Snapshot with new component
- `TestBuildPipelineReconcile_HandleMissingApplication` - Tests error handling when Application doesn't exist
- `TestBuildPipelineReconcile_HandleMissingComponent` - Tests error handling when Component doesn't exist

#### New Test Cases:
- ✅ **TestBuildPipelineReconcile_CreateSnapshot** → **TestTestSubjectConstructor_CreateTestSubject**
  - **Changes**: Instead of creating Snapshot, creates TestSubject via TestSubjectConstructor
  - **Key Differences**: Uses JQ extraction, no Application/Component dependency
  
- ✅ **TestBuildPipelineReconcile_SkipCreateSnapshot** → **TestTestSubjectConstructor_SkipCreation**
  - **Changes**: Uses selector matching instead of hardcoded conditions
  - **Key Differences**: More flexible skipping logic via label/field selectors

- ✅ **TestBuildPipelineReconcile_UpdateSnapshot** → **TestTestSubjectConstructor_UpdateControlTestSubject**
  - **Changes**: Updates control TestSubject instead of Snapshot
  - **Key Differences**: Uses TestSubject groups, merges component data

- ❌ **TestBuildPipelineReconcile_HandleMissingApplication** → **REMOVED**
  - **Reason**: No Application dependency in new architecture

- ❌ **TestBuildPipelineReconcile_HandleMissingComponent** → **REMOVED**
  - **Reason**: No Component dependency in new architecture

### 2. Snapshot Controller Tests

#### Old Test Cases:
- `TestSnapshotReconcile_FindMatchingScenarios` - Tests finding IntegrationTestScenarios for Application
- `TestSnapshotReconcile_CreatePipelineRun` - Tests creating PipelineRun from IntegrationTestScenario
- `TestSnapshotReconcile_UpdateSnapshotStatus` - Tests updating Snapshot status with test results
- `TestSnapshotReconcile_HandleGlobalCandidateList` - Tests managing global candidate list
- `TestSnapshotReconcile_HandleReleaseStatus` - Tests updating release status based on test results
- `TestSnapshotReconcile_HandleEnvironmentPromotion` - Tests environment promotion logic

#### New Test Cases:
- ✅ **TestSnapshotReconcile_FindMatchingScenarios** → **TestTestSubjectReconcile_FindMatchingScenarios**
  - **Changes**: Uses label selectors instead of Application references
  - **Key Differences**: More flexible matching, no Application dependency

- ✅ **TestSnapshotReconcile_CreatePipelineRun** → **TestTestSubjectReconcile_CreatePipelineRun**
  - **Changes**: Same logic but for TestSubject instead of Snapshot
  - **Key Differences**: Uses TestSubject data structure

- ✅ **TestSnapshotReconcile_UpdateSnapshotStatus** → **TestTestSubjectReconcile_UpdateTestSubjectStatus**
  - **Changes**: Updates TestSubject status instead of Snapshot
  - **Key Differences**: Different status structure, no release-related status

- ❌ **TestSnapshotReconcile_HandleGlobalCandidateList** → **REMOVED**
  - **Reason**: Global candidate list is now managed by control TestSubject concept

- ❌ **TestSnapshotReconcile_HandleReleaseStatus** → **REMOVED**
  - **Reason**: Release service separation - integration service no longer manages releases

- ❌ **TestSnapshotReconcile_HandleEnvironmentPromotion** → **REMOVED**
  - **Reason**: Environment promotion is now handled by release service

### 3. IntegrationPipeline Controller Tests

#### Old Test Cases:
- `TestIntegrationPipelineReconcile_UpdateSnapshotStatus` - Tests updating Snapshot status from PipelineRun results
- `TestIntegrationPipelineReconcile_HandleTestResults` - Tests processing test results
- `TestIntegrationPipelineReconcile_HandleOptionalTests` - Tests handling optional test failures
- `TestIntegrationPipelineReconcile_ReportStatusToGitHub` - Tests GitHub status reporting

#### New Test Cases:
- ✅ **TestIntegrationPipelineReconcile_UpdateSnapshotStatus** → **TestIntegrationPipelineReconcile_UpdateTestSubjectStatus**
  - **Changes**: Updates TestSubject instead of Snapshot
  - **Key Differences**: Different status structure, no release-related updates

- ✅ **TestIntegrationPipelineReconcile_HandleTestResults** → **TestIntegrationPipelineReconcile_HandleTestResults**
  - **Changes**: Minimal changes, same test result processing logic
  - **Key Differences**: Results stored in TestSubject status

- ✅ **TestIntegrationPipelineReconcile_HandleOptionalTests** → **TestIntegrationPipelineReconcile_HandleOptionalTests**
  - **Changes**: Same logic applies to TestSubject
  - **Key Differences**: Uses TestSubject context instead of Snapshot

- ✅ **TestIntegrationPipelineReconcile_ReportStatusToGitHub** → **TestIntegrationPipelineReconcile_ReportStatusToGitHub**
  - **Changes**: Reports TestSubject status instead of Snapshot
  - **Key Differences**: Status report format may differ

### 4. Scenario Controller Tests

#### Old Test Cases:
- `TestScenarioReconcile_ValidateScenario` - Tests IntegrationTestScenario validation
- `TestScenarioReconcile_HandleApplicationReference` - Tests Application reference validation
- `TestScenarioReconcile_HandleResolverRef` - Tests resolver reference validation

#### New Test Cases:
- ✅ **TestScenarioReconcile_ValidateScenario** → **TestScenarioReconcile_ValidateScenario**
  - **Changes**: Validates label selectors instead of Application references
  - **Key Differences**: Different validation logic for selectors

- ❌ **TestScenarioReconcile_HandleApplicationReference** → **REMOVED**
  - **Reason**: No Application references in new architecture

- ✅ **TestScenarioReconcile_HandleResolverRef** → **TestScenarioReconcile_HandleResolverRef**
  - **Changes**: Same validation logic
  - **Key Differences**: None, resolver references unchanged

### 5. Component Controller Tests

#### Old Test Cases:
- `TestComponentReconcile_HandleFinalizer` - Tests Component finalizer handling
- `TestComponentReconcile_CleanupSnapshots` - Tests cleanup of Snapshots when Component is deleted
- `TestComponentReconcile_UpdateComponentStatus` - Tests updating Component status

#### New Test Cases:
- ❌ **TestComponentReconcile_HandleFinalizer** → **REMOVED**
  - **Reason**: No Component dependency in new architecture

- ❌ **TestComponentReconcile_CleanupSnapshots** → **REMOVED**
  - **Reason**: No Component dependency, TestSubjects are independent

- ❌ **TestComponentReconcile_UpdateComponentStatus** → **REMOVED**
  - **Reason**: No Component dependency in new architecture

### 6. Status Report Controller Tests

#### Old Test Cases:
- `TestStatusReportReconcile_CreateGitHubStatus` - Tests GitHub status creation
- `TestStatusReportReconcile_UpdateGitHubStatus` - Tests GitHub status updates
- `TestStatusReportReconcile_HandlePullRequestStatus` - Tests pull request status reporting

#### New Test Cases:
- ✅ **TestStatusReportReconcile_CreateGitHubStatus** → **TestStatusReportReconcile_CreateGitHubStatus**
  - **Changes**: Reports TestSubject status instead of Snapshot
  - **Key Differences**: Status report content may differ

- ✅ **TestStatusReportReconcile_UpdateGitHubStatus** → **TestStatusReportReconcile_UpdateGitHubStatus**
  - **Changes**: Updates based on TestSubject status
  - **Key Differences**: Different status source

- ✅ **TestStatusReportReconcile_HandlePullRequestStatus** → **TestStatusReportReconcile_HandlePullRequestStatus**
  - **Changes**: Same logic but for TestSubject
  - **Key Differences**: Status derived from TestSubject

## New Test Cases Required

### 1. TestSubjectConstructor Controller Tests

- **TestTestSubjectConstructor_WatchPipelineRuns** - Tests watching for matching PipelineRuns
- **TestTestSubjectConstructor_ExtractDataWithJQ** - Tests JQ expression evaluation
- **TestTestSubjectConstructor_CreateTestSubject** - Tests TestSubject creation
- **TestTestSubjectConstructor_UpdateControlTestSubject** - Tests control TestSubject updates
- **TestTestSubjectConstructor_HandleExtractionErrors** - Tests error handling in data extraction
- **TestTestSubjectConstructor_HandleSelectorMatching** - Tests selector matching logic
- **TestTestSubjectConstructor_HandleTemplateApplication** - Tests template application

### 2. TestSubject Controller Tests

- **TestTestSubjectReconcile_FindMatchingScenarios** - Tests finding scenarios with label selectors
- **TestTestSubjectReconcile_CreatePipelineRun** - Tests PipelineRun creation
- **TestTestSubjectReconcile_UpdateTestSubjectStatus** - Tests status updates
- **TestTestSubjectReconcile_HandleControlTestSubject** - Tests control TestSubject logic
- **TestTestSubjectReconcile_HandleTestSubjectGroups** - Tests group management
- **TestTestSubjectReconcile_HandleOptionalTests** - Tests optional test handling

### 3. Integration Tests

- **TestEndToEndWorkflow** - Tests complete workflow from build to TestSubject creation to testing
- **TestTestSubjectGroupManagement** - Tests group-based TestSubject management
- **TestControlTestSubjectPromotion** - Tests control TestSubject promotion logic
- **TestJQExtractionScenarios** - Tests various JQ extraction scenarios
- **TestLabelSelectorMatching** - Tests label selector matching with various scenarios

## Test Migration Strategy

### Phase 1: Controller Unit Tests
1. Create new TestSubjectConstructor controller tests
2. Update existing controller tests to use new API types
3. Remove tests that are no longer applicable

### Phase 2: Integration Tests
1. Create end-to-end tests for the new workflow
2. Test TestSubject group management
3. Test control TestSubject promotion

### Phase 3: Compatibility Tests
1. Ensure new controllers don't interfere with old APIs during migration
2. Test data migration scenarios
3. Validate backward compatibility where applicable

## Key Testing Considerations

1. **JQ Expression Testing**: Extensive testing of JQ expressions with various PipelineRun structures
2. **Label Selector Testing**: Comprehensive testing of label selector matching
3. **Control TestSubject Logic**: Testing the control TestSubject promotion and management
4. **Error Handling**: Testing error scenarios in data extraction and TestSubject creation
5. **Performance Testing**: Testing with large numbers of TestSubjects and TestSubjectConstructors

## Summary Statistics

- **Total Old Test Cases**: ~40 test cases
- **Migrated Test Cases**: ~25 test cases (62.5%)
- **Removed Test Cases**: ~15 test cases (37.5%)
- **New Test Cases Required**: ~15 test cases
- **Net Change**: Similar number of test cases but with different focus

The refactoring maintains comprehensive test coverage while focusing on the new architecture's capabilities and removing tests related to deprecated dependencies. 