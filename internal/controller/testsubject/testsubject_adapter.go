/*
Copyright 2023 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testsubject

import (
	"context"
	"fmt"
	"strings"

	integrationv1alpha1 "github.com/konflux-ci/integration-service/api/integration/v1alpha1"
	"github.com/konflux-ci/integration-service/helpers"
	"github.com/konflux-ci/operator-toolkit/controller"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Adapter holds the objects needed to reconcile a TestSubject.
type Adapter struct {
	testSubject *integrationv1alpha1.TestSubject
	logger      helpers.IntegrationLogger
	client      client.Client
	context     context.Context
}

// NewAdapter creates and returns an Adapter instance.
func NewAdapter(context context.Context, testSubject *integrationv1alpha1.TestSubject, logger helpers.IntegrationLogger, client client.Client) *Adapter {
	return &Adapter{
		testSubject: testSubject,
		logger:      logger,
		client:      client,
		context:     context,
	}
}

// EnsureIntegrationTestPipelineRunsExist ensures that PipelineRuns exist for all matching IntegrationTestScenarios
func (a *Adapter) EnsureIntegrationTestPipelineRunsExist() (controller.OperationResult, error) {
	// Get all IntegrationTestScenarios that match this TestSubject
	scenarios, err := a.getMatchingIntegrationTestScenarios()
	if err != nil {
		return controller.RequeueWithError(err)
	}

	if len(scenarios) == 0 {
		a.logger.Info("No matching IntegrationTestScenarios found for TestSubject")
		return controller.ContinueProcessing()
	}

	// Create PipelineRuns for each matching scenario
	for _, scenario := range scenarios {
		pipelineRun, err := a.createPipelineRunForScenario(&scenario)
		if err != nil {
			a.logger.Error(err, "Failed to create PipelineRun for scenario", "scenario", scenario.Name)
			return controller.RequeueWithError(err)
		}
		if pipelineRun != nil {
			a.logger.Info("Created PipelineRun for IntegrationTestScenario",
				"pipelineRun", pipelineRun.Name, "scenario", scenario.Name)
		}
	}

	return controller.ContinueProcessing()
}

// EnsureTestResultsUpdated ensures that test results are updated in the TestSubject status
func (a *Adapter) EnsureTestResultsUpdated() (controller.OperationResult, error) {
	// Get all PipelineRuns for this TestSubject
	pipelineRuns, err := a.getTestPipelineRuns()
	if err != nil {
		return controller.RequeueWithError(err)
	}

	// Update test results based on PipelineRun statuses
	testResults := make([]integrationv1alpha1.TestSubjectTestResult, 0)

	for _, pipelineRun := range pipelineRuns {
		scenario, exists := pipelineRun.Labels[helpers.PipelineRunScenarioLabel]
		if !exists {
			continue
		}

		result := integrationv1alpha1.TestSubjectTestResult{
			Scenario: scenario,
			Status:   a.getPipelineRunStatus(&pipelineRun),
			Summary:  a.getPipelineRunSummary(&pipelineRun),
			LogURL:   a.getPipelineRunLogURL(&pipelineRun),
		}

		if pipelineRun.Status.StartTime != nil {
			result.StartTime = pipelineRun.Status.StartTime
		}
		if pipelineRun.Status.CompletionTime != nil {
			result.CompletionTime = pipelineRun.Status.CompletionTime
		}

		testResults = append(testResults, result)
	}

	// Update TestSubject status with test results
	if len(testResults) > 0 {
		a.testSubject.Status.TestResults = testResults
		if err := a.client.Status().Update(a.context, a.testSubject); err != nil {
			a.logger.Error(err, "Failed to update TestSubject status with test results")
			return controller.RequeueWithError(err)
		}
		a.logger.Info("Updated TestSubject status with test results", "resultCount", len(testResults))
	}

	return controller.ContinueProcessing()
}

// EnsureControlTestSubjectUpdated ensures that this TestSubject becomes the control if all required tests pass
func (a *Adapter) EnsureControlTestSubjectUpdated() (controller.OperationResult, error) {
	// Check if all required tests have passed
	allRequiredTestsPassed, err := a.allRequiredTestsPassed()
	if err != nil {
		return controller.RequeueWithError(err)
	}

	if !allRequiredTestsPassed {
		a.logger.Info("Not all required tests have passed, not updating control TestSubject")
		return controller.ContinueProcessing()
	}

	// Get the current group label
	groupLabel, exists := a.testSubject.Labels[integrationv1alpha1.TestSubjectGroupLabel]
	if !exists {
		a.logger.Info("TestSubject missing group label, skipping control update")
		return controller.ContinueProcessing()
	}

	// Remove control label from any existing control TestSubject in the same group
	if err := a.removeControlLabelFromOtherTestSubjects(groupLabel); err != nil {
		a.logger.Error(err, "Failed to remove control label from other TestSubjects")
		return controller.RequeueWithError(err)
	}

	// Add control label to this TestSubject
	if a.testSubject.Labels == nil {
		a.testSubject.Labels = make(map[string]string)
	}
	a.testSubject.Labels[integrationv1alpha1.ControlTestSubjectLabel] = "true"

	if err := a.client.Update(a.context, a.testSubject); err != nil {
		a.logger.Error(err, "Failed to update TestSubject with control label")
		return controller.RequeueWithError(err)
	}

	a.logger.Info("Updated TestSubject to be the control TestSubject", "group", groupLabel)
	return controller.ContinueProcessing()
}

// getMatchingIntegrationTestScenarios returns all IntegrationTestScenarios that match this TestSubject
func (a *Adapter) getMatchingIntegrationTestScenarios() ([]integrationv1alpha1.IntegrationTestScenario, error) {
	scenarios := &integrationv1alpha1.IntegrationTestScenarioList{}
	if err := a.client.List(a.context, scenarios, &client.ListOptions{
		Namespace: a.testSubject.Namespace,
	}); err != nil {
		return nil, err
	}

	var matchingScenarios []integrationv1alpha1.IntegrationTestScenario
	for _, scenario := range scenarios.Items {
		if a.testSubjectMatchesScenario(&scenario) {
			matchingScenarios = append(matchingScenarios, scenario)
		}
	}

	return matchingScenarios, nil
}

// testSubjectMatchesScenario checks if a TestSubject matches a given IntegrationTestScenario
func (a *Adapter) testSubjectMatchesScenario(scenario *integrationv1alpha1.IntegrationTestScenario) bool {
	selector := &metav1.LabelSelector{
		MatchLabels:      scenario.Spec.Selector.MatchLabels,
		MatchExpressions: scenario.Spec.Selector.MatchExpressions,
	}

	labelSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		a.logger.Error(err, "Failed to convert label selector", "scenario", scenario.Name)
		return false
	}

	return labelSelector.Matches(labels.Set(a.testSubject.Labels))
}

// createPipelineRunForScenario creates a PipelineRun for a given IntegrationTestScenario
func (a *Adapter) createPipelineRunForScenario(scenario *integrationv1alpha1.IntegrationTestScenario) (*tektonv1.PipelineRun, error) {
	// Check if PipelineRun already exists
	pipelineRunName := strings.ToLower(a.testSubject.Name + "-" + scenario.Name)
	existingPipelineRun := &tektonv1.PipelineRun{}
	err := a.client.Get(a.context, types.NamespacedName{
		Name:      pipelineRunName,
		Namespace: a.testSubject.Namespace,
	}, existingPipelineRun)
	if err == nil {
		// PipelineRun already exists
		return nil, nil
	}
	if !errors.IsNotFound(err) {
		return nil, err
	}

	// Create PipelineRun for this scenario
	pipelineRun := &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipelineRunName,
			Namespace: a.testSubject.Namespace,
			Labels: map[string]string{
				helpers.PipelineRunTestSubjectLabel: a.testSubject.Name,
				helpers.PipelineRunScenarioLabel:    scenario.Name,
			},
		},
		Spec: tektonv1.PipelineRunSpec{
			PipelineRef: &tektonv1.PipelineRef{
				ResolverRef: tektonv1.ResolverRef{
					Resolver: tektonv1.ResolverName(scenario.Spec.ResolverRef.Resolver),
					Params:   a.convertResolverParams(scenario.Spec.ResolverRef.Params),
				},
			},
			Params: a.buildPipelineParams(scenario),
		},
	}

	if err := a.client.Create(a.context, pipelineRun); err != nil {
		return nil, err
	}

	return pipelineRun, nil
}

// getTestPipelineRuns returns all PipelineRuns for this TestSubject
func (a *Adapter) getTestPipelineRuns() ([]tektonv1.PipelineRun, error) {
	pipelineRuns := &tektonv1.PipelineRunList{}
	if err := a.client.List(a.context, pipelineRuns, &client.ListOptions{
		Namespace: a.testSubject.Namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			helpers.PipelineRunTestSubjectLabel: a.testSubject.Name,
		}),
	}); err != nil {
		return nil, err
	}

	return pipelineRuns.Items, nil
}

// getPipelineRunStatus returns the status of a PipelineRun
func (a *Adapter) getPipelineRunStatus(pipelineRun *tektonv1.PipelineRun) string {
	if pipelineRun.Status.CompletionTime != nil {
		if helpers.HasPipelineRunSucceeded(pipelineRun) {
			return "Passed"
		}
		return "Failed"
	}
	if pipelineRun.Status.StartTime != nil {
		return "InProgress"
	}
	return "Pending"
}

// getPipelineRunSummary returns a summary of the PipelineRun
func (a *Adapter) getPipelineRunSummary(pipelineRun *tektonv1.PipelineRun) string {
	if pipelineRun.Status.CompletionTime != nil {
		if helpers.HasPipelineRunSucceeded(pipelineRun) {
			return "Integration test passed successfully"
		}
		// Get the failure reason from conditions
		if condition := pipelineRun.Status.GetCondition(apis.ConditionSucceeded); condition != nil {
			return fmt.Sprintf("Integration test failed: %s", condition.Message)
		}
		return "Integration test failed"
	}
	return "Integration test in progress"
}

// getPipelineRunLogURL returns the log URL for a PipelineRun
func (a *Adapter) getPipelineRunLogURL(pipelineRun *tektonv1.PipelineRun) string {
	// This would need to be implemented based on the logging infrastructure
	return fmt.Sprintf("/logs/pipelinerun/%s/%s", pipelineRun.Namespace, pipelineRun.Name)
}

// allRequiredTestsPassed checks if all required tests have passed
func (a *Adapter) allRequiredTestsPassed() (bool, error) {
	scenarios, err := a.getMatchingIntegrationTestScenarios()
	if err != nil {
		return false, err
	}

	for _, scenario := range scenarios {
		if scenario.Spec.Optional {
			continue // Skip optional scenarios
		}

		// Check if there's a corresponding test result
		testPassed := false
		for _, result := range a.testSubject.Status.TestResults {
			if result.Scenario == scenario.Name && result.Status == "Passed" {
				testPassed = true
				break
			}
		}

		if !testPassed {
			return false, nil
		}
	}

	return true, nil
}

// removeControlLabelFromOtherTestSubjects removes the control label from other TestSubjects in the same group
func (a *Adapter) removeControlLabelFromOtherTestSubjects(groupLabel string) error {
	testSubjects := &integrationv1alpha1.TestSubjectList{}
	if err := a.client.List(a.context, testSubjects, &client.ListOptions{
		Namespace: a.testSubject.Namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			integrationv1alpha1.TestSubjectGroupLabel:   groupLabel,
			integrationv1alpha1.ControlTestSubjectLabel: "true",
		}),
	}); err != nil {
		return err
	}

	for _, testSubject := range testSubjects.Items {
		if testSubject.Name == a.testSubject.Name {
			continue // Skip self
		}

		// Remove control label
		if testSubject.Labels != nil {
			delete(testSubject.Labels, integrationv1alpha1.ControlTestSubjectLabel)
			if err := a.client.Update(a.context, &testSubject); err != nil {
				return err
			}
			a.logger.Info("Removed control label from TestSubject", "testSubject", testSubject.Name)
		}
	}

	return nil
}

// convertResolverParams converts resolver parameters to Tekton format
func (a *Adapter) convertResolverParams(params []integrationv1alpha1.ResolverParameter) tektonv1.Params {
	tektonParams := make(tektonv1.Params, len(params))
	for i, param := range params {
		tektonParams[i] = tektonv1.Param{
			Name:  param.Name,
			Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: param.Value},
		}
	}
	return tektonParams
}

// buildPipelineParams builds the parameters for the PipelineRun
func (a *Adapter) buildPipelineParams(scenario *integrationv1alpha1.IntegrationTestScenario) tektonv1.Params {
	params := make(tektonv1.Params, 0)

	// Add parameters from the scenario
	for _, param := range scenario.Spec.Params {
		tektonParam := tektonv1.Param{
			Name: param.Name,
		}

		if param.Value != "" {
			tektonParam.Value = tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: param.Value}
		} else if len(param.Values) > 0 {
			tektonParam.Value = tektonv1.ParamValue{Type: tektonv1.ParamTypeArray, ArrayVal: param.Values}
		}

		params = append(params, tektonParam)
	}

	// Add TestSubject-specific parameters
	params = append(params, tektonv1.Param{
		Name:  "TEST_SUBJECT_NAME",
		Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: a.testSubject.Name},
	})

	return params
}
