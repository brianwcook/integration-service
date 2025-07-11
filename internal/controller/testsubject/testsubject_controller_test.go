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

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	integrationv1alpha1 "github.com/konflux-ci/integration-service/api/integration/v1alpha1"
	"github.com/konflux-ci/integration-service/helpers"
)

var _ = Describe("TestSubject Controller", func() {
	var (
		ctx         context.Context
		k8sClient   client.Client
		reconciler  *Reconciler
		scheme      *runtime.Scheme
		testSubject *integrationv1alpha1.TestSubject
		scenario    *integrationv1alpha1.IntegrationTestScenario
		namespace   = "default"
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		// Register core Kubernetes types
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		// Register custom types
		Expect(integrationv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(tektonv1.AddToScheme(scheme)).To(Succeed())

		testSubject = &integrationv1alpha1.TestSubject{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "integration.konflux-ci.dev/v1alpha1",
				Kind:       "TestSubject",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-subject-1",
				Namespace: namespace,
				Labels: map[string]string{
					integrationv1alpha1.TestSubjectGroupLabel: "my-app",
				},
			},
			Spec: integrationv1alpha1.TestSubjectSpec{
				Components: []integrationv1alpha1.TestSubjectComponent{
					{
						Name:           "frontend",
						ContainerImage: "quay.io/myorg/frontend:v1.0.0",
						Source: integrationv1alpha1.TestSubjectComponentSource{
							Git: &integrationv1alpha1.TestSubjectGitSource{
								URL:      "https://github.com/myorg/frontend",
								Revision: "main",
							},
						},
					},
				},
			},
		}

		scenario = &integrationv1alpha1.IntegrationTestScenario{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "integration.konflux-ci.dev/v1alpha1",
				Kind:       "IntegrationTestScenario",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "security-scan",
				Namespace: namespace,
			},
			Spec: integrationv1alpha1.IntegrationTestScenarioSpec{
				Selector: integrationv1alpha1.IntegrationTestScenarioSelector{
					MatchLabels: map[string]string{
						integrationv1alpha1.TestSubjectGroupLabel: "my-app",
					},
				},
				ResolverRef: integrationv1alpha1.ResolverRef{
					Resolver: "git",
					Params: []integrationv1alpha1.ResolverParameter{
						{Name: "url", Value: "https://github.com/myorg/test-definitions"},
						{Name: "revision", Value: "main"},
						{Name: "pathInRepo", Value: "security/scan-pipeline.yaml"},
					},
				},
				Params: []integrationv1alpha1.PipelineParameter{
					{Name: "IMAGE_URL", Value: "$(params.TEST_SUBJECT_IMAGES)"},
				},
			},
		}
	})

	Context("When reconciling a TestSubject", func() {
		var contextTestSubject *integrationv1alpha1.TestSubject
		var contextScenario *integrationv1alpha1.IntegrationTestScenario

		BeforeEach(func() {
			// Create a fresh scheme for this context
			scheme = runtime.NewScheme()
			Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
			Expect(integrationv1alpha1.AddToScheme(scheme)).To(Succeed())
			Expect(tektonv1.AddToScheme(scheme)).To(Succeed())

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&integrationv1alpha1.TestSubject{}).
				Build()

			// Create fresh objects for this context
			contextTestSubject = testSubject.DeepCopy()
			contextScenario = scenario.DeepCopy()

			// Create the objects in the fake client
			err := k8sClient.Create(ctx, contextTestSubject)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Create(ctx, contextScenario)
			Expect(err).NotTo(HaveOccurred())

			reconciler = &Reconciler{
				Client: k8sClient,
				Log:    logr.Discard(),
				Scheme: scheme,
			}
		})

		It("should find matching scenarios", func() {
			result, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      contextTestSubject.Name,
					Namespace: contextTestSubject.Namespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// Verify PipelineRun was created
			pipelineRuns := &tektonv1.PipelineRunList{}
			err = k8sClient.List(ctx, pipelineRuns, client.InNamespace(namespace))
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRuns.Items).To(HaveLen(1))

			pipelineRun := pipelineRuns.Items[0]
			Expect(pipelineRun.Labels[helpers.PipelineRunTestSubjectLabel]).To(Equal(contextTestSubject.Name))
			Expect(pipelineRun.Labels[helpers.PipelineRunScenarioLabel]).To(Equal(contextScenario.Name))
		})

		It("should not create duplicate PipelineRuns", func() {
			// First reconcile
			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      contextTestSubject.Name,
					Namespace: contextTestSubject.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Second reconcile
			_, err = reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      contextTestSubject.Name,
					Namespace: contextTestSubject.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Should still have only one PipelineRun
			pipelineRuns := &tektonv1.PipelineRunList{}
			err = k8sClient.List(ctx, pipelineRuns, client.InNamespace(namespace))
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRuns.Items).To(HaveLen(1))
		})

		It("should skip scenarios that don't match", func() {
			// Create a scenario that doesn't match
			nonMatchingScenario := &integrationv1alpha1.IntegrationTestScenario{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other-test",
					Namespace: namespace,
				},
				Spec: integrationv1alpha1.IntegrationTestScenarioSpec{
					Selector: integrationv1alpha1.IntegrationTestScenarioSelector{
						MatchLabels: map[string]string{
							integrationv1alpha1.TestSubjectGroupLabel: "other-app",
						},
					},
					ResolverRef: integrationv1alpha1.ResolverRef{
						Resolver: "git",
						Params: []integrationv1alpha1.ResolverParameter{
							{Name: "url", Value: "https://github.com/myorg/test-definitions"},
						},
					},
				},
			}

			err := k8sClient.Create(ctx, nonMatchingScenario)
			Expect(err).NotTo(HaveOccurred())

			_, err = reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      contextTestSubject.Name,
					Namespace: contextTestSubject.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Should only create PipelineRun for matching scenario
			pipelineRuns := &tektonv1.PipelineRunList{}
			err = k8sClient.List(ctx, pipelineRuns, client.InNamespace(namespace))
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRuns.Items).To(HaveLen(1))
			Expect(pipelineRuns.Items[0].Labels[helpers.PipelineRunScenarioLabel]).To(Equal("security-scan"))
		})

		It("should handle TestSubject with no matching scenarios", func() {
			// Update TestSubject to have different group
			contextTestSubject.Labels[integrationv1alpha1.TestSubjectGroupLabel] = "different-app"
			err := k8sClient.Update(ctx, contextTestSubject)
			Expect(err).NotTo(HaveOccurred())

			_, err = reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      contextTestSubject.Name,
					Namespace: contextTestSubject.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Should not create any PipelineRuns
			pipelineRuns := &tektonv1.PipelineRunList{}
			err = k8sClient.List(ctx, pipelineRuns, client.InNamespace(namespace))
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRuns.Items).To(HaveLen(0))
		})
	})

	Context("When handling optional scenarios", func() {
		var contextTestSubject *integrationv1alpha1.TestSubject
		var contextScenario *integrationv1alpha1.IntegrationTestScenario
		var optionalScenario *integrationv1alpha1.IntegrationTestScenario

		BeforeEach(func() {
			// Create a fresh scheme for this context
			scheme = runtime.NewScheme()
			Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
			Expect(integrationv1alpha1.AddToScheme(scheme)).To(Succeed())
			Expect(tektonv1.AddToScheme(scheme)).To(Succeed())

			// Create fresh objects for this context
			contextTestSubject = testSubject.DeepCopy()
			contextScenario = scenario.DeepCopy()

			// Create optional scenario
			optionalScenario = contextScenario.DeepCopy()
			optionalScenario.Name = "optional-test"
			optionalScenario.Spec.Optional = true

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&integrationv1alpha1.TestSubject{}).
				Build()

				// Create the objects in the fake client
			err := k8sClient.Create(ctx, contextTestSubject)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Create(ctx, contextScenario)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Create(ctx, optionalScenario)
			Expect(err).NotTo(HaveOccurred())

			reconciler = &Reconciler{
				Client: k8sClient,
				Log:    logr.Discard(),
				Scheme: scheme,
			}
		})

		It("should create PipelineRuns for both required and optional scenarios", func() {
			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      contextTestSubject.Name,
					Namespace: contextTestSubject.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			pipelineRuns := &tektonv1.PipelineRunList{}
			err = k8sClient.List(ctx, pipelineRuns, client.InNamespace(namespace))
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRuns.Items).To(HaveLen(2))
		})
	})

	Context("When TestSubject is marked as control", func() {
		var contextTestSubject *integrationv1alpha1.TestSubject
		var contextScenario *integrationv1alpha1.IntegrationTestScenario
		var controlTestSubject *integrationv1alpha1.TestSubject

		BeforeEach(func() {
			// Create a fresh scheme for this context
			scheme = runtime.NewScheme()
			Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
			Expect(integrationv1alpha1.AddToScheme(scheme)).To(Succeed())
			Expect(tektonv1.AddToScheme(scheme)).To(Succeed())

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&integrationv1alpha1.TestSubject{}).
				Build()

			// Create fresh objects for this context
			contextTestSubject = testSubject.DeepCopy()
			contextScenario = scenario.DeepCopy()

			controlTestSubject = contextTestSubject.DeepCopy()
			controlTestSubject.Labels[integrationv1alpha1.ControlTestSubjectLabel] = "true"

			// Create the objects in the fake client
			err := k8sClient.Create(ctx, controlTestSubject)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Create(ctx, contextScenario)
			Expect(err).NotTo(HaveOccurred())

			reconciler = &Reconciler{
				Client: k8sClient,
				Log:    logr.Discard(),
				Scheme: scheme,
			}
		})

		It("should still run tests for control TestSubject", func() {
			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      controlTestSubject.Name,
					Namespace: controlTestSubject.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			pipelineRuns := &tektonv1.PipelineRunList{}
			err = k8sClient.List(ctx, pipelineRuns, client.InNamespace(namespace))
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRuns.Items).To(HaveLen(1))
		})
	})

	Context("When TestSubject doesn't exist", func() {
		BeforeEach(func() {
			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&integrationv1alpha1.TestSubject{}).
				Build()

			reconciler = &Reconciler{
				Client: k8sClient,
				Log:    logr.Discard(),
				Scheme: scheme,
			}
		})

		It("should handle not found gracefully", func() {
			result, err := reconciler.Reconcile(ctx, ctrl.Request{
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
