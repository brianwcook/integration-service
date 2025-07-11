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
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	integrationv1alpha1 "github.com/konflux-ci/integration-service/api/integration/v1alpha1"
)

var (
	k8sClient client.Client
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Test Suite - Kind Cluster")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("connecting to Kind cluster")

	// Load kubeconfig to connect to the existing Kind cluster
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	Expect(err).NotTo(HaveOccurred())
	Expect(config).NotTo(BeNil())

	// Add all required schemes
	err = integrationv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = tektonv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(config, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	By("verifying connection to cluster")
	nodes := &corev1.NodeList{}
	err = k8sClient.List(ctx, nodes)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(nodes.Items)).To(BeNumerically(">", 0), "Cluster should have at least one node")

	By("verifying integration service is deployed")
	Eventually(func() error {
		deployments := &corev1.PodList{}
		return k8sClient.List(ctx, deployments, client.InNamespace("integration-service-system"))
	}, 2*time.Minute, 10*time.Second).Should(Succeed())

	By("verifying Tekton is installed")
	Eventually(func() error {
		pods := &corev1.PodList{}
		return k8sClient.List(ctx, pods, client.InNamespace("tekton-pipelines"))
	}, 2*time.Minute, 10*time.Second).Should(Succeed())
})

var _ = AfterSuite(func() {
	cancel()
	By("integration tests completed")
})

// Helper functions for real integration tests

// CreateTestNamespace creates a test namespace with a unique name
func CreateTestNamespace() string {
	name := "integration-test-" + rand.String(8)
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
	return name
}

// DeleteTestNamespace deletes a test namespace
func DeleteTestNamespace(name string) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
}

// WaitForTestSubject waits for a TestSubject to exist
func WaitForTestSubject(namespace, name string) *integrationv1alpha1.TestSubject {
	testSubject := &integrationv1alpha1.TestSubject{}
	Eventually(func() error {
		return k8sClient.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      name,
		}, testSubject)
	}, 2*time.Minute, 5*time.Second).Should(Succeed())
	return testSubject
}

// WaitForTestSubjectWithLabel waits for a TestSubject with specific label to exist
func WaitForTestSubjectWithLabel(namespace, labelKey, labelValue string) *integrationv1alpha1.TestSubject {
	var testSubject *integrationv1alpha1.TestSubject
	Eventually(func() bool {
		testSubjects := &integrationv1alpha1.TestSubjectList{}
		err := k8sClient.List(ctx, testSubjects, client.InNamespace(namespace))
		if err != nil {
			return false
		}

		for _, ts := range testSubjects.Items {
			if ts.Labels != nil && ts.Labels[labelKey] == labelValue {
				testSubject = &ts
				return true
			}
		}
		return false
	}, 6*time.Minute, 10*time.Second).Should(BeTrue())
	return testSubject
}

// WaitForPipelineRun waits for a PipelineRun with specific label to exist
func WaitForPipelineRun(namespace, labelKey, labelValue string) *tektonv1.PipelineRun {
	var pipelineRun *tektonv1.PipelineRun
	Eventually(func() bool {
		pipelineRuns := &tektonv1.PipelineRunList{}
		err := k8sClient.List(ctx, pipelineRuns, client.InNamespace(namespace))
		if err != nil {
			return false
		}

		for _, pr := range pipelineRuns.Items {
			if pr.Labels != nil && pr.Labels[labelKey] == labelValue {
				pipelineRun = &pr
				return true
			}
		}
		return false
	}, 6*time.Minute, 10*time.Second).Should(BeTrue())
	return pipelineRun
}

