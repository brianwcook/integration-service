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

package testsubjectconstructor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/itchyny/gojq"
	integrationv1alpha1 "github.com/konflux-ci/integration-service/api/integration/v1alpha1"
	"github.com/konflux-ci/integration-service/helpers"
	"github.com/konflux-ci/operator-toolkit/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Adapter holds the objects needed to reconcile a TestSubjectConstructor.
type Adapter struct {
	constructor *integrationv1alpha1.TestSubjectConstructor
	logger      helpers.IntegrationLogger
	client      client.Client
	context     context.Context
}

// NewAdapter creates and returns an Adapter instance.
func NewAdapter(context context.Context, constructor *integrationv1alpha1.TestSubjectConstructor, logger helpers.IntegrationLogger, client client.Client) *Adapter {
	return &Adapter{
		constructor: constructor,
		logger:      logger,
		client:      client,
		context:     context,
	}
}

// EnsureTestSubjectConstructed ensures that TestSubjects are constructed based on the constructor configuration
func (a *Adapter) EnsureTestSubjectConstructed() (controller.OperationResult, error) {
	// This operation is primarily reactive - it responds to resource changes
	// The actual construction happens in ProcessTriggeringResource
	a.logger.Info("TestSubjectConstructor is monitoring for triggering resources")
	return controller.ContinueProcessing()
}

// EnsureStatusUpdated ensures that the status of the TestSubjectConstructor is updated
func (a *Adapter) EnsureStatusUpdated() (controller.OperationResult, error) {
	// Update the status with current state
	if a.constructor.Status.LastProcessedResource == nil {
		a.constructor.Status.LastProcessedResource = &integrationv1alpha1.TestSubjectProcessedResource{}
	}

	// Update conditions to indicate the constructor is ready
	readyCondition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "ConstructorReady",
		Message:            "TestSubjectConstructor is ready to process resources",
	}

	// Update or add the condition
	conditionExists := false
	for i, condition := range a.constructor.Status.Conditions {
		if condition.Type == "Ready" {
			a.constructor.Status.Conditions[i] = readyCondition
			conditionExists = true
			break
		}
	}
	if !conditionExists {
		a.constructor.Status.Conditions = append(a.constructor.Status.Conditions, readyCondition)
	}

	if err := a.client.Status().Update(a.context, a.constructor); err != nil {
		a.logger.Error(err, "Failed to update TestSubjectConstructor status")
		return controller.RequeueWithError(err)
	}

	return controller.ContinueProcessing()
}

// ProcessTriggeringResource processes a resource that matches the constructor's selector
func (a *Adapter) ProcessTriggeringResource(resource client.Object) error {
	a.logger.Info("Processing triggering resource", "resource", resource.GetName())

	// Extract data from the resource using the configured extractors
	extractedData, err := a.extractDataFromResource(resource)
	if err != nil {
		a.logger.Error(err, "Failed to extract data from resource")
		return err
	}

	// Get the control TestSubject for this group (if any)
	controlTestSubject, err := a.getControlTestSubject(extractedData.GroupLabel)
	if err != nil {
		a.logger.Error(err, "Failed to get control TestSubject")
		return err
	}

	// Create a new TestSubject
	newTestSubject, err := a.createTestSubject(extractedData, controlTestSubject)
	if err != nil {
		a.logger.Error(err, "Failed to create TestSubject")
		return err
	}

	// Update the constructor status with the processed resource
	if err := a.updateProcessedResourceStatus(resource, newTestSubject); err != nil {
		a.logger.Error(err, "Failed to update processed resource status")
		return err
	}

	a.logger.Info("Successfully processed triggering resource",
		"resource", resource.GetName(),
		"createdTestSubject", newTestSubject.Name)

	return nil
}

// ExtractedData holds the data extracted from a resource
type ExtractedData struct {
	ImageURL      string
	ImageDigest   string
	ComponentName string
	GroupLabel    string
	GitSource     *integrationv1alpha1.TestSubjectGitSource
}

