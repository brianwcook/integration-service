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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// TestSubjectGroupLabel is the label used to group TestSubjects
	TestSubjectGroupLabel = "integration.konflux-ci.dev/test-subject-group"

	// ControlTestSubjectLabel is the label used to mark the control TestSubject
	ControlTestSubjectLabel = "integration.konflux-ci.dev/control-test-subject"
)

// TestSubjectSpec defines the desired state of TestSubject
type TestSubjectSpec struct {
	// Components is a list of components that make up the test subject
	Components []TestSubjectComponent `json:"components"`

	// Source contains information about the source code that triggered this test subject
	Source TestSubjectSource `json:"source,omitempty"`
}

// TestSubjectComponent represents a component within a test subject
type TestSubjectComponent struct {
	// Name is the name of the component
	Name string `json:"name"`

	// ContainerImage is the container image for this component
	ContainerImage string `json:"containerImage"`

	// Source contains information about the source code for this component
	Source TestSubjectComponentSource `json:"source,omitempty"`
}

// TestSubjectComponentSource represents the source information for a component
type TestSubjectComponentSource struct {
	// Git contains git source information
	Git *TestSubjectGitSource `json:"git,omitempty"`
}

// TestSubjectSource represents the source information for the test subject
type TestSubjectSource struct {
	// Git contains git source information
	Git *TestSubjectGitSource `json:"git,omitempty"`
}

// TestSubjectGitSource represents git source information
type TestSubjectGitSource struct {
	// URL is the git repository URL
	URL string `json:"url"`

	// Revision is the git commit SHA or branch/tag
	Revision string `json:"revision"`

	// Context is the directory context within the git repository
	Context string `json:"context,omitempty"`
}

// TestSubjectStatus defines the observed state of TestSubject
type TestSubjectStatus struct {
	// Conditions represent the latest available observations of the TestSubject state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// TestResults contains results from integration tests
	TestResults []TestSubjectTestResult `json:"testResults,omitempty"`
}

// TestSubjectTestResult represents the result of a test on this TestSubject
type TestSubjectTestResult struct {
	// Scenario is the name of the IntegrationTestScenario that was executed
	Scenario string `json:"scenario"`

	// Status is the status of the test (InProgress, Passed, Failed, etc.)
	Status string `json:"status"`

	// Summary contains a summary of the test results
	Summary string `json:"summary,omitempty"`

	// StartTime is when the test started
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the test completed
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// LogURL is the URL to the test logs
	LogURL string `json:"logURL,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ts
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Group",type="string",JSONPath=".metadata.labels['integration.konflux-ci.dev/test-subject-group']"
// +kubebuilder:printcolumn:name="Control",type="string",JSONPath=".metadata.labels['integration.konflux-ci.dev/control-test-subject']"

// TestSubject is the Schema for the testsubjects API
type TestSubject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TestSubjectSpec   `json:"spec,omitempty"`
	Status TestSubjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TestSubjectList contains a list of TestSubject
type TestSubjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TestSubject `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TestSubject{}, &TestSubjectList{})
}
