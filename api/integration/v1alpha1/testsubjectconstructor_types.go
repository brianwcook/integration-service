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

// TestSubjectConstructorSpec defines the desired state of TestSubjectConstructor
type TestSubjectConstructorSpec struct {
	// Selector defines which resources should trigger TestSubject construction
	Selector TestSubjectSelector `json:"selector"`

	// Extractor defines how to extract data from the triggering resource
	Extractor TestSubjectExtractor `json:"extractor"`

	// Template defines the template for the TestSubject to be created
	Template TestSubjectTemplate `json:"template,omitempty"`
}

// TestSubjectSelector defines the criteria for selecting resources
type TestSubjectSelector struct {
	// Fields defines field-based selection criteria
	Fields TestSubjectFieldSelector `json:"fields,omitempty"`

	// Labels defines label-based selection criteria
	Labels TestSubjectLabelSelector `json:"labels,omitempty"`
}

// TestSubjectFieldSelector defines field-based selection criteria
type TestSubjectFieldSelector struct {
	// Match defines the resource fields to match
	Match map[string]string `json:"match,omitempty"`
}

// TestSubjectLabelSelector defines label-based selection criteria
type TestSubjectLabelSelector struct {
	// Match defines the labels to match
	Match map[string]string `json:"match,omitempty"`
}

// TestSubjectExtractor defines how to extract data from resources
type TestSubjectExtractor struct {
	// ImageURL is the JQ expression to extract the image URL
	ImageURL string `json:"image_url,omitempty"`

	// ImageDigest is the JQ expression to extract the image digest
	ImageDigest string `json:"image_digest,omitempty"`

	// Name is the JQ expression to extract the component name
	Name string `json:"name,omitempty"`

	// Source defines how to extract source information
	Source TestSubjectSourceExtractor `json:"source,omitempty"`
}

// TestSubjectSourceExtractor defines how to extract source information
type TestSubjectSourceExtractor struct {
	// Git defines how to extract git source information
	Git TestSubjectGitSourceExtractor `json:"git,omitempty"`
}

// TestSubjectGitSourceExtractor defines how to extract git source information
type TestSubjectGitSourceExtractor struct {
	// URL is the JQ expression to extract the git URL
	URL string `json:"url,omitempty"`

	// Revision is the JQ expression to extract the git revision
	Revision string `json:"revision,omitempty"`

	// Context is the JQ expression to extract the git context
	Context string `json:"context,omitempty"`
}

// TestSubjectTemplate defines the template for TestSubject creation
type TestSubjectTemplate struct {
	// Labels defines labels to apply to the created TestSubject
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations defines annotations to apply to the created TestSubject
	Annotations map[string]string `json:"annotations,omitempty"`
}

// TestSubjectConstructorStatus defines the observed state of TestSubjectConstructor
type TestSubjectConstructorStatus struct {
	// Conditions represent the latest available observations of the TestSubjectConstructor state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastProcessedResource contains information about the last processed resource
	LastProcessedResource *TestSubjectProcessedResource `json:"lastProcessedResource,omitempty"`
}

// TestSubjectProcessedResource contains information about a processed resource
type TestSubjectProcessedResource struct {
	// Name is the name of the resource
	Name string `json:"name"`

	// Namespace is the namespace of the resource
	Namespace string `json:"namespace"`

	// Kind is the kind of the resource
	Kind string `json:"kind"`

	// APIVersion is the API version of the resource
	APIVersion string `json:"apiVersion"`

	// ProcessedAt is when the resource was processed
	ProcessedAt *metav1.Time `json:"processedAt,omitempty"`

	// CreatedTestSubject is the name of the TestSubject created from this resource
	CreatedTestSubject string `json:"createdTestSubject,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=tsc
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Last Processed",type="string",JSONPath=".status.lastProcessedResource.name"

// TestSubjectConstructor is the Schema for the testsubjectconstructors API
type TestSubjectConstructor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TestSubjectConstructorSpec   `json:"spec,omitempty"`
	Status TestSubjectConstructorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TestSubjectConstructorList contains a list of TestSubjectConstructor
type TestSubjectConstructorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TestSubjectConstructor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TestSubjectConstructor{}, &TestSubjectConstructorList{})
}
