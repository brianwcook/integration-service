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

package integration

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	integrationv1alpha1 "github.com/konflux-ci/integration-service/api/integration/v1alpha1"
)

var _ = Describe("TestSubject Webhook Integration Tests", func() {
	var testNamespace string

	BeforeEach(func() {
		testNamespace = "webhook-test-" + GenerateRandomString(8)
		CreateNamespace(testNamespace)
	})

	AfterEach(func() {
		DeleteNamespace(testNamespace)
	})

	Context("TestSubject Components Immutability", func() {
		It("should allow creating TestSubject with components", func() {
			By("Creating a TestSubject with components")
			testSubject := &integrationv1alpha1.TestSubject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-subject-create",
					Namespace: testNamespace,
					Labels: map[string]string{
						integrationv1alpha1.TestSubjectGroupLabel: "create-test-app",
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

			Expect(k8sClient.Create(ctx, testSubject)).To(Succeed())

			By("Verifying the TestSubject was created successfully")
			createdTestSubject := &integrationv1alpha1.TestSubject{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-subject-create",
					Namespace: testNamespace,
				}, createdTestSubject)
			}, time.Minute, time.Second).Should(Succeed())

			Expect(createdTestSubject.Spec.Components).To(HaveLen(1))
			Expect(createdTestSubject.Spec.Components[0].Name).To(Equal("frontend"))
			Expect(createdTestSubject.Spec.Components[0].ContainerImage).To(Equal("quay.io/myorg/frontend:v1.0.0"))
		})

		It("should reject adding components to existing TestSubject", func() {
			By("Creating a TestSubject with one component")
			testSubject := &integrationv1alpha1.TestSubject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-subject-add",
					Namespace: testNamespace,
					Labels: map[string]string{
						integrationv1alpha1.TestSubjectGroupLabel: "add-test-app",
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

			Expect(k8sClient.Create(ctx, testSubject)).To(Succeed())

			By("Attempting to add a new component")
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-subject-add",
					Namespace: testNamespace,
				}, testSubject)
			}, time.Minute, time.Second).Should(Succeed())

			// Try to add a new component
			testSubject.Spec.Components = append(testSubject.Spec.Components, integrationv1alpha1.TestSubjectComponent{
				Name:           "database",
				ContainerImage: "quay.io/myorg/database:v1.0.0",
			})

			By("Verifying the update is rejected by the webhook")
			err := k8sClient.Update(ctx, testSubject)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("components field is immutable"))
		})

		It("should reject modifying existing components", func() {
			By("Creating a TestSubject with components")
			testSubject := &integrationv1alpha1.TestSubject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-subject-modify",
					Namespace: testNamespace,
					Labels: map[string]string{
						integrationv1alpha1.TestSubjectGroupLabel: "modify-test-app",
					},
				},
				Spec: integrationv1alpha1.TestSubjectSpec{
					Components: []integrationv1alpha1.TestSubjectComponent{
						{
							Name:           "api",
							ContainerImage: "quay.io/myorg/api:v1.0.0",
							Source: integrationv1alpha1.TestSubjectComponentSource{
								Git: &integrationv1alpha1.TestSubjectGitSource{
									URL:      "https://github.com/myorg/api",
									Revision: "main",
								},
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, testSubject)).To(Succeed())

			By("Attempting to modify the container image")
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-subject-modify",
					Namespace: testNamespace,
				}, testSubject)
			}, time.Minute, time.Second).Should(Succeed())

			// Try to modify the container image
			testSubject.Spec.Components[0].ContainerImage = "quay.io/myorg/api:v2.0.0"

			By("Verifying the update is rejected by the webhook")
			err := k8sClient.Update(ctx, testSubject)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("components field is immutable"))
		})

		It("should reject removing components", func() {
			By("Creating a TestSubject with multiple components")
			testSubject := &integrationv1alpha1.TestSubject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-subject-remove",
					Namespace: testNamespace,
					Labels: map[string]string{
						integrationv1alpha1.TestSubjectGroupLabel: "remove-test-app",
					},
				},
				Spec: integrationv1alpha1.TestSubjectSpec{
					Components: []integrationv1alpha1.TestSubjectComponent{
						{
							Name:           "frontend",
							ContainerImage: "quay.io/myorg/frontend:v1.0.0",
						},
						{
							Name:           "backend",
							ContainerImage: "quay.io/myorg/backend:v1.0.0",
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, testSubject)).To(Succeed())

			By("Attempting to remove a component")
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-subject-remove",
					Namespace: testNamespace,
				}, testSubject)
			}, time.Minute, time.Second).Should(Succeed())

			// Try to remove the last component
			testSubject.Spec.Components = testSubject.Spec.Components[:1]

			By("Verifying the update is rejected by the webhook")
			err := k8sClient.Update(ctx, testSubject)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("components field is immutable"))
		})

		It("should reject reordering components", func() {
			By("Creating a TestSubject with multiple components")
			testSubject := &integrationv1alpha1.TestSubject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-subject-reorder",
					Namespace: testNamespace,
					Labels: map[string]string{
						integrationv1alpha1.TestSubjectGroupLabel: "reorder-test-app",
					},
				},
				Spec: integrationv1alpha1.TestSubjectSpec{
					Components: []integrationv1alpha1.TestSubjectComponent{
						{
							Name:           "component-a",
							ContainerImage: "quay.io/myorg/component-a:v1.0.0",
						},
						{
							Name:           "component-b",
							ContainerImage: "quay.io/myorg/component-b:v1.0.0",
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, testSubject)).To(Succeed())

			By("Attempting to reorder components")
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-subject-reorder",
					Namespace: testNamespace,
				}, testSubject)
			}, time.Minute, time.Second).Should(Succeed())

			// Try to swap the order of components
			testSubject.Spec.Components[0], testSubject.Spec.Components[1] =
				testSubject.Spec.Components[1], testSubject.Spec.Components[0]

			By("Verifying the update is rejected by the webhook")
			err := k8sClient.Update(ctx, testSubject)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("components field is immutable"))
		})

		It("should reject modifying source information", func() {
			By("Creating a TestSubject with source information")
			testSubject := &integrationv1alpha1.TestSubject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-subject-source",
					Namespace: testNamespace,
					Labels: map[string]string{
						integrationv1alpha1.TestSubjectGroupLabel: "source-test-app",
					},
				},
				Spec: integrationv1alpha1.TestSubjectSpec{
					Components: []integrationv1alpha1.TestSubjectComponent{
						{
							Name:           "service",
							ContainerImage: "quay.io/myorg/service:v1.0.0",
							Source: integrationv1alpha1.TestSubjectComponentSource{
								Git: &integrationv1alpha1.TestSubjectGitSource{
									URL:      "https://github.com/myorg/service",
									Revision: "main",
									Context:  ".",
								},
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, testSubject)).To(Succeed())

			By("Attempting to modify git revision")
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-subject-source",
					Namespace: testNamespace,
				}, testSubject)
			}, time.Minute, time.Second).Should(Succeed())

			// Try to modify the git revision
			testSubject.Spec.Components[0].Source.Git.Revision = "develop"

			By("Verifying the update is rejected by the webhook")
			err := k8sClient.Update(ctx, testSubject)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("components field is immutable"))
		})

		It("should allow updates to metadata and non-components fields", func() {
			By("Creating a TestSubject")
			testSubject := &integrationv1alpha1.TestSubject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-subject-metadata",
					Namespace: testNamespace,
					Labels: map[string]string{
						integrationv1alpha1.TestSubjectGroupLabel: "metadata-test-app",
					},
				},
				Spec: integrationv1alpha1.TestSubjectSpec{
					Components: []integrationv1alpha1.TestSubjectComponent{
						{
							Name:           "app",
							ContainerImage: "quay.io/myorg/app:v1.0.0",
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, testSubject)).To(Succeed())

			By("Updating metadata and preserving components")
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-subject-metadata",
					Namespace: testNamespace,
				}, testSubject)
			}, time.Minute, time.Second).Should(Succeed())

			// Update labels and annotations but keep components unchanged
			testSubject.Labels["updated"] = "true"
			if testSubject.Annotations == nil {
				testSubject.Annotations = make(map[string]string)
			}
			testSubject.Annotations["test"] = "annotation"

			By("Verifying the update is accepted by the webhook")
			Expect(k8sClient.Update(ctx, testSubject)).To(Succeed())

			By("Verifying the metadata changes were applied")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-subject-metadata",
					Namespace: testNamespace,
				}, testSubject)
				if err != nil {
					return false
				}
				return testSubject.Labels["updated"] == "true" && testSubject.Annotations["test"] == "annotation"
			}, time.Minute, time.Second).Should(BeTrue())

			By("Verifying components remained unchanged")
			Expect(testSubject.Spec.Components).To(HaveLen(1))
			Expect(testSubject.Spec.Components[0].Name).To(Equal("app"))
			Expect(testSubject.Spec.Components[0].ContainerImage).To(Equal("quay.io/myorg/app:v1.0.0"))
		})

		It("should handle empty components correctly", func() {
			By("Creating a TestSubject with empty components")
			testSubject := &integrationv1alpha1.TestSubject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-subject-empty",
					Namespace: testNamespace,
					Labels: map[string]string{
						integrationv1alpha1.TestSubjectGroupLabel: "empty-test-app",
					},
				},
				Spec: integrationv1alpha1.TestSubjectSpec{
					Components: []integrationv1alpha1.TestSubjectComponent{},
				},
			}

			Expect(k8sClient.Create(ctx, testSubject)).To(Succeed())

			By("Attempting to add components to empty TestSubject")
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-subject-empty",
					Namespace: testNamespace,
				}, testSubject)
			}, time.Minute, time.Second).Should(Succeed())

			// Try to add components to previously empty TestSubject
			testSubject.Spec.Components = []integrationv1alpha1.TestSubjectComponent{
				{
					Name:           "new-component",
					ContainerImage: "quay.io/myorg/new:v1.0.0",
				},
			}

			By("Verifying the update is rejected by the webhook")
			err := k8sClient.Update(ctx, testSubject)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("components field is immutable"))
		})
	})

	Context("TestSubject Webhook Error Handling", func() {
		It("should handle webhook failures gracefully", func() {
			By("Creating a TestSubject and verifying webhook is working")
			testSubject := &integrationv1alpha1.TestSubject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-subject-webhook",
					Namespace: testNamespace,
					Labels: map[string]string{
						integrationv1alpha1.TestSubjectGroupLabel: "webhook-test-app",
					},
				},
				Spec: integrationv1alpha1.TestSubjectSpec{
					Components: []integrationv1alpha1.TestSubjectComponent{
						{
							Name:           "test-component",
							ContainerImage: "quay.io/myorg/test:v1.0.0",
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, testSubject)).To(Succeed())

			By("Verifying webhook validation works")
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-subject-webhook",
					Namespace: testNamespace,
				}, testSubject)
			}, time.Minute, time.Second).Should(Succeed())

			// Try to modify components - should fail
			testSubject.Spec.Components[0].ContainerImage = "quay.io/myorg/test:v2.0.0"
			err := k8sClient.Update(ctx, testSubject)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("components field is immutable"))
		})
	})
})
