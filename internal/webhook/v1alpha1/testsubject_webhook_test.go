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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	integrationv1alpha1 "github.com/konflux-ci/integration-service/api/integration/v1alpha1"
)

var _ = Describe("TestSubject webhook", Ordered, func() {

	var (
		testSubject        *integrationv1alpha1.TestSubject
		originalComponents []integrationv1alpha1.TestSubjectComponent
		testNamespace      = "default"
	)

	BeforeAll(func() {
		originalComponents = []integrationv1alpha1.TestSubjectComponent{
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
			{
				Name:           "backend",
				ContainerImage: "quay.io/myorg/backend:v1.0.0",
				Source: integrationv1alpha1.TestSubjectComponentSource{
					Git: &integrationv1alpha1.TestSubjectGitSource{
						URL:      "https://github.com/myorg/backend",
						Revision: "main",
					},
				},
			},
		}
	})

	BeforeEach(func() {
		testSubject = &integrationv1alpha1.TestSubject{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "integration.konflux-ci.dev/v1alpha1",
				Kind:       "TestSubject",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-subject",
				Namespace: testNamespace,
				Labels: map[string]string{
					integrationv1alpha1.TestSubjectGroupLabel: "my-app",
				},
			},
			Spec: integrationv1alpha1.TestSubjectSpec{
				Components: originalComponents,
			},
		}
	})

	AfterEach(func() {
		err := k8sClient.Delete(ctx, testSubject)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
	})

	It("should successfully create a TestSubject with components", func() {
		Expect(k8sClient.Create(ctx, testSubject)).Should(Succeed())

		// Verify the TestSubject was created with the correct components
		createdTestSubject := &integrationv1alpha1.TestSubject{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-subject",
				Namespace: testNamespace,
			}, createdTestSubject)
		}).Should(Succeed())

		Expect(createdTestSubject.Spec.Components).To(Equal(originalComponents))
	})

	It("should reject updates to the components field", func() {
		// Create the TestSubject first
		Expect(k8sClient.Create(ctx, testSubject)).Should(Succeed())

		// Try to update the components field
		updatedTestSubject := &integrationv1alpha1.TestSubject{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-subject",
				Namespace: testNamespace,
			}, updatedTestSubject)
		}).Should(Succeed())

		// Add a new component
		updatedTestSubject.Spec.Components = append(updatedTestSubject.Spec.Components, integrationv1alpha1.TestSubjectComponent{
			Name:           "database",
			ContainerImage: "quay.io/myorg/database:v1.0.0",
			Source: integrationv1alpha1.TestSubjectComponentSource{
				Git: &integrationv1alpha1.TestSubjectGitSource{
					URL:      "https://github.com/myorg/database",
					Revision: "main",
				},
			},
		})

		// This update should fail due to webhook validation
		Expect(k8sClient.Update(ctx, updatedTestSubject)).ShouldNot(Succeed())
	})

	It("should reject modifications to existing components", func() {
		// Create the TestSubject first
		Expect(k8sClient.Create(ctx, testSubject)).Should(Succeed())

		// Try to modify an existing component
		updatedTestSubject := &integrationv1alpha1.TestSubject{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-subject",
				Namespace: testNamespace,
			}, updatedTestSubject)
		}).Should(Succeed())

		// Modify the container image of the first component
		updatedTestSubject.Spec.Components[0].ContainerImage = "quay.io/myorg/frontend:v2.0.0"

		// This update should fail due to webhook validation
		Expect(k8sClient.Update(ctx, updatedTestSubject)).ShouldNot(Succeed())
	})

	It("should reject removal of components", func() {
		// Create the TestSubject first
		Expect(k8sClient.Create(ctx, testSubject)).Should(Succeed())

		// Try to remove a component
		updatedTestSubject := &integrationv1alpha1.TestSubject{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-subject",
				Namespace: testNamespace,
			}, updatedTestSubject)
		}).Should(Succeed())

		// Remove the last component
		updatedTestSubject.Spec.Components = updatedTestSubject.Spec.Components[:len(updatedTestSubject.Spec.Components)-1]

		// This update should fail due to webhook validation
		Expect(k8sClient.Update(ctx, updatedTestSubject)).ShouldNot(Succeed())
	})

	It("should reject reordering of components", func() {
		// Create the TestSubject first
		Expect(k8sClient.Create(ctx, testSubject)).Should(Succeed())

		// Try to reorder components
		updatedTestSubject := &integrationv1alpha1.TestSubject{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-subject",
				Namespace: testNamespace,
			}, updatedTestSubject)
		}).Should(Succeed())

		// Swap the order of components
		updatedTestSubject.Spec.Components[0], updatedTestSubject.Spec.Components[1] =
			updatedTestSubject.Spec.Components[1], updatedTestSubject.Spec.Components[0]

		// This update should fail due to webhook validation
		Expect(k8sClient.Update(ctx, updatedTestSubject)).ShouldNot(Succeed())
	})

	It("should reject modifications to component source", func() {
		// Create the TestSubject first
		Expect(k8sClient.Create(ctx, testSubject)).Should(Succeed())

		// Try to modify component source
		updatedTestSubject := &integrationv1alpha1.TestSubject{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-subject",
				Namespace: testNamespace,
			}, updatedTestSubject)
		}).Should(Succeed())

		// Modify the git revision of the first component
		updatedTestSubject.Spec.Components[0].Source.Git.Revision = "develop"

		// This update should fail due to webhook validation
		Expect(k8sClient.Update(ctx, updatedTestSubject)).ShouldNot(Succeed())
	})

	It("should handle invalid object types gracefully", func() {
		validator := &TestSubjectCustomValidator{}

		// Create a dummy object for testing
		dummyObject := &integrationv1alpha1.IntegrationTestScenario{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dummy",
				Namespace: testNamespace,
			},
		}

		// Test ValidateCreate with wrong object type
		_, err := validator.ValidateCreate(ctx, dummyObject)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("expected a TestSubject object"))

		// Test ValidateUpdate with wrong oldObj type
		_, err = validator.ValidateUpdate(ctx, dummyObject, testSubject)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("expected a TestSubject object for oldObj"))

		// Test ValidateUpdate with wrong newObj type
		_, err = validator.ValidateUpdate(ctx, testSubject, dummyObject)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("expected a TestSubject object for newObj"))

		// Test ValidateDelete with wrong object type
		_, err = validator.ValidateDelete(ctx, dummyObject)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("expected a TestSubject object"))
	})

	It("should directly test validator methods without cluster", func() {
		validator := &TestSubjectCustomValidator{}

		// Test ValidateCreate directly
		_, err := validator.ValidateCreate(ctx, testSubject)
		Expect(err).NotTo(HaveOccurred())

		// Test ValidateUpdate with identical components (should succeed)
		testSubject2 := testSubject.DeepCopy()
		_, err = validator.ValidateUpdate(ctx, testSubject, testSubject2)
		Expect(err).NotTo(HaveOccurred())

		// Test ValidateUpdate with different components (should fail)
		testSubject3 := testSubject.DeepCopy()
		testSubject3.Spec.Components[0].ContainerImage = "quay.io/myorg/frontend:v2.0.0"
		_, err = validator.ValidateUpdate(ctx, testSubject, testSubject3)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("components field is immutable"))

		// Test ValidateDelete directly
		_, err = validator.ValidateDelete(ctx, testSubject)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should allow updates to other fields while preserving components", func() {
		// Create the TestSubject first
		Expect(k8sClient.Create(ctx, testSubject)).Should(Succeed())

		// Update non-components fields
		updatedTestSubject := &integrationv1alpha1.TestSubject{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-subject",
				Namespace: testNamespace,
			}, updatedTestSubject)
		}).Should(Succeed())

		// Modify labels and annotations (but keep components unchanged)
		updatedTestSubject.Labels["updated"] = "true"
		if updatedTestSubject.Annotations == nil {
			updatedTestSubject.Annotations = make(map[string]string)
		}
		updatedTestSubject.Annotations["test"] = "annotation"

		// This update should succeed as components are not modified
		Expect(k8sClient.Update(ctx, updatedTestSubject)).Should(Succeed())

		// Verify the changes were applied
		Eventually(func() error {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-subject",
				Namespace: testNamespace,
			}, updatedTestSubject)
			if err != nil {
				return err
			}
			if updatedTestSubject.Labels["updated"] != "true" {
				return errors.NewBadRequest("Label was not updated")
			}
			if updatedTestSubject.Annotations["test"] != "annotation" {
				return errors.NewBadRequest("Annotation was not updated")
			}
			return nil
		}).Should(Succeed())

		// Verify components remained unchanged
		Expect(updatedTestSubject.Spec.Components).To(Equal(originalComponents))
	})

	It("should handle TestSubject with empty components", func() {
		// Create TestSubject with empty components
		emptyTestSubject := &integrationv1alpha1.TestSubject{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "integration.konflux-ci.dev/v1alpha1",
				Kind:       "TestSubject",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "empty-test-subject",
				Namespace: testNamespace,
				Labels: map[string]string{
					integrationv1alpha1.TestSubjectGroupLabel: "empty-app",
				},
			},
			Spec: integrationv1alpha1.TestSubjectSpec{
				Components: []integrationv1alpha1.TestSubjectComponent{},
			},
		}

		Expect(k8sClient.Create(ctx, emptyTestSubject)).Should(Succeed())

		// Try to add components to empty TestSubject
		updatedEmptyTestSubject := &integrationv1alpha1.TestSubject{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{
				Name:      "empty-test-subject",
				Namespace: testNamespace,
			}, updatedEmptyTestSubject)
		}).Should(Succeed())

		// Add components to previously empty TestSubject
		updatedEmptyTestSubject.Spec.Components = []integrationv1alpha1.TestSubjectComponent{
			{
				Name:           "new-component",
				ContainerImage: "quay.io/myorg/new:v1.0.0",
			},
		}

		// This should fail as components are immutable
		Expect(k8sClient.Update(ctx, updatedEmptyTestSubject)).ShouldNot(Succeed())

		// Cleanup
		err := k8sClient.Delete(ctx, emptyTestSubject)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
	})
})