// extractDataFromResource extracts data from a resource using JQ expressions
func (a *Adapter) extractDataFromResource(resource client.Object) (*ExtractedData, error) {
	// Convert resource to JSON for JQ processing
	resourceJSON, err := json.Marshal(resource)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource to JSON: %w", err)
	}

	var jsonData interface{}
	if err := json.Unmarshal(resourceJSON, &jsonData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	extractor := a.constructor.Spec.Extractor
	data := &ExtractedData{}

	// Extract image URL
	if extractor.ImageURL != "" {
		imageURL, err := a.executeJQExpression(extractor.ImageURL, jsonData)
		if err != nil {
			return nil, fmt.Errorf("failed to extract image URL: %w", err)
		}
		data.ImageURL = imageURL
	}

	// Extract image digest
	if extractor.ImageDigest != "" {
		imageDigest, err := a.executeJQExpression(extractor.ImageDigest, jsonData)
		if err != nil {
			return nil, fmt.Errorf("failed to extract image digest: %w", err)
		}
		data.ImageDigest = imageDigest
	}

	// Extract component name
	if extractor.Name != "" {
		componentName, err := a.executeJQExpression(extractor.Name, jsonData)
		if err != nil {
			return nil, fmt.Errorf("failed to extract component name: %w", err)
		}
		data.ComponentName = componentName
	}

	// Extract git source information
	if extractor.Source.Git.URL != "" {
		gitURL, err := a.executeJQExpression(extractor.Source.Git.URL, jsonData)
		if err != nil {
			return nil, fmt.Errorf("failed to extract git URL: %w", err)
		}

		data.GitSource = &integrationv1alpha1.TestSubjectGitSource{
			URL: gitURL,
		}

		if extractor.Source.Git.Revision != "" {
			gitRevision, err := a.executeJQExpression(extractor.Source.Git.Revision, jsonData)
			if err != nil {
				return nil, fmt.Errorf("failed to extract git revision: %w", err)
			}
			data.GitSource.Revision = gitRevision
		}

		if extractor.Source.Git.Context != "" {
			gitContext, err := a.executeJQExpression(extractor.Source.Git.Context, jsonData)
			if err != nil {
				return nil, fmt.Errorf("failed to extract git context: %w", err)
			}
			data.GitSource.Context = gitContext
		}
	}

	// Determine group label (default to "default" if not specified in template)
	data.GroupLabel = "default"
	if a.constructor.Spec.Template.Labels != nil {
		if groupLabel, exists := a.constructor.Spec.Template.Labels[integrationv1alpha1.TestSubjectGroupLabel]; exists {
			data.GroupLabel = groupLabel
		}
	}

	return data, nil
}

// executeJQExpression executes a JQ expression against JSON data
func (a *Adapter) executeJQExpression(expression string, data interface{}) (string, error) {
	query, err := gojq.Parse(expression)
	if err != nil {
		return "", fmt.Errorf("failed to parse JQ expression %q: %w", expression, err)
	}

	iter := query.Run(data)
	result, ok := iter.Next()
	if !ok {
		return "", fmt.Errorf("JQ expression %q returned no results", expression)
	}

	if err, ok := result.(error); ok {
		return "", fmt.Errorf("JQ expression %q failed: %w", expression, err)
	}

	// Convert result to string
	switch v := result.(type) {
	case string:
		return v, nil
	case nil:
		return "", nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

// getControlTestSubject gets the current control TestSubject for the given group
func (a *Adapter) getControlTestSubject(groupLabel string) (*integrationv1alpha1.TestSubject, error) {
	testSubjects := &integrationv1alpha1.TestSubjectList{}
	if err := a.client.List(a.context, testSubjects, &client.ListOptions{
		Namespace: a.constructor.Namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			integrationv1alpha1.TestSubjectGroupLabel:   groupLabel,
			integrationv1alpha1.ControlTestSubjectLabel: "true",
		}),
	}); err != nil {
		return nil, err
	}

	if len(testSubjects.Items) == 0 {
		return nil, nil // No control TestSubject exists
	}

	if len(testSubjects.Items) > 1 {
		a.logger.Info("Multiple control TestSubjects found, using the first one", "count", len(testSubjects.Items))
	}

	return &testSubjects.Items[0], nil
}