// CreateSuccessfulBuildPipelineRun creates a PipelineRun that simulates a successful build
func CreateSuccessfulBuildPipelineRun(namespace, componentName, imageURL string) *tektonv1.PipelineRun {
	pipelineRun := &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("build-%s-%s", componentName, rand.String(5)),
			Namespace: namespace,
			Labels: map[string]string{
				"pipelines.appstudio.openshift.io/type": "build",
				"appstudio.openshift.io/component":      componentName,
			},
		},
		Spec: tektonv1.PipelineRunSpec{
			// Use inline pipeline spec instead of referencing non-existent pipeline
			PipelineSpec: &tektonv1.PipelineSpec{
				// Declare results at the Pipeline level so they are available in PipelineRun status
				Results: []tektonv1.PipelineResult{
					{
						Name:        "IMAGE_URL",
						Description: "Built image URL",
						Value:       tektonv1.ResultValue{Type: "string", StringVal: "$(tasks.build-task.results.IMAGE_URL)"},
					},
					{
						Name:        "CHAINS-GIT_URL",
						Description: "Git URL",
						Value:       tektonv1.ResultValue{Type: "string", StringVal: "$(tasks.build-task.results.CHAINS-GIT_URL)"},
					},
					{
						Name:        "CHAINS-GIT_COMMIT",
						Description: "Git commit",
						Value:       tektonv1.ResultValue{Type: "string", StringVal: "$(tasks.build-task.results.CHAINS-GIT_COMMIT)"},
					},
				},
				Tasks: []tektonv1.PipelineTask{
					{
						Name: "build-task",
						TaskSpec: &tektonv1.EmbeddedTask{
							TaskSpec: tektonv1.TaskSpec{
								Steps: []tektonv1.Step{
									{
										Name:  "build-and-set-results",
										Image: "registry.access.redhat.com/ubi8/ubi-minimal:latest",
										Script: fmt.Sprintf(`
											echo 'Build completed successfully'
											printf '%s' > $(results.IMAGE_URL.path)
											printf 'https://github.com/myorg/%s' > $(results.CHAINS-GIT_URL.path)
											printf 'abc123%s' > $(results.CHAINS-GIT_COMMIT.path)
										`, imageURL, componentName, rand.String(6)),
									},
								},
								Results: []tektonv1.TaskResult{
									{Name: "IMAGE_URL", Description: "Built image URL"},
									{Name: "CHAINS-GIT_URL", Description: "Git URL"},
									{Name: "CHAINS-GIT_COMMIT", Description: "Git commit"},
								},
							},
						},
					},
				},
			},
		},
		Status: tektonv1.PipelineRunStatus{
			PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
				StartTime:      &metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
				CompletionTime: &metav1.Time{Time: time.Now()},
				Results: []tektonv1.PipelineRunResult{
					{
						Name:  "IMAGE_URL",
						Value: *tektonv1.NewStructuredValues(imageURL),
					},
					{
						Name:  "CHAINS-GIT_URL",
						Value: *tektonv1.NewStructuredValues(fmt.Sprintf("https://github.com/myorg/%s", componentName)),
					},
					{
						Name:  "CHAINS-GIT_COMMIT",
						Value: *tektonv1.NewStructuredValues("abc123" + rand.String(6)),
					},
				},
			},
		},
	}

	// Set the success condition manually using SetCondition
	pipelineRun.Status.SetCondition(&apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionTrue,
	})

	Expect(k8sClient.Create(ctx, pipelineRun)).To(Succeed())

	// Update status separately as it may be a subresource
	Expect(k8sClient.Status().Update(ctx, pipelineRun)).To(Succeed())

	return pipelineRun
}

// CreateIntegrationTestScenario creates an IntegrationTestScenario
func CreateIntegrationTestScenario(namespace, name, groupLabel string, optional bool) *integrationv1alpha1.IntegrationTestScenario {
	scenario := &integrationv1alpha1.IntegrationTestScenario{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				integrationv1alpha1.TestSubjectGroupLabel: groupLabel,
				"test.appstudio.openshift.io/optional":    fmt.Sprintf("%t", optional),
			},
		},
		Spec: integrationv1alpha1.IntegrationTestScenarioSpec{
			Selector: integrationv1alpha1.IntegrationTestScenarioSelector{
				MatchLabels: map[string]string{
					integrationv1alpha1.TestSubjectGroupLabel: groupLabel,
				},
			},
			ResolverRef: integrationv1alpha1.ResolverRef{
				Resolver: "bundles",
				Params: []integrationv1alpha1.ResolverParameter{
					{
						Name:  "bundle",
						Value: "quay.io/konflux-ci/tekton-catalog/pipeline-integration-test:latest",
					},
					{
						Name:  "name",
						Value: "integration-test",
					},
				},
			},
			Params: []integrationv1alpha1.PipelineParameter{
				{
					Name:  "IMAGE_URL",
					Value: "$(test_subject.components.component-name.containerImage)",
				},
			},
			Optional: optional,
		},
	}

	Expect(k8sClient.Create(ctx, scenario)).To(Succeed())
	return scenario
}

// CreateTestSubjectConstructor creates a TestSubjectConstructor
func CreateTestSubjectConstructor(namespace, name, groupLabel string) *integrationv1alpha1.TestSubjectConstructor {
	constructor := &integrationv1alpha1.TestSubjectConstructor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
				Name:     ".metadata.labels[\"appstudio.openshift.io/component\"]",
				ImageURL: ".status.results[] | select(.name == \"IMAGE_URL\") | .value",
			},
			Template: integrationv1alpha1.TestSubjectTemplate{
				Labels: map[string]string{
					integrationv1alpha1.TestSubjectGroupLabel: groupLabel,
					"created-by": name,
				},
			},
		},
	}

	Expect(k8sClient.Create(ctx, constructor)).To(Succeed())
	return constructor
}
