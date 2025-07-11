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

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	integrationv1alpha1 "github.com/konflux-ci/integration-service/api/integration/v1alpha1"
)

var _ = Describe("TestSubjectConstructor Controller", func() {
	var (
		ctx         context.Context
		k8sClient   client.Client
		reconciler  *Reconciler
		scheme      *runtime.Scheme
		constructor *integrationv1alpha1.TestSubjectConstructor
		pipelineRun *tektonv1.PipelineRun
		namespace   = "default"
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(integrationv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(tektonv1.AddToScheme(scheme)).To(Succeed())

		constructor = &integrationv1alpha1.TestSubjectConstructor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-constructor",
				Namespace: namespace,
			},
			Spec: integrationv1alpha1.TestSubjectConstructorSpec{
				Selector: integrationv1alpha1.TestSubjectSelector{
					Fields: integrationv1alpha1.TestSubjectFieldSelector{
						Match: map[string]string{
							"kind": "PipelineRun",
						},
					},
					Labels: integrationv1alpha1.TestSubjectLabelSelector{
						Match: map[string]string{
							"pipelines.appstudio.openshift.io/type": "build",
						},
					},
				},
				Extractor: integrationv1alpha1.TestSubjectExtractor{
					Name:     ".metadata.labels[\"appstudio.openshift.io/component\"]",
					ImageURL: ".status.results[] | select(.name == \"IMAGE_URL\") | .value",
					Source: integrationv1alpha1.TestSubjectSourceExtractor{
						Git: integrationv1alpha1.TestSubjectGitSourceExtractor{
							URL:      ".status.results[] | select(.name == \"CHAINS-GIT_URL\") | .value",
							Revision: ".status.results[] | select(.name == \"CHAINS-GIT_COMMIT\") | .value",
						},
					},
				},
				Template: integrationv1alpha1.TestSubjectTemplate{
					Labels: map[string]string{
						integrationv1alpha1.TestSubjectGroupLabel: "my-app",
						"created-by": "build-constructor",
					},
				},
			},
		}

		pipelineRun = &tektonv1.PipelineRun{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PipelineRun",
				APIVersion: "tekton.dev/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-pipeline-run",
				Namespace: namespace,
				Labels: map[string]string{
					"pipelines.appstudio.openshift.io/type": "build",
					"appstudio.openshift.io/component":      "frontend",
				},
			},
			Spec: tektonv1.PipelineRunSpec{},
			Status: tektonv1.PipelineRunStatus{
				PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
					CompletionTime: &metav1.Time{Time: metav1.Now().Time},
					Results: []tektonv1.PipelineRunResult{
						{
							Name:  "IMAGE_URL",
							Value: *tektonv1.NewStructuredValues("quay.io/myorg/frontend:sha-abc123"),
						},
						{
							Name:  "CHAINS-GIT_URL",
							Value: *tektonv1.NewStructuredValues("https://github.com/myorg/frontend"),
						},
						{
							Name:  "CHAINS-GIT_COMMIT",
							Value: *tektonv1.NewStructuredValues("abc123"),
						},
					},
				},
			},
		}
	})

	Context("When reconciling a TestSubjectConstructor", func() {
		BeforeEach(func() {
			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(constructor).
				WithStatusSubresource(&integrationv1alpha1.TestSubjectConstructor{}).
				Build()

			reconciler = &Reconciler{
				Client: k8sClient,
				Log:    logr.Discard(),
				Scheme: scheme,
			}
		})

		It("should update status to ready", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      constructor.Name,
					Namespace: constructor.Namespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// Check that status was updated
			updatedConstructor := &integrationv1alpha1.TestSubjectConstructor{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      constructor.Name,
				Namespace: constructor.Namespace,
			}, updatedConstructor)
			Expect(err).NotTo(HaveOccurred())

			// Should have Ready condition
			Expect(updatedConstructor.Status.Conditions).To(HaveLen(1))
			Expect(updatedConstructor.Status.Conditions[0].Type).To(Equal("Ready"))
			Expect(updatedConstructor.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
		})
	})

	Context("When processing a triggering resource", func() {
		BeforeEach(func() {
			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(constructor, pipelineRun).
				WithStatusSubresource(&integrationv1alpha1.TestSubjectConstructor{}).
				WithStatusSubresource(&integrationv1alpha1.TestSubject{}).
				Build()

			reconciler = &Reconciler{
				Client: k8sClient,
				Log:    logr.Discard(),
				Scheme: scheme,
			}
		})

		It("should create TestSubject when PipelineRun matches selector", func() {
			// Test the resource matching logic
			matches := reconciler.resourceMatchesConstructor(pipelineRun, constructor)
			Expect(matches).To(BeTrue())

			// Process the triggering resource
			result, err := reconciler.ReconcileTriggeredResource(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      pipelineRun.Name,
					Namespace: pipelineRun.Namespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// Verify TestSubject was created
			testSubjects := &integrationv1alpha1.TestSubjectList{}
			err = k8sClient.List(ctx, testSubjects, client.InNamespace(namespace))
			Expect(err).NotTo(HaveOccurred())
			Expect(testSubjects.Items).To(HaveLen(1))

			testSubject := testSubjects.Items[0]
			Expect(testSubject.Labels[integrationv1alpha1.TestSubjectGroupLabel]).To(Equal("my-app"))
			Expect(testSubject.Labels["created-by"]).To(Equal("build-constructor"))
			Expect(testSubject.Spec.Components).To(HaveLen(1))
			Expect(testSubject.Spec.Components[0].Name).To(Equal("frontend"))
			Expect(testSubject.Spec.Components[0].ContainerImage).To(Equal("quay.io/myorg/frontend:sha-abc123"))
		})

		It("should not create TestSubject when PipelineRun doesn't match selector", func() {
			// Update PipelineRun to not match
			pipelineRun.Labels["pipelines.appstudio.openshift.io/type"] = "test"
			err := k8sClient.Update(ctx, pipelineRun)
			Expect(err).NotTo(HaveOccurred())

			// Test the resource matching logic
			matches := reconciler.resourceMatchesConstructor(pipelineRun, constructor)
			Expect(matches).To(BeFalse())

			result, err := reconciler.ReconcileTriggeredResource(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      pipelineRun.Name,
					Namespace: pipelineRun.Namespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// Verify no TestSubject was created
			testSubjects := &integrationv1alpha1.TestSubjectList{}
			err = k8sClient.List(ctx, testSubjects, client.InNamespace(namespace))
			Expect(err).NotTo(HaveOccurred())
			Expect(testSubjects.Items).To(HaveLen(0))
		})
	})

	Context("When updating control TestSubject", func() {
		var controlTestSubject *integrationv1alpha1.TestSubject

		BeforeEach(func() {
			controlTestSubject = &integrationv1alpha1.TestSubject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "control-test-subject",
					Namespace: namespace,
					Labels: map[string]string{
						integrationv1alpha1.TestSubjectGroupLabel:   "my-app",
						integrationv1alpha1.ControlTestSubjectLabel: "true",
					},
				},
				Spec: integrationv1alpha1.TestSubjectSpec{
					Components: []integrationv1alpha1.TestSubjectComponent{
						{
							Name:           "backend",
							ContainerImage: "quay.io/myorg/backend:v1.0.0",
						},
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(constructor, pipelineRun, controlTestSubject).
				WithStatusSubresource(&integrationv1alpha1.TestSubjectConstructor{}).
				WithStatusSubresource(&integrationv1alpha1.TestSubject{}).
				Build()

			reconciler = &Reconciler{
				Client: k8sClient,
				Log:    logr.Discard(),
				Scheme: scheme,
			}
		})

		It("should update control TestSubject with new component", func() {
			result, err := reconciler.ReconcileTriggeredResource(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      pipelineRun.Name,
					Namespace: pipelineRun.Namespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// Verify new TestSubject was created (not the control one)
			testSubjects := &integrationv1alpha1.TestSubjectList{}
			err = k8sClient.List(ctx, testSubjects, client.InNamespace(namespace))
			Expect(err).NotTo(HaveOccurred())
			Expect(testSubjects.Items).To(HaveLen(2)) // control + new

			// Find the new TestSubject (not the control)
			var newTestSubject *integrationv1alpha1.TestSubject
			for _, ts := range testSubjects.Items {
				if ts.Labels[integrationv1alpha1.ControlTestSubjectLabel] != "true" {
					newTestSubject = &ts
					break
				}
			}
			Expect(newTestSubject).NotTo(BeNil())

			// New TestSubject should have both components (from control + new)
			Expect(newTestSubject.Spec.Components).To(HaveLen(2))
			componentNames := make([]string, len(newTestSubject.Spec.Components))
			for i, comp := range newTestSubject.Spec.Components {
				componentNames[i] = comp.Name
			}
			Expect(componentNames).To(ContainElements("backend", "frontend"))
		})
	})

	Context("When handling JQ extraction errors", func() {
		BeforeEach(func() {
			// Create constructor with invalid JQ expression
			badConstructor := constructor.DeepCopy()
			badConstructor.Name = "bad-constructor"
			badConstructor.Spec.Extractor.Name = "[[{invalid@#$%^&*()_+}]]"

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(badConstructor, pipelineRun).
				WithStatusSubresource(&integrationv1alpha1.TestSubjectConstructor{}).
				WithStatusSubresource(&integrationv1alpha1.TestSubject{}).
				Build()

			reconciler = &Reconciler{
				Client: k8sClient,
				Log:    logr.Discard(),
				Scheme: scheme,
			}
		})

		It("should handle JQ extraction errors gracefully", func() {
			result, err := reconciler.ReconcileTriggeredResource(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      pipelineRun.Name,
					Namespace: pipelineRun.Namespace,
				},
			})

			// Should handle error gracefully without crashing
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// Should not create any TestSubjects
			testSubjects := &integrationv1alpha1.TestSubjectList{}
			err = k8sClient.List(ctx, testSubjects, client.InNamespace(namespace))
			Expect(err).NotTo(HaveOccurred())
			Expect(testSubjects.Items).To(HaveLen(0))
		})
	})

	Context("When TestSubjectConstructor doesn't exist", func() {
		BeforeEach(func() {
			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&integrationv1alpha1.TestSubjectConstructor{}).
				Build()

			reconciler = &Reconciler{
				Client: k8sClient,
				Log:    logr.Discard(),
				Scheme: scheme,
			}
		})

		It("should handle not found gracefully", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent",
					Namespace: namespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})
	})
})
