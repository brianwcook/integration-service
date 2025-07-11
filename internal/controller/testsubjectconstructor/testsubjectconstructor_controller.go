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
	integrationv1alpha1 "github.com/konflux-ci/integration-service/api/integration/v1alpha1"
	"github.com/konflux-ci/integration-service/helpers"
	"github.com/konflux-ci/operator-toolkit/controller"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciler reconciles a TestSubjectConstructor object
type Reconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// NewTestSubjectConstructorReconciler creates and returns a Reconciler.
func NewTestSubjectConstructorReconciler(client client.Client, logger *logr.Logger, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		Client: client,
		Log:    logger.WithName("testsubjectconstructor"),
		Scheme: scheme,
	}
}

//+kubebuilder:rbac:groups=integration.konflux-ci.dev,resources=testsubjectconstructors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=integration.konflux-ci.dev,resources=testsubjectconstructors/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=integration.konflux-ci.dev,resources=testsubjectconstructors/finalizers,verbs=update
//+kubebuilder:rbac:groups=integration.konflux-ci.dev,resources=testsubjects,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tekton.dev,resources=pipelineruns,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := helpers.IntegrationLogger{Logger: r.Log.WithValues("testsubjectconstructor", req.NamespacedName)}

	constructor := &integrationv1alpha1.TestSubjectConstructor{}
	err := r.Get(ctx, req.NamespacedName, constructor)
	if err != nil {
		logger.Error(err, "Failed to get TestSubjectConstructor for", "req", req.NamespacedName)
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	adapter := NewAdapter(ctx, constructor, logger, r.Client)

	return controller.ReconcileHandler([]controller.Operation{
		adapter.EnsureTestSubjectConstructed,
		adapter.EnsureStatusUpdated,
	})
}

// ReconcileTriggeredResource handles reconciliation when a watched resource changes
func (r *Reconciler) ReconcileTriggeredResource(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := helpers.IntegrationLogger{Logger: r.Log.WithValues("triggered-resource", req.NamespacedName)}

	// Get the resource that triggered this reconciliation
	pipelineRun := &tektonv1.PipelineRun{}
	err := r.Get(ctx, req.NamespacedName, pipelineRun)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get triggering PipelineRun")
		return ctrl.Result{}, err
	}

	// Find all TestSubjectConstructors that match this resource
	constructors := &integrationv1alpha1.TestSubjectConstructorList{}
	if err := r.List(ctx, constructors, &client.ListOptions{
		Namespace: pipelineRun.Namespace,
	}); err != nil {
		logger.Error(err, "Failed to list TestSubjectConstructors")
		return ctrl.Result{}, err
	}

	// Process each matching constructor
	var processingErrors []error
	for _, constructor := range constructors.Items {
		if r.resourceMatchesConstructor(pipelineRun, &constructor) {
			adapter := NewAdapter(ctx, &constructor, logger, r.Client)
			if err := adapter.ProcessTriggeringResource(pipelineRun); err != nil {
				logger.Error(err, "Failed to process triggering resource", "constructor", constructor.Name)
				processingErrors = append(processingErrors, err)
			}
		}
	}

	// If all processing attempts failed, return an error
	if len(processingErrors) > 0 {
		return ctrl.Result{}, processingErrors[0]
	}

	return ctrl.Result{}, nil
}

// resourceMatchesConstructor checks if a resource matches the selector criteria of a TestSubjectConstructor
func (r *Reconciler) resourceMatchesConstructor(pipelineRun *tektonv1.PipelineRun, constructor *integrationv1alpha1.TestSubjectConstructor) bool {
	// Check field matches
	for field, expectedValue := range constructor.Spec.Selector.Fields.Match {
		switch field {
		case "kind":
			if pipelineRun.Kind != expectedValue {
				return false
			}
		case "version":
			if pipelineRun.APIVersion != expectedValue {
				return false
			}
		// Add more field checks as needed
		default:
			// For unknown fields, assume they don't match
			return false
		}
	}

	// Check label matches
	for label, expectedValue := range constructor.Spec.Selector.Labels.Match {
		if pipelineRun.Labels == nil || pipelineRun.Labels[label] != expectedValue {
			return false
		}
	}

	return true
}

// AdapterInterface is an interface defining all the operations that should be defined in a TestSubjectConstructor adapter.
type AdapterInterface interface {
	EnsureTestSubjectConstructed() (controller.OperationResult, error)
	EnsureStatusUpdated() (controller.OperationResult, error)
	ProcessTriggeringResource(resource client.Object) error
}

// SetupController creates a new TestSubjectConstructor controller and adds it to the Manager.
func SetupController(manager ctrl.Manager, log *logr.Logger) error {
	return setupControllerWithManager(manager, NewTestSubjectConstructorReconciler(manager.GetClient(), log, manager.GetScheme()))
}

// setupControllerWithManager sets up the controller with the Manager
func setupControllerWithManager(manager ctrl.Manager, reconciler *Reconciler) error {
	// Create the main controller for TestSubjectConstructor
	err := ctrl.NewControllerManagedBy(manager).
		For(&integrationv1alpha1.TestSubjectConstructor{}).
		Named("testsubjectconstructor").
		Complete(reconciler)
	if err != nil {
		return err
	}

	// Create a separate controller for watching PipelineRuns
	return ctrl.NewControllerManagedBy(manager).
		Named("testsubjectconstructor-pipelinerun-watcher").
		Watches(&tektonv1.PipelineRun{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				// Send PipelineRun events to ReconcileTriggeredResource by using the PipelineRun name
				return []reconcile.Request{
					{NamespacedName: types.NamespacedName{
						Name:      obj.GetName(),
						Namespace: obj.GetNamespace(),
					}},
				}
			}),
			builder.WithPredicates(
				predicate.Funcs{
					CreateFunc: func(e event.CreateEvent) bool {
						return false
					},
					UpdateFunc: func(e event.UpdateEvent) bool {
						// Only trigger on successful completion
						oldPR, oldOk := e.ObjectOld.(*tektonv1.PipelineRun)
						newPR, newOk := e.ObjectNew.(*tektonv1.PipelineRun)
						if !oldOk || !newOk {
							return false
						}

						// Trigger when pipeline run becomes successful
						oldCompleted := oldPR.Status.CompletionTime != nil
						newCompleted := newPR.Status.CompletionTime != nil

						return !oldCompleted && newCompleted && isSuccessful(newPR)
					},
					DeleteFunc: func(e event.DeleteEvent) bool {
						return false
					},
				},
			),
		).
		Complete(&TriggeredResourceReconciler{reconciler: reconciler})
}

// TriggeredResourceReconciler is a wrapper to handle PipelineRun events
type TriggeredResourceReconciler struct {
	reconciler *Reconciler
}

// Reconcile handles PipelineRun events and delegates to ReconcileTriggeredResource
func (r *TriggeredResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.ReconcileTriggeredResource(ctx, req)
}

// isSuccessful checks if a PipelineRun completed successfully
func isSuccessful(pipelineRun *tektonv1.PipelineRun) bool {
	return pipelineRun.Status.CompletionTime != nil &&
		pipelineRun.Status.GetCondition(apis.ConditionSucceeded) != nil &&
		pipelineRun.Status.GetCondition(apis.ConditionSucceeded).IsTrue()
}
