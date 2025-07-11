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
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	integrationv1alpha1 "github.com/konflux-ci/integration-service/api/integration/v1alpha1"
	"github.com/konflux-ci/integration-service/helpers"
)

var _ = Describe("End-to-End Integration Tests", func() {
	var testNamespace string

	BeforeEach(func() {
		testNamespace = "e2e-test-" + GenerateRandomString(8)
		CreateNamespace(testNamespace)
	})

	AfterEach(func() {
		DeleteNamespace(testNamespace)
	})

	Context("Complete Build-to-Test Workflow", func() {
		It("should create TestSubject from successful build and run integration tests", func() {
			By("Creating a TestSubjectConstructor")
			_ = CreateTestSubjectConstructor(testNamespace, "build-constructor", "my-app")

			By("Creating IntegrationTestScenarios")
			_ = CreateIntegrationTestScenario(testNamespace, "security-scan", "my-app", false)
			_ = CreateIntegrationTestScenario(testNamespace, "performance-test", "my-app", true)

			By("Creating a successful build PipelineRun")
			_ = CreateSuccessfulBuildPipelineRun(testNamespace, "frontend", "quay.io/myorg/frontend:sha-abc123")

			By("Waiting for TestSubject to be created automatically")
			testSubject := WaitForTestSubjectWithLabel(testNamespace, integrationv1alpha1.TestSubjectGroupLabel, "my-app")
			Expect(testSubject).NotTo(BeNil())
			Expect(testSubject.Labels["created-by"]).To(Equal("build-constructor"))
			Expect(testSubject.Spec.Components).To(HaveLen(1))
			Expect(testSubject.Spec.Components[0].Name).To(Equal("frontend"))
			Expect(testSubject.Spec.Components[0].ContainerImage).To(Equal("quay.io/myorg/frontend:sha-abc123"))

			By("Waiting for integration test PipelineRuns to be created")
			securityPR := WaitForPipelineRun(testNamespace, helpers.PipelineRunScenarioLabel, "security-scan")
			performancePR := WaitForPipelineRun(testNamespace, helpers.PipelineRunScenarioLabel, "performance-test")

			Expect(securityPR.Labels[helpers.PipelineRunTestSubjectLabel]).To(Equal(testSubject.Name))
			Expect(performancePR.Labels[helpers.PipelineRunTestSubjectLabel]).To(Equal(testSubject.Name))

			By("Verifying PipelineRun parameters are set correctly")
			// Check that TEST_SUBJECT_IMAGES parameter is populated
			found := false
			for _, param := range securityPR.Spec.Params {
				if param.Name == "IMAGE_URL" {
					Expect(param.Value.StringVal).To(ContainSubstring("frontend"))
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "IMAGE_URL parameter should be set")
		})
	})

	Context("TestSubject Group Management", func() {
		It("should manage control TestSubject correctly", func() {
			By("Creating a TestSubjectConstructor")
			_ = CreateTestSubjectConstructor(testNamespace, "build-constructor", "multi-component-app")

			By("Creating the first component build")
			_ = CreateSuccessfulBuildPipelineRun(testNamespace, "frontend", "quay.io/myorg/frontend:v1.0.0")

			By("Waiting for the first TestSubject")
			testSubject1 := WaitForTestSubjectWithLabel(testNamespace, integrationv1alpha1.TestSubjectGroupLabel, "multi-component-app")
			Expect(testSubject1.Spec.Components).To(HaveLen(1))
			Expect(testSubject1.Spec.Components[0].Name).To(Equal("frontend"))

			By("Creating the second component build")
			_ = CreateSuccessfulBuildPipelineRun(testNamespace, "backend", "quay.io/myorg/backend:v1.0.0")

			By("Waiting for the second TestSubject to be created")
			Eventually(func() int {
				testSubjects := &integrationv1alpha1.TestSubjectList{}
				err := k8sClient.List(ctx, testSubjects, client.InNamespace(testNamespace))
				if err != nil {
					return 0
				}
				count := 0
				for _, ts := range testSubjects.Items {
					if ts.Labels[integrationv1alpha1.TestSubjectGroupLabel] == "multi-component-app" {
						count++
					}
				}
				return count
			}, time.Minute, time.Second).Should(Equal(2))

			By("Verifying the second TestSubject includes both components")
			testSubjects := &integrationv1alpha1.TestSubjectList{}
			err := k8sClient.List(ctx, testSubjects, client.InNamespace(testNamespace))
			Expect(err).NotTo(HaveOccurred())

			var secondTestSubject *integrationv1alpha1.TestSubject
			for _, ts := range testSubjects.Items {
				if ts.Labels[integrationv1alpha1.TestSubjectGroupLabel] == "multi-component-app" &&
					len(ts.Spec.Components) == 2 {
					secondTestSubject = &ts
					break
				}
			}
			Expect(secondTestSubject).NotTo(BeNil())

			componentNames := make([]string, len(secondTestSubject.Spec.Components))
			for i, comp := range secondTestSubject.Spec.Components {
				componentNames[i] = comp.Name
			}
			Expect(componentNames).To(ContainElements("frontend", "backend"))
		})
	})

	Context("Label Selector Matching", func() {
		It("should only run tests for matching TestSubjects", func() {
			By("Creating TestSubjectConstructors for different groups")
			constructor1 := CreateTestSubjectConstructor(testNamespace, "app1-constructor", "app1")
			constructor2 := CreateTestSubjectConstructor(testNamespace, "app2-constructor", "app2")

			By("Creating IntegrationTestScenarios for specific groups")
			app1Scenario := CreateIntegrationTestScenario(testNamespace, "app1-test", "app1", false)
			app2Scenario := CreateIntegrationTestScenario(testNamespace, "app2-test", "app2", false)

			By("Creating builds for both apps")
			app1Build := CreateSuccessfulBuildPipelineRun(testNamespace, "app1-frontend", "quay.io/myorg/app1-frontend:v1.0.0")
			app2Build := CreateSuccessfulBuildPipelineRun(testNamespace, "app2-frontend", "quay.io/myorg/app2-frontend:v1.0.0")

			// Update the labels to match the different constructors
			app1Build.Labels["appstudio.openshift.io/component"] = "app1-frontend"
			app2Build.Labels["appstudio.openshift.io/component"] = "app2-frontend"
			Expect(k8sClient.Update(ctx, app1Build)).To(Succeed())
			Expect(k8sClient.Update(ctx, app2Build)).To(Succeed())

			By("Waiting for TestSubjects to be created")
			app1TestSubject := WaitForTestSubjectWithLabel(testNamespace, integrationv1alpha1.TestSubjectGroupLabel, "app1")
			app2TestSubject := WaitForTestSubjectWithLabel(testNamespace, integrationv1alpha1.TestSubjectGroupLabel, "app2")

			By("Waiting for PipelineRuns to be created")
			app1PR := WaitForPipelineRun(testNamespace, helpers.PipelineRunScenarioLabel, "app1-test")
			app2PR := WaitForPipelineRun(testNamespace, helpers.PipelineRunScenarioLabel, "app2-test")

			By("Verifying PipelineRuns are associated with correct TestSubjects")
			Expect(app1PR.Labels[helpers.PipelineRunTestSubjectLabel]).To(Equal(app1TestSubject.Name))
			Expect(app2PR.Labels[helpers.PipelineRunTestSubjectLabel]).To(Equal(app2TestSubject.Name))

			By("Verifying no cross-contamination of tests")
			Eventually(func() int {
				pipelineRuns := &tektonv1.PipelineRunList{}
				err := k8sClient.List(ctx, pipelineRuns, client.InNamespace(testNamespace))
				if err != nil {
					return 0
				}
				return len(pipelineRuns.Items)
			}, time.Minute, time.Second).Should(Equal(4)) // 2 builds + 2 tests
		})
	})

	Context("Optional Test Handling", func() {
		It("should create PipelineRuns for both required and optional scenarios", func() {
			By("Creating a TestSubjectConstructor")
			constructor := CreateTestSubjectConstructor(testNamespace, "build-constructor", "test-app")

			By("Creating required and optional scenarios")
			requiredScenario := CreateIntegrationTestScenario(testNamespace, "required-test", "test-app", false)
			optionalScenario := CreateIntegrationTestScenario(testNamespace, "optional-test", "test-app", true)

			By("Creating a successful build")
			buildPR := CreateSuccessfulBuildPipelineRun(testNamespace, "component", "quay.io/myorg/component:v1.0.0")

			By("Waiting for TestSubject to be created")
			testSubject := WaitForTestSubjectWithLabel(testNamespace, integrationv1alpha1.TestSubjectGroupLabel, "test-app")

			By("Waiting for both test PipelineRuns to be created")
			requiredPR := WaitForPipelineRun(testNamespace, helpers.PipelineRunScenarioLabel, "required-test")
			optionalPR := WaitForPipelineRun(testNamespace, helpers.PipelineRunScenarioLabel, "optional-test")

			By("Verifying both PipelineRuns are associated with the TestSubject")
			Expect(requiredPR.Labels[helpers.PipelineRunTestSubjectLabel]).To(Equal(testSubject.Name))
			Expect(optionalPR.Labels[helpers.PipelineRunTestSubjectLabel]).To(Equal(testSubject.Name))

			By("Verifying optional label is set correctly")
			Expect(optionalPR.Labels[helpers.PipelineRunOptionalLabel]).To(Equal("true"))
			Expect(requiredPR.Labels[helpers.PipelineRunOptionalLabel]).To(Equal("false"))
		})
	})

	Context("Error Handling", func() {
		It("should handle invalid JQ expressions gracefully", func() {
			By("Creating a TestSubjectConstructor with invalid JQ")
			constructor := &integrationv1alpha1.TestSubjectConstructor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bad-constructor",
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
						Name:     ".invalid.jq.expression[", // Invalid JQ
						ImageURL: ".status.results[] | select(.name == \"IMAGE_URL\") | .value.stringVal",
					},
					Template: integrationv1alpha1.TestSubjectTemplate{
						Labels: map[string]string{
							integrationv1alpha1.TestSubjectGroupLabel: "error-test",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, constructor)).To(Succeed())

			By("Creating a successful build")
			buildPR := CreateSuccessfulBuildPipelineRun(testNamespace, "component", "quay.io/myorg/component:v1.0.0")

			By("Verifying no TestSubject is created due to JQ error")
			Consistently(func() int {
				testSubjects := &integrationv1alpha1.TestSubjectList{}
				err := k8sClient.List(ctx, testSubjects, client.InNamespace(testNamespace))
				if err != nil {
					return 0
				}
				count := 0
				for _, ts := range testSubjects.Items {
					if ts.Labels[integrationv1alpha1.TestSubjectGroupLabel] == "error-test" {
						count++
					}
				}
				return count
			}, 30*time.Second, 5*time.Second).Should(Equal(0))
		})
	})
})

// GenerateRandomString generates a random string of specified length
func GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}
