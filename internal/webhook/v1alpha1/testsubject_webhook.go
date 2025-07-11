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
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	integrationv1alpha1 "github.com/konflux-ci/integration-service/api/integration/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var testsubjectlog = logf.Log.WithName("testsubject-webhook")

func SetupTestSubjectWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&integrationv1alpha1.TestSubject{}).
		WithValidator(&TestSubjectCustomValidator{}).
		Complete()
}

// TestSubjectCustomValidator is a webhook handler and does not need deepcopy methods.
// +k8s:deepcopy-gen=false
type TestSubjectCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

// +kubebuilder:webhook:path=/validate-integration-konflux-ci-dev-v1alpha1-testsubject,mutating=false,failurePolicy=fail,sideEffects=None,groups=integration.konflux-ci.dev,resources=testsubjects,verbs=create;update;delete,versions=v1alpha1,name=vtestsubject.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &TestSubjectCustomValidator{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *TestSubjectCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	testSubject, ok := obj.(*integrationv1alpha1.TestSubject)
	if !ok {
		return nil, fmt.Errorf("expected a TestSubject object but got %T", obj)
	}

	testsubjectlog.Info("Validating TestSubject upon creation", "name", testSubject.GetName())

	// No specific validation needed for create operations
	// Components can be set during creation

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *TestSubjectCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	oldTestSubject, ok := oldObj.(*integrationv1alpha1.TestSubject)
	if !ok {
		return nil, fmt.Errorf("expected a TestSubject object for oldObj but got %T", oldObj)
	}

	newTestSubject, ok := newObj.(*integrationv1alpha1.TestSubject)
	if !ok {
		return nil, fmt.Errorf("expected a TestSubject object for newObj but got %T", newObj)
	}

	testsubjectlog.Info("Validating TestSubject upon update", "name", newTestSubject.GetName())

	// Check if components field has been modified
	if !reflect.DeepEqual(oldTestSubject.Spec.Components, newTestSubject.Spec.Components) {
		testsubjectlog.Info("Components field modification detected", "name", newTestSubject.GetName())
		return nil, field.Invalid(
			field.NewPath("spec").Child("components"),
			newTestSubject.Spec.Components,
			"components field is immutable and cannot be modified after creation",
		)
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *TestSubjectCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	testSubject, ok := obj.(*integrationv1alpha1.TestSubject)
	if !ok {
		return nil, fmt.Errorf("expected a TestSubject object but got %T", obj)
	}

	testsubjectlog.Info("Validating TestSubject upon deletion", "name", testSubject.GetName())

	// No specific validation needed for delete operations

	return nil, nil
}
