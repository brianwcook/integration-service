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
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	resolutionv1beta1 "github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/apis"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	integrationv1alpha1 "github.com/konflux-ci/integration-service/api/integration/v1alpha1"
	controllers "github.com/konflux-ci/integration-service/internal/controller"
	testsubjectwebhook "github.com/konflux-ci/integration-service/internal/webhook/v1alpha1"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
	mgr       ctrl.Manager
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Test Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s",
			fmt.Sprintf("1.28.3-%s-%s", runtime.GOOS, runtime.GOARCH)),
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{
				filepath.Join("..", "..", "config", "webhook", "manifests.yaml"),
			},
		},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	// Add all required schemes
	err = integrationv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = tektonv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = resolutionv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Create manager with webhook server
	webhookInstallOptions := &testEnv.WebhookInstallOptions
	mgr, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    webhookInstallOptions.LocalServingHost,
			Port:    webhookInstallOptions.LocalServingPort,
			CertDir: webhookInstallOptions.LocalServingCertDir,
		}),
		LeaderElection: false,
		Metrics:        metricsserver.Options{BindAddress: "0"},
	})
	Expect(err).NotTo(HaveOccurred())

	// Setup controllers
	err = controllers.SetupControllers(mgr)
	Expect(err).NotTo(HaveOccurred())

	// Setup TestSubject webhook
	err = testsubjectwebhook.SetupTestSubjectWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	// Start the manager
	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).NotTo(HaveOccurred(), "failed to run manager")
	}()

	// Wait for the manager to be ready
	Eventually(func() bool {
		return mgr.GetCache().WaitForCacheSync(ctx)
	}, time.Minute, time.Second).Should(BeTrue())

	// Wait for the webhook server to get ready
	if webhookInstallOptions.LocalServingHost != "" {
		dialer := &net.Dialer{Timeout: time.Second}
		addrPort := fmt.Sprintf("%s:%d", webhookInstallOptions.LocalServingHost, webhookInstallOptions.LocalServingPort)
		Eventually(func() error {
			conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true})
			if err != nil {
				return err
			}
			return conn.Close()
		}, time.Minute, time.Second).Should(Succeed())
	}
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

// Helper functions for integration tests

// CreateNamespace creates a test namespace
func CreateNamespace(name string) *corev1.Namespace {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
	return namespace
}

// DeleteNamespace deletes a test namespace
func DeleteNamespace(name string) {
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
	}, time.Minute, time.Second).Should(Succeed())
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
			if ts.Labels[labelKey] == labelValue {
				testSubject = &ts
				return true
			}
		}
		return false
	}, time.Minute, time.Second).Should(BeTrue())
	return testSubject
}

// WaitForPipelineRun waits for a PipelineRun to exist
func WaitForPipelineRun(namespace, labelKey, labelValue string) *tektonv1.PipelineRun {
	var pipelineRun *tektonv1.PipelineRun
	Eventually(func() bool {
		pipelineRuns := &tektonv1.PipelineRunList{}
		err := k8sClient.List(ctx, pipelineRuns, client.InNamespace(namespace))
		if err != nil {
			return false
		}

		for _, pr := range pipelineRuns.Items {
			if pr.Labels[labelKey] == labelValue {
				pipelineRun = &pr
				return true
			}
		}
		return false
	}, time.Minute, time.Second).Should(BeTrue())
	return pipelineRun
}

// CreateSuccessfulBuildPipelineRun creates a successful build PipelineRun
func CreateSuccessfulBuildPipelineRun(namespace, componentName, imageURL string) *tektonv1.PipelineRun {
	pipelineRun := &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "build-",
			Namespace:    namespace,
			Labels: map[string]string{
				"pipelines.appstudio.openshift.io/type": "build",
				"appstudio.openshift.io/component":      componentName,
			},
		},
		Spec: tektonv1.PipelineRunSpec{
			PipelineRef: &tektonv1.PipelineRef{
				Name: "dummy-pipeline",
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
						Value: *tektonv1.NewStructuredValues("https://github.com/myorg/" + componentName),
					},
					{
						Name:  "CHAINS-GIT_COMMIT",
						Value: *tektonv1.NewStructuredValues("abc123"),
					},
				},
			},
		},
	}

	Expect(k8sClient.Create(ctx, pipelineRun)).To(Succeed())

	// Update status to mark as successful
	pipelineRun.Status.SetCondition(&apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: "True",
	})
	Expect(k8sClient.Status().Update(ctx, pipelineRun)).To(Succeed())

	return pipelineRun
}

// CreateIntegrationTestScenario creates an IntegrationTestScenario
func CreateIntegrationTestScenario(namespace, name, groupLabel string, optional bool) *integrationv1alpha1.IntegrationTestScenario {
	scenario := &integrationv1alpha1.IntegrationTestScenario{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: integrationv1alpha1.IntegrationTestScenarioSpec{
			Selector: integrationv1alpha1.IntegrationTestScenarioSelector{
				MatchLabels: map[string]string{
					integrationv1alpha1.TestSubjectGroupLabel: groupLabel,
				},
			},
			ResolverRef: integrationv1alpha1.ResolverRef{
				Resolver: "git",
				Params: []integrationv1alpha1.ResolverParameter{
					{Name: "url", Value: "https://github.com/myorg/test-definitions"},
					{Name: "revision", Value: "main"},
					{Name: "pathInRepo", Value: fmt.Sprintf("%s/pipeline.yaml", name)},
				},
			},
			Params: []integrationv1alpha1.PipelineParameter{
				{Name: "IMAGE_URL", Value: "$(params.TEST_SUBJECT_IMAGES)"},
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
				ImageURL: ".status.results[] | select(.name == \"IMAGE_URL\") | .value.stringVal",
				Source: integrationv1alpha1.TestSubjectSourceExtractor{
					Git: integrationv1alpha1.TestSubjectGitSourceExtractor{
						URL:      ".status.results[] | select(.name == \"CHAINS-GIT_URL\") | .value.stringVal",
						Revision: ".status.results[] | select(.name == \"CHAINS-GIT_COMMIT\") | .value.stringVal",
					},
				},
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
