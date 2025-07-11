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
	"sigs.k8s.io/controller-runtime/pkg/client"

	integrationv1alpha1 "github.com/konflux-ci/integration-service/api/integration/v1alpha1"
	"github.com/konflux-ci/integration-service/helpers"
	"github.com/konflux-ci/integration-service/tekton"
	v1 "k8s.io/api/core/v1"
)

var _ = Describe("End-to-End Integration Tests - Real Kind Cluster", func() {
	var testNamespace string

	BeforeEach(func() {
		testNamespace = CreateTestNamespace()
		By("Created test namespace: " + testNamespace)
	})

	AfterEach(func() {
		By("Cleaning up test namespace: " + testNamespace)
		DeleteTestNamespace(testNamespace)
	})

	Context("ADR-0033 TestSubjectConstructor Workflow", func() {
		It("should create TestSubject from build PipelineRun and trigger integration tests", func() {
			By("Creating a TestSubjectConstructor")
			constructor := CreateTestSubjectConstructor(testNamespace, "build-constructor", "my-app")
			Expect(constructor).NotTo(BeNil())

			By("Creating IntegrationTestScenarios")
			securityScenario := CreateIntegrationTestScenario(testNamespace, "security-scan", "my-app", false)
			performanceScenario := CreateIntegrationTestScenario(testNamespace, "performance-test", "my-app", true)
			Expect(securityScenario).NotTo(BeNil())
			Expect(performanceScenario).NotTo(BeNil())

			By("Creating a successful build PipelineRun")
			buildPR := CreateSuccessfulBuildPipelineRun(testNamespace, "frontend", "quay.io/myorg/frontend:v1.2.3")
			Expect(buildPR).NotTo(BeNil())

			By("Waiting for TestSubjectConstructor to process the build and create TestSubject")
			testSubject := WaitForTestSubjectWithLabel(testNamespace, integrationv1alpha1.TestSubjectGroupLabel, "my-app")
			Expect(testSubject).NotTo(BeNil())
			Expect(testSubject.Labels["created-by"]).To(Equal("build-constructor"))
			Expect(testSubject.Spec.Components).To(HaveLen(1))
			Expect(testSubject.Spec.Components[0].Name).To(Equal("frontend"))
			Expect(testSubject.Spec.Components[0].ContainerImage).To(Equal("quay.io/myorg/frontend:v1.2.3"))

			By("Waiting for integration test PipelineRuns to be created")
			securityPR := WaitForPipelineRun(testNamespace, helpers.PipelineRunScenarioLabel, "security-scan")
			performancePR := WaitForPipelineRun(testNamespace, helpers.PipelineRunScenarioLabel, "performance-test")

			By("Verifying PipelineRuns are correctly labeled and configured")
			Expect(securityPR.Labels[helpers.PipelineRunTestSubjectLabel]).To(Equal(testSubject.Name))
			Expect(performancePR.Labels[helpers.PipelineRunTestSubjectLabel]).To(Equal(testSubject.Name))

			// Verify optional labeling
			Expect(securityPR.Labels[tekton.OptionalLabel]).To(Equal("false"))
			Expect(performancePR.Labels[tekton.OptionalLabel]).To(Equal("true"))

			By("Verifying PipelineRun parameters contain TestSubject data")
			found := false
			for _, param := range securityPR.Spec.Params {
				if param.Name == "IMAGE_URL" {
					Expect(param.Value.StringVal).To(ContainSubstring("frontend"))
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "IMAGE_URL parameter should be populated from TestSubject")
		})
	})

	Context("Multi-Component TestSubject Management", func() {
		It("should build cumulative TestSubjects as components are built", func() {
			By("Creating a TestSubjectConstructor for multi-component app")
			constructor := CreateTestSubjectConstructor(testNamespace, "multi-constructor", "multi-app")
			Expect(constructor).NotTo(BeNil())

			By("Creating first component build")
			frontendBuild := CreateSuccessfulBuildPipelineRun(testNamespace, "frontend", "quay.io/myorg/frontend:v1.0.0")
			Expect(frontendBuild).NotTo(BeNil())

			By("Waiting for first TestSubject")
			firstTestSubject := WaitForTestSubjectWithLabel(testNamespace, integrationv1alpha1.TestSubjectGroupLabel, "multi-app")
			Expect(firstTestSubject.Spec.Components).To(HaveLen(1))
			Expect(firstTestSubject.Spec.Components[0].Name).To(Equal("frontend"))

			By("Creating second component build")
			backendBuild := CreateSuccessfulBuildPipelineRun(testNamespace, "backend", "quay.io/myorg/backend:v2.0.0")
			Expect(backendBuild).NotTo(BeNil())

			By("Waiting for new TestSubject with both components")
			Eventually(func() bool {
				testSubjects := &integrationv1alpha1.TestSubjectList{}
				err := k8sClient.List(ctx, testSubjects, client.InNamespace(testNamespace))
				if err != nil {
					return false
				}

				for _, ts := range testSubjects.Items {
					if ts.Labels != nil &&
						ts.Labels[integrationv1alpha1.TestSubjectGroupLabel] == "multi-app" &&
						len(ts.Spec.Components) == 2 {

						// Verify both components are present
						componentNames := make(map[string]bool)
						for _, comp := range ts.Spec.Components {
							componentNames[comp.Name] = true
						}
						return componentNames["frontend"] && componentNames["backend"]
					}
				}
				return false
			}, 3*time.Minute, 15*time.Second).Should(BeTrue(), "TestSubject with both components should be created")
		})
	})

	Context("TestSubject Validation Webhook", func() {
		It("should validate TestSubject creation and prevent invalid configurations", func() {
			By("Attempting to create TestSubject with invalid component configuration")
			invalidTestSubject := &integrationv1alpha1.TestSubject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-test-subject",
					Namespace: testNamespace,
				},
				Spec: integrationv1alpha1.TestSubjectSpec{
					Components: []integrationv1alpha1.TestSubjectComponent{
						{
							Name:           "", // Invalid: empty name
							ContainerImage: "quay.io/myorg/invalid:latest",
						},
					},
				},
			}

			By("Expecting webhook to reject invalid TestSubject")
			err := k8sClient.Create(ctx, invalidTestSubject)
			Expect(err).To(HaveOccurred(), "Webhook should reject TestSubject with empty component name")
		})

		It("should allow valid TestSubject creation", func() {
			By("Creating a valid TestSubject")
			validTestSubject := &integrationv1alpha1.TestSubject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-test-subject",
					Namespace: testNamespace,
				},
				Spec: integrationv1alpha1.TestSubjectSpec{
					Components: []integrationv1alpha1.TestSubjectComponent{
						{
							Name:           "valid-component",
							ContainerImage: "quay.io/myorg/valid:v1.0.0",
						},
					},
				},
			}

			By("Expecting webhook to accept valid TestSubject")
			err := k8sClient.Create(ctx, validTestSubject)
			Expect(err).NotTo(HaveOccurred(), "Webhook should accept valid TestSubject")
		})
	})

	Context("Error Handling and Resilience", func() {
		It("should handle invalid JQ expressions gracefully", func() {
			By("Creating TestSubjectConstructor with invalid JQ expression")
			invalidConstructor := &integrationv1alpha1.TestSubjectConstructor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-constructor",
					Namespace: testNamespace,
				},
				Spec: integrationv1alpha1.TestSubjectConstructorSpec{
					Selector: integrationv1alpha1.TestSubjectSelector{
						Labels: integrationv1alpha1.TestSubjectLabelSelector{
							Match: map[string]string{
								"pipelines.appstudio.openshift.io/type": "build",
							},
						},
					},
					Extractor: integrationv1alpha1.TestSubjectExtractor{
						Name:     "[[{invalid@#$%^&*()_+}]]", // Invalid JQ
						ImageURL: ".status.results[] | select(.name == \"IMAGE_URL\") | .value",
					},
					Template: integrationv1alpha1.TestSubjectTemplate{
						Labels: map[string]string{
							integrationv1alpha1.TestSubjectGroupLabel: "error-test",
						},
					},
				},
			}

			err := k8sClient.Create(ctx, invalidConstructor)
			Expect(err).NotTo(HaveOccurred(), "Constructor creation should succeed")

			By("Creating a build that would trigger the invalid constructor")
			buildPR := CreateSuccessfulBuildPipelineRun(testNamespace, "error-component", "quay.io/myorg/error:latest")
			Expect(buildPR).NotTo(BeNil())

			By("Verifying no TestSubject is created due to JQ error")
			Consistently(func() bool {
				testSubjects := &integrationv1alpha1.TestSubjectList{}
				err := k8sClient.List(ctx, testSubjects, client.InNamespace(testNamespace))
				if err != nil {
					return false
				}

				for _, ts := range testSubjects.Items {
					if ts.Labels != nil && ts.Labels[integrationv1alpha1.TestSubjectGroupLabel] == "error-test" {
						return false // Found a TestSubject that shouldn't exist
					}
				}
				return true // No invalid TestSubjects found (good)
			}, 30*time.Second, 5*time.Second).Should(BeTrue(), "No TestSubject should be created from invalid JQ")
		})
	})

	Context("Integration Service Controller Health", func() {
		It("should verify all controllers are running and healthy", func() {
			By("Checking integration service deployment status")
			Eventually(func() bool {
				pods := &v1.PodList{}
				err := k8sClient.List(ctx, pods, client.InNamespace("integration-service-system"))
				if err != nil {
					return false
				}

				readyPods := 0
				for _, pod := range pods.Items {
					if pod.Status.Phase == v1.PodRunning {
						readyCount := 0
						for _, condition := range pod.Status.Conditions {
							if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
								readyCount++
							}
						}
						if readyCount > 0 {
							readyPods++
						}
					}
				}
				return readyPods > 0
			}, 2*time.Minute, 10*time.Second).Should(BeTrue(), "Integration service controllers should be healthy")

			By("Verifying Tekton is operational")
			Eventually(func() bool {
				pods := &v1.PodList{}
				err := k8sClient.List(ctx, pods, client.InNamespace("tekton-pipelines"))
				if err != nil {
					return false
				}

				readyPods := 0
				for _, pod := range pods.Items {
					if pod.Status.Phase == v1.PodRunning {
						readyPods++
					}
				}
				return readyPods >= 2 // controller + webhook
			}, 2*time.Minute, 10*time.Second).Should(BeTrue(), "Tekton should be operational")
		})
	})
})