// createTestSubject creates a new TestSubject based on extracted data and control TestSubject
func (a *Adapter) createTestSubject(extractedData *ExtractedData, controlTestSubject *integrationv1alpha1.TestSubject) (*integrationv1alpha1.TestSubject, error) {
	// Generate a unique name for the TestSubject
	testSubjectName := fmt.Sprintf("%s-%s-%d",
		a.constructor.Name,
		extractedData.ComponentName,
		time.Now().Unix())

	// Start with the control TestSubject as the basis if it exists
	var components []integrationv1alpha1.TestSubjectComponent
	if controlTestSubject != nil {
		components = make([]integrationv1alpha1.TestSubjectComponent, len(controlTestSubject.Spec.Components))
		copy(components, controlTestSubject.Spec.Components)
	}

	// Update or add the component with new data
	componentUpdated := false
	for i, component := range components {
		if component.Name == extractedData.ComponentName {
			// Update existing component
			components[i].ContainerImage = extractedData.ImageURL
			if extractedData.GitSource != nil {
				components[i].Source.Git = extractedData.GitSource
			}
			componentUpdated = true
			break
		}
	}

	// If component wasn't updated, add it as new
	if !componentUpdated {
		newComponent := integrationv1alpha1.TestSubjectComponent{
			Name:           extractedData.ComponentName,
			ContainerImage: extractedData.ImageURL,
		}
		if extractedData.GitSource != nil {
			newComponent.Source.Git = extractedData.GitSource
		}
		components = append(components, newComponent)
	}

	// Create the TestSubject
	testSubject := &integrationv1alpha1.TestSubject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testSubjectName,
			Namespace: a.constructor.Namespace,
			Labels:    make(map[string]string),
		},
		Spec: integrationv1alpha1.TestSubjectSpec{
			Components: components,
		},
	}

	// Set the group label
	testSubject.Labels[integrationv1alpha1.TestSubjectGroupLabel] = extractedData.GroupLabel

	// Apply template labels and annotations
	if a.constructor.Spec.Template.Labels != nil {
		for k, v := range a.constructor.Spec.Template.Labels {
			testSubject.Labels[k] = v
		}
	}
	if a.constructor.Spec.Template.Annotations != nil {
		if testSubject.Annotations == nil {
			testSubject.Annotations = make(map[string]string)
		}
		for k, v := range a.constructor.Spec.Template.Annotations {
			testSubject.Annotations[k] = v
		}
	}

	// Set git source at the TestSubject level if specified
	if extractedData.GitSource != nil {
		testSubject.Spec.Source.Git = extractedData.GitSource
	}

	// Create the TestSubject
	if err := a.client.Create(a.context, testSubject); err != nil {
		return nil, err
	}

	return testSubject, nil
}

// updateProcessedResourceStatus updates the status with information about the processed resource
func (a *Adapter) updateProcessedResourceStatus(resource client.Object, createdTestSubject *integrationv1alpha1.TestSubject) error {
	a.constructor.Status.LastProcessedResource = &integrationv1alpha1.TestSubjectProcessedResource{
		Name:               resource.GetName(),
		Namespace:          resource.GetNamespace(),
		Kind:               resource.GetObjectKind().GroupVersionKind().Kind,
		APIVersion:         resource.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		ProcessedAt:        &metav1.Time{Time: time.Now()},
		CreatedTestSubject: createdTestSubject.Name,
	}

	return a.client.Status().Update(a.context, a.constructor)
}
