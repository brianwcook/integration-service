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
	integrationv1alpha1 "github.com/konflux-ci/integration-service/api/integration/v1alpha1"
	"github.com/konflux-ci/integration-service/helpers"
	"github.com/konflux-ci/operator-toolkit/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// Reconciler reconciles a TestSubject object
type Reconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// NewTestSubjectReconciler creates and returns a Reconciler.
func NewTestSubjectReconciler(client client.Client, logger *logr.Logger, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		Client: client,
		Log:    logger.WithName("testsubject"),
		Scheme: scheme,
	}
}

//+kubebuilder:rbac:groups=integration.konflux-ci.dev,resources=testsubjects,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=integration.konflux-ci.dev,resources=testsubjects/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=integration.konflux-ci.dev,resources=testsubjects/finalizers,verbs=update
//+kubebuilder:rbac:groups=integration.konflux-ci.dev,resources=integrationtestscenarios,verbs=get;list;watch
//+kubebuilder:rbac:groups=tekton.dev,resources=pipelineruns,verbs=get;list;watch;create;update;patch;delete;deletecollection
//+kubebuilder:rbac:groups=tekton.dev,resources=pipelineruns/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := helpers.IntegrationLogger{Logger: r.Log.WithValues("testsubject", req.NamespacedName)}

	testSubject := &integrationv1alpha1.TestSubject{}
	err := r.Get(ctx, req.NamespacedName, testSubject)
	if err != nil {
		logger.Error(err, "Failed to get TestSubject for", "req", req.NamespacedName)
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Ensure TestSubject has the default group label if not set
	if testSubject.Labels == nil {
		testSubject.Labels = make(map[string]string)
	}
	if _, exists := testSubject.Labels[integrationv1alpha1.TestSubjectGroupLabel]; !exists {
		testSubject.Labels[integrationv1alpha1.TestSubjectGroupLabel] = "default"
		if err := r.Update(ctx, testSubject); err != nil {
			logger.Error(err, "Failed to update TestSubject with default group label")
			return ctrl.Result{}, err
		}
		logger.Info("Added default group label to TestSubject")
		return ctrl.Result{Requeue: true}, nil
	}

	adapter := NewAdapter(ctx, testSubject, logger, r.Client)

	return controller.ReconcileHandler([]controller.Operation{
		adapter.EnsureIntegrationTestPipelineRunsExist,
		adapter.EnsureTestResultsUpdated,
		adapter.EnsureControlTestSubjectUpdated,
	})
}

// AdapterInterface is an interface defining all the operations that should be defined in a TestSubject adapter.
type AdapterInterface interface {
	EnsureIntegrationTestPipelineRunsExist() (controller.OperationResult, error)
	EnsureTestResultsUpdated() (controller.OperationResult, error)
	EnsureControlTestSubjectUpdated() (controller.OperationResult, error)
}

// SetupController creates a new TestSubject controller and adds it to the Manager.
func SetupController(manager ctrl.Manager, log *logr.Logger) error {
	return setupControllerWithManager(manager, NewTestSubjectReconciler(manager.GetClient(), log, manager.GetScheme()))
}

// setupControllerWithManager sets up the controller with the Manager which monitors TestSubjects
func setupControllerWithManager(manager ctrl.Manager, controller *Reconciler) error {
	return ctrl.NewControllerManagedBy(manager).
		For(&integrationv1alpha1.TestSubject{}).
		WithEventFilter(predicate.Or(
			TestSubjectCreatedPredicate(),
			TestSubjectTestStatusChangedPredicate(),
		)).
		Complete(controller)
}
